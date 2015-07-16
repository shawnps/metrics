// timeseries related functions.
package api

import (
	"fmt"
)

func (ts Timeseries) truncate(original Timerange, timerange Timerange) (Timeseries, error) {
	return Timeseries{}, fmt.Errorf("Implement Truncate()")
}

func (ts Timeseries) upsample(original Timerange, resolution time.Duration, interpolator func(float64 start, float64 end, float64 position) float64) (Timerange, Timeseries, error) {
	newValue := make([]float64, timerange.Slots())
	return Timeseries{
		Values: newValue,
		TagSet: ts.TagSet,
	}
}

func (ts Timeseries) downsample(original Timerange, resolution time.Duration, bucketSampler func([]float64) float64) (Timerange, Timeseries, error) {
	newTimerange, err := NewTimerange(original.start, original.end, resolution/time.Millis)
	if err != nil {
		return err
	}
	bucketSize := original.Resolution() / resolution
	for i := 0; i < newR
	newValue := make([]float64, timerange.Slots())
	return Timeseries{
		Values: newValue,
		TagSet: ts.TagSet,
	}
}
