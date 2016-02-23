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

package filter

import (
	"math"
	"sort"
	"time"

	"github.com/square/metrics/api"
)

type filterList struct {
	index     []int
	value     []float64
	ascending bool
}

func (list filterList) Len() int {
	return len(list.index)
}
func (list filterList) Less(i, j int) bool {
	if math.IsNaN(list.value[j]) && !math.IsNaN(list.value[i]) {
		return true
	}
	if list.ascending {
		return list.value[i] < list.value[j]
	} else {
		return list.value[j] < list.value[i]
	}

}
func (list filterList) Swap(i, j int) {
	list.index[i], list.index[j] = list.index[j], list.index[i]
	list.value[i], list.value[j] = list.value[j], list.value[i]
}

func sortSeries(series []api.Timeseries, summary func([]float64) float64, lowest bool) ([]api.Timeseries, []float64) {
	array := filterList{
		index:     make([]int, len(series)),
		value:     make([]float64, len(series)),
		ascending: lowest,
	}
	for i := range array.index {
		array.index[i] = i
		array.value[i] = summary(series[i].Values)
	}
	sort.Sort(array)
	result := make([]api.Timeseries, len(series))
	weights := make([]float64, len(series))
	for i, index := range array.index {
		result[i] = series[index]
		weights[i] = array.value[index]
	}
	return result, weights
}

// FilterRecentBy reduces the number of things in the series `list` to at most the given `count`.
// However, it only considered recent points when evaluating their ordering.
func FilterByRecent(list api.SeriesList, count int, summary func([]float64) float64, lowest bool, duration time.Duration) api.SeriesList {
	slots := int(duration / list.Timerange.Resolution())
	if slots <= 0 {
		slots = 1
	}
	if slots > list.Timerange.Slots() {
		slots = list.Timerange.Slots()
	}
	sorted, _ := sortSeries(list.Series, func(values []float64) float64 {
		return summary(values[len(values)-slots:])
	}, lowest)

	if len(list.Series) < count {
		// Limit the count to the number of available series
		count = len(list.Series)
	}

	return api.SeriesList{
		Series:    sorted[:count],
		Timerange: list.Timerange,
	}
}

// FilterThresholdBy reduces the number of things in the series `list` to those whose `summar` is at at least/at most the threshold.
// However, it only considers the data points as recent as the duration permits.
func FilterThresholdByRecent(list api.SeriesList, threshold float64, summary func([]float64) float64, lowest bool, duration time.Duration) api.SeriesList {
	slots := int(duration / list.Timerange.Resolution())
	if slots > list.Timerange.Slots() {
		slots = list.Timerange.Slots()
	}
	sorted, values := sortSeries(list.Series, func(values []float64) float64 {
		return summary(values[len(values)-slots:])
	}, lowest)

	result := []api.Timeseries{}
	for i := range sorted {
		// Since the series are sorted, once one of them falls outside the threshold, we can stop.
		if (lowest && values[i] > threshold) || (!lowest && values[i] < threshold) {
			break
		}
		result = append(result, sorted[i])
	}

	return api.SeriesList{
		Series:    result,
		Timerange: list.Timerange,
	}
}
