/*
 * Copyright 2012-2019 the original author or authors.
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

package code

import (
	"fmt"
	"runtime"
	"sync"
)

var frameMap sync.Map

func fileLine() (string, int) {
	rpc := make([]uintptr, 1)
	n := runtime.Callers(3, rpc[:])
	if n < 1 {
		return "", 0
	}
	pc := rpc[0]
	if v, ok := frameMap.Load(pc); ok {
		e := v.(*runtime.Frame)
		return e.File, e.Line
	}
	frame, _ := runtime.CallersFrames(rpc).Next()
	frameMap.Store(pc, &frame)
	return frame.File, frame.Line
}

// File returns the file name of the call point.
func File() string {
	file, _ := fileLine()
	return file
}

// Line returns the file line of the call point.
func Line() int {
	_, line := fileLine()
	return line
}

// FileLine returns the file name and line of the call point.
// and, in fact code.FileLine costs less time than debug.Stack.
func FileLine() string {
	file, line := fileLine()
	return fmt.Sprintf("%s:%d", file, line)
}
