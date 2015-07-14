// Copyright 2015 Square Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"fmt"
	"sync"
	"time"

	"github.com/square/metrics/inspect"
)

// Cancellable represents a cancellable work.
// See https://blog.golang.org/context
type Cancellable interface {
	Done() chan struct{}         // returns a channel that is closed when the work is done.
	Deadline() (time.Time, bool) // deadline for the request.
}

type Cache struct {
	lock     sync.RWMutex
	elements map[serializedKey]Timeseries
}

type CacheKey struct {
	Metric       TaggedMetric
	Timerange    Timerange
	SampleMethod SampleMethod
}

type serializedKey struct {
	MetricKey      MetricKey
	SerializedTags string
	Timerange      Timerange
	SampleMethod   SampleMethod
}

func (c Cache) GetOrCompute(key CacheKey, function func() Timeseries) Timeseries {
	serialized := serializedKey{
		MetricKey:      key.Metric.MetricKey,
		SerializedTags: key.Metric.TagSet.Serialize(),
		SampleMethod:   key.SampleMethod,
		Timerange:      key.Timerange,
	}
	c.lock.RLock()
	if value, ok := c.elements[serialized]; ok {
		c.lock.RUnlock()
		return value
	}
	c.lock.RUnlock()

	computed := function()
	c.lock.Lock()
	c.elements[serialized] = computed
	c.lock.Unlock()
	return computed
}

type DefaultCancellable struct {
	done     chan struct{}
	deadline *time.Time
}

func (c DefaultCancellable) Done() chan struct{} {
	return c.done
}

func (c DefaultCancellable) Deadline() (time.Time, bool) {
	if c.deadline == nil {
		return time.Time{}, false
	} else {
		return *c.deadline, true
	}
}

func NewTimeoutCancellable(t time.Time) Cancellable {
	return DefaultCancellable{make(chan struct{}), &t}
}

func NewCancellable() Cancellable {
	return DefaultCancellable{make(chan struct{}), nil}
}

// FetchSeriesRequest contains all the information to fetch a single series of metric
// from a backend.
type CommonBackendRequest struct {
	// fields
	SampleMethod SampleMethod // up/downsampling behavior.
	Timerange    Timerange    // time range to fetch data from.

	// the query environment
	API         API
	Cancellable Cancellable
	Profiler    *inspect.Profiler
}

type FetchSeriesRequest struct {
	Metric TaggedMetric
	CommonBackendRequest
}

type FetchMultipleRequest struct {
	Metrics []TaggedMetric
	CommonBackendRequest
}

func (r FetchMultipleRequest) ToSingle(metric TaggedMetric) FetchSeriesRequest {
	return FetchSeriesRequest{
		Metric:               metric,
		CommonBackendRequest: r.CommonBackendRequest,
	}
}

// Backend describes how to fetch time-series data from a given backend.
type Backend interface {
	// FetchSingleSeries should return an instance of BackendError
	FetchSingleSeries(request FetchSeriesRequest) (Timeseries, error)
}

type MultiBackend interface {
	// FetchManySeries fetches the series provided by the given TaggedMetrics
	// corresponding to the Timerange, down/upsampling if necessary using
	// SampleMethod. It may fetch in series or parallel, etc.
	FetchMultipleSeries(request FetchMultipleRequest) (SeriesList, error)
}

type BackendErrorCode int

const (
	FetchTimeoutError  BackendErrorCode = iota + 1 // error while fetching - timeout.
	FetchIOError                                   // error while fetching - general IO.
	InvalidSeriesError                             // the given series is not well-defined.
	LimitError                                     // the fetch limit is reached.
	Unsupported                                    // the given fetch operation is unsupported by the backend.
)

type BackendError struct {
	Metric  TaggedMetric
	Code    BackendErrorCode
	Message string
}

func (err BackendError) Error() string {
	message := "[%s] unknown error"
	switch err.Code {
	case FetchTimeoutError:
		message = "[%s] timeout"
	case InvalidSeriesError:
		message = "[%s] invalid series"
	case LimitError:
		message = "[%s] limit reached"
	case Unsupported:
		message = "[%s] unsupported operation"
	}
	formatted := fmt.Sprintf(message, string(err.Metric.MetricKey))
	if err.Message != "" {
		formatted = formatted + " - " + err.Message
	}
	return formatted
}

func (err BackendError) TokenName() string {
	return string(err.Metric.MetricKey)
}

// ProfilingBackend wraps an ordinary backend so that whenever data is fetched, a profile is recorded for the fetch's duration.
type ProfilingBackend struct {
	Backend Backend
}

func (b ProfilingBackend) FetchSingleSeries(request FetchSeriesRequest) (Timeseries, error) {
	defer request.Profiler.Record("fetchSingleSeries")()
	return b.Backend.FetchSingleSeries(request)
}

// ProfilingMultiBackend wraps an ordinary multibackend so that whenever data is fetched, a profile is recorded for the fetches' durations.
type ProfilingMultiBackend struct {
	MultiBackend MultiBackend
}

func (b ProfilingMultiBackend) FetchMultipleSeries(request FetchMultipleRequest) (SeriesList, error) {
	defer request.Profiler.Record("fetchMultipleSeries")()
	return b.MultiBackend.FetchMultipleSeries(request)
}
