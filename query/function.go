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
	if f.ProcessContext == nil {
		f.ProcessContext = func(ctx EvaluationContext) EvaluationContext { return ctx }
	}
	fs.mapping[f.Name] = f
	return true
}

func (fs Functions) RegisterBinary(name string, evaluate func(float64, float64) float64) bool {
	return false
}

func (fs Functions) Get(name string) MetricFunction {
	return fs.mapping[name]
}

type MetricFunction struct {
	Name           string                                    // name of the function
	MinArgument    int                                       // min number of arguments.
	MaxArgument    int                                       // max number of arguments - -1 for no max.
	ProcessContext func(EvaluationContext) EvaluationContext // if set, pre-processes the context.
	Compute        func([]value, []string) (value, error)    // evaluates the given expression.
}

func (f MetricFunction) Evaluate(arguments []Expression, groupBy []string) (value, error) {
	/*
	name := f.Name
	length := len(arguments)
	if f.MinArgument > length || (f.MaxArgument != -1 && f.MaxArgument < length) {
		return nil, ArgumentLengthError{name, f.MinArgument, f.MaxArgument, length}
	}
	*/
	return nil, nil
}
