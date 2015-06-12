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

package blueflood

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/square/metrics/api"
	"github.com/square/metrics/log"
)

type httpClient interface {
	// our own client to mock out the standard golang HTTP Client.
	Get(url string) (resp *http.Response, err error)
}

type blueflood struct {
	baseUrl  string
	tenantId string
	client   httpClient
}

type queryResponse struct {
	Values []metricPoint `json:"values"`
}

type metricPoint struct {
	Points    int     `json:"numPoints"`
	Timestamp int64   `json:"timestamp"`
	Average   float64 `json:"average"`
	Max       float64 `json:"max"`
	Min       float64 `json:"min"`
	Variance  float64 `json:"variance"`
}

// resolutions supported by Blueflood
type resolution string

const (
	resolutionFull    resolution = "FULL"
	resolution5Min               = "MIN5"
	resolution20Min              = "MIN20"
	resolution60Min              = "MIN60"
	resolution240Min             = "MIN240"
	resolution1440Min            = "MIN1440"
)

func NewBlueflood(baseUrl string, tenantId string) api.Backend {
	return &blueflood{baseUrl: baseUrl, tenantId: tenantId, client: http.DefaultClient}
}

func (b *blueflood) FetchSingleSeries(request api.FetchSeriesRequest) (api.Timeseries, error) {
	sampler, ok := samplerMap[request.SampleMethod]
	if !ok {
		return api.Timeseries{}, errors.New(fmt.Sprintf("Unsupported SampleMethod %d", request.SampleMethod))
	}

	graphiteName, err := request.Api.ToGraphiteName(request.Metric)

	if err != nil {
		return api.Timeseries{}, err
	}

	// Issue GET to fetch metrics
	queryUrl, err := url.Parse(fmt.Sprintf("%s/v2.0/%s/views/%s",
		b.baseUrl,
		b.tenantId,
		graphiteName))

	if err != nil {
		return api.Timeseries{}, err
	}

	params := url.Values{}
	params.Set("from", strconv.FormatInt(request.Timerange.Start(), 10))
	// Pull a bit outside of the requested range from blueflood so we
	// have enough data to generate all snapped values
	params.Set("to", strconv.FormatInt(request.Timerange.End()+request.Timerange.Resolution(), 10))
	params.Set("resolution", string(bluefloodResolution(request.Timerange.Resolution())))
	params.Set("select", fmt.Sprintf("numPoints,%s", strings.ToLower(sampler.fieldName)))

	queryUrl.RawQuery = params.Encode()

	log.Infof("Blueflood fetch: %s", queryUrl.String())
	resp, err := b.client.Get(queryUrl.String())
	if err != nil {
		return api.Timeseries{}, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return api.Timeseries{}, err
	}

	log.Infof("Fetch result: %s", string(body))

	var result queryResponse
	err = json.Unmarshal(body, &result)
	if err != nil {
		return api.Timeseries{}, err
	}

	// Construct a Timeseries from the result:

	// buckets are each filled with from the points stored in result.Values, according to their timestamps.
	buckets := bucketsFromMetricPoints(result.Values, sampler.fieldSelector, request.Timerange)

	// values will hold the final values to be returned as the series.
	values := make([]float64, request.Timerange.Slots())

	for i, bucket := range buckets {
		if len(bucket) == 0 {
			values[i] = math.NaN()
			continue
		}
		values[i] = sampler.bucketSampler(bucket)
	}

	log.Infof("Constructed timeseries from result: %v", values)

	return api.Timeseries{
		Values: values,
		TagSet: request.Metric.TagSet,
	}, nil
}

func addMetricPoint(metricPoint metricPoint, field func(metricPoint) float64, timerange api.Timerange, buckets [][]float64) bool {
	value := field(metricPoint)
	// The index to assign within the array is computed using the timestamp.
	// It floors to the nearest index.
	index := (metricPoint.Timestamp - timerange.Start()) / timerange.Resolution()
	if index < 0 || index >= int64(timerange.Slots()) {
		return false
	}
	buckets[index] = append(buckets[index], value)
	return true
}

func bucketsFromMetricPoints(metricPoints []metricPoint, resultField func(metricPoint) float64, timerange api.Timerange) [][]float64 {
	buckets := make([][]float64, timerange.Slots())
	// Make the buckets:
	for i := range buckets {
		buckets[i] = []float64{}
	}
	for _, point := range metricPoints {
		addMetricPoint(point, resultField, timerange, buckets)
	}
	return buckets
}

var samplerMap map[api.SampleMethod]struct {
	fieldName     string
	fieldSelector func(point metricPoint) float64
	bucketSampler func([]float64) float64
} = map[api.SampleMethod]struct {
	fieldName     string
	fieldSelector func(point metricPoint) float64
	bucketSampler func([]float64) float64
}{
	api.SampleMean: {
		fieldName:     "average",
		fieldSelector: func(point metricPoint) float64 { return point.Average },
		bucketSampler: func(bucket []float64) float64 {
			value := 0.0
			for _, v := range bucket {
				value += v
			}
			return value / float64(len(bucket))
		},
	},
	api.SampleMin: {
		fieldName:     "min",
		fieldSelector: func(point metricPoint) float64 { return point.Min },
		bucketSampler: func(bucket []float64) float64 {
			value := bucket[0]
			for _, v := range bucket {
				value = math.Min(value, v)
			}
			return value
		},
	},
	api.SampleMax: {
		fieldName:     "max",
		fieldSelector: func(point metricPoint) float64 { return point.Max },
		bucketSampler: func(bucket []float64) float64 {
			value := bucket[0]
			for _, v := range bucket {
				value = math.Max(value, v)
			}
			return value
		},
	},
}

// Blueflood keys the resolution param to a java enum, so we have to convert
// between them.
func bluefloodResolution(r int64) resolution {
	switch {
	case r < 5*60*1000:
		return resolutionFull
	case r < 20*60*1000:
		return resolution5Min
	case r < 60*60*1000:
		return resolution20Min
	case r < 240*60*1000:
		return resolution60Min
	case r < 1440*60*1000:
		return resolution240Min
	}
	return resolution1440Min
}
