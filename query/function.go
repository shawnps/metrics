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

package query

var DefaultRegistry = NewFunctions()

type Functions struct {
	mapping map[string]MetricFunction
}

func NewFunctions() Functions {
	return Functions{make(map[string]MetricFunction)}
}

func (fs Functions) Register(f MetricFunction) bool {
	if _, ok := fs.mapping[f.Name]; !ok {
		return false
	}
	if f.Compute == nil {
		return false
	}
	fs.mapping[f.Name] = f
	return true
}

func (fs Functions) RegisterBinary(name string, evaluate func(float64, float64) float64) bool {
	return fs.RegisterEvaluator(name, 2, 2, func(values []value, groupBy []string) (value, error) {
	leftList, err := values[0].toSeriesList(context.Timerange)
	if err != nil {
		return nil, err
	}
	rightList, err := values[1].toSeriesList(context.Timerange)
	if err != nil {
		return nil, err
	}

	joined := join([]api.SeriesList{leftList, rightList})

	result := make([]api.Timeseries, len(joined.Rows))

	for i, row := range joined.Rows {
		left := row.Row[0]
		right := row.Row[1]
		array := make([]float64, len(left.Values))
		for j := 0; j < len(left.Values); j++ {
			array[j] = evaluate(left.Values[j], right.Values[j])
		}
		result[i] = api.Timeseries{array, row.TagSet}
	}

	return seriesListValue(api.SeriesList{
		Series:    result,
		Timerange: context.Timerange,
	}), nil
	})
}

func (fs Functions) RegisterEvaluator(name string, min int, max int, evaluate func([]value, []string) (value, error)) bool {
	return fs.Register(MetricFunction{
		name,
		min,
		max,
		func(context EvaluationContext, arguments []Expression, groupBy []string) (value, error) {
			values, err := evaluateExpressions(context, arguments)
			if err != nil {
				return nil, err
			}
			return evaluate(values, groupBy)
		},
	})
}

func (fs Functions) Get(name string) MetricFunction {
	return fs.mapping[name]
}

type MetricFunction struct {
	Name           string                                    // name of the function
	MinArgument    int                                       // min number of arguments.
	MaxArgument    int                                       // max number of arguments - -1 for no max.
	Compute        func(EvaluationContext, []Expression, []string) (value, error)
	// Compute        func([]value, []string) (value, error)    // evaluates the given expression.
}

func (f MetricFunction) Evaluate(
	context EvaluationContext,
	arguments []Expression,
	groupBy []string,
) (value, error) {
	// preprocessing
	length := len(arguments)
	if f.MinArgument > length || (f.MaxArgument != -1 && f.MaxArgument < length) {
		return nil, ArgumentLengthError{f.Name, f.MinArgument, f.MaxArgument, length}
	}
	return f.Compute(context, arguments, groupBy)
}
