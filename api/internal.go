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

/*
DEPRECATED
*/

import (
	"strings"
)

func escapeString(input string) string {
	// must be first
	input = strings.Replace(input, `\`, `\\`, -1)
	input = strings.Replace(input, `,`, `\,`, -1)
	input = strings.Replace(input, `=`, `\=`, -1)
	return input
}

func unescapeString(input string) string {
	input = strings.Replace(input, `\,`, `,`, -1)
	input = strings.Replace(input, `\=`, `=`, -1)
	// must be last.
	input = strings.Replace(input, `\\`, `\`, -1)
	return input
}
