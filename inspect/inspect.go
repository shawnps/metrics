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

// TODO - explore how to measure per-query metric.
// TODO - expose this (maybe in UI? dump it in a JSON / table? or even, a command?)
// TODO - wire this up with the square's metrics system.
package inspect

import (
	"fmt"
	"time"

	"github.com/rcrowley/go-metrics"
	"github.com/square/metrics/api"
)

type Metric interface {
	RecordQueryLatency(string, bool, time.Duration) // records the latency of a single query.
	RecordFetchesPerQuery(string, time.Duration)    // records the # of fetches of a single query.
	RecordFetchLatency(time.Duration)               // records the latency of a single fetch.
	RecordApiLatency(string, time.Duration)         // records the latency of a single API call.
}

type EnvMetric struct {
	registry metrics.Registry // per-registry histograms.
}

func (m EnvMetric) RecordQueryLatency(name string, success bool, d time.Duration) {
	// todo - record success / failure in an counter.
	// todo - track syntax errors in a different bin?
	m.histogram("query-latency", name).Update(int64(d))
}

func (m EnvMetric) RecordFetchPerQuery(name string, d time.Duration) {
	m.histogram("fetch-per-query", name).Update(int64(d))
}

func (m EnvMetric) RecordFetchLatency(name string, d time.Duration) {
	m.histogram("fetch-latency", name).Update(int64(d))
}

func (m EnvMetric) RecordApiLatency(name string, d time.Duration) {
	m.histogram("api-latency", name).Update(int64(d))
}

func (m EnvMetric) histogram(group, name string) metrics.Histogram {
	newHistogram := func() metrics.Histogram {
		return metrics.NewHistogram(metrics.NewUniformSample(512))
	}
	return m.registry.GetOrRegister(fmt.Sprintf("%s.%s", group, name), newHistogram).(metrics.Histogram)
}

// wrapped version of API, where the api calls are recorded.
type inspectedAPI struct {
	metric  Metric
	wrapped api.API
}

func (a inspectedAPI) AddMetric(metric api.TaggedMetric) error {
	start := time.Now()
	defer a.record("AddMetric", start)
	return a.wrapped.AddMetric(metric)
}

func (a inspectedAPI) RemoveMetric(metric api.TaggedMetric) error {
	start := time.Now()
	defer a.record("RemoveMetric", start)
	return a.wrapped.RemoveMetric(metric)
}

func (a inspectedAPI) ToGraphiteName(metric api.TaggedMetric) (api.GraphiteMetric, error) {
	start := time.Now()
	defer a.record("ToGraphiteName", start)
	return a.wrapped.ToGraphiteName(metric)
}

func (a inspectedAPI) ToTaggedName(metric api.GraphiteMetric) (api.TaggedMetric, error) {
	start := time.Now()
	defer a.record("ToTaggedName", start)
	return a.wrapped.ToTaggedName(metric)
}

func (a inspectedAPI) GetAllTags(metricKey api.MetricKey) ([]api.TagSet, error) {
	start := time.Now()
	defer a.record("GetAllTags", start)
	return a.wrapped.GetAllTags(metricKey)
}

func (a inspectedAPI) GetAllMetrics() ([]api.MetricKey, error) {
	start := time.Now()
	defer a.record("GetAllMetrics", start)
	return a.wrapped.GetAllMetrics()
}

func (a inspectedAPI) GetMetricsForTag(tagKey, tagValue string) ([]api.MetricKey, error) {
	start := time.Now()
	defer a.record("GetMetricsForTag", start)
	return a.wrapped.GetMetricsForTag(tagKey, tagValue)
}

func (a inspectedAPI) record(name string, start time.Time) {
	a.metric.RecordApiLatency(name, time.Now().Sub(start))
}
