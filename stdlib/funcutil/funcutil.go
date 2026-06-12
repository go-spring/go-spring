/*
 * Copyright 2024 The Go-Spring Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package funcutil

import (
	"reflect"
	"runtime"
	"strings"
)

// FuncName returns the function name for a given function.
func FuncName(fn any) string {
	_, _, fnName := FileLine(fn)
	return fnName
}

// FileLine returns the file, line number, and function name for a given function.
// It uses reflection and runtime information to extract these details.
// 'fn' is expected to be a function or method value.
func FileLine(fn any) (file string, line int, fnName string) {

	fnPtr := reflect.ValueOf(fn).Pointer()
	fnInfo := runtime.FuncForPC(fnPtr)
	file, line = fnInfo.FileLine(fnPtr)

	s := fnInfo.Name()
	i := strings.LastIndex(s, "/")
	if i > 0 {
		s = s[i+1:]
	}

	// method values are printed as "T.m-fm"
	s = strings.TrimRight(s, "-fm")
	return file, line, s
}
