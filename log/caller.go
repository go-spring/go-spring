/*
 * Copyright 2025 The Go-Spring Authors.
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

package log

import (
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/go-spring/stdlib/errutil"
)

// callerType indicates the type of caller information to use.
var callerType = CallerTypeFast

func init() {
	if s, ok := os.LookupEnv("GS_LOGGER_CALLER_TYPE"); ok {
		r, err := ParseCallerType(s)
		if err != nil {
			panic(err)
		}
		callerType = r
	}
}

// CallerType defines the type of caller information to retrieve.
type CallerType int

const (
	// CallerTypeDefault indicates that the caller information should be retrieved
	// using the default method.
	CallerTypeDefault CallerType = iota

	// CallerTypeFast indicates that the caller information should be retrieved
	// using a faster but less accurate method.
	CallerTypeFast

	// CallerTypeNone indicates that no caller information should be retrieved.
	CallerTypeNone
)

// ParseCallerType parses a string representation of a CallerType
// and returns the corresponding value.
func ParseCallerType(s string) (CallerType, error) {
	switch s = strings.TrimSpace(s); s {
	case "default":
		return CallerTypeDefault, nil
	case "fast":
		return CallerTypeFast, nil
	case "none":
		return CallerTypeNone, nil
	default:
		return 0, errutil.Explain(nil, "invalid caller type %q", s)
	}
}

// frameCache is used to cache call site information.
// Benchmarking shows that using this cache improves performance by about 50%.
var frameCache sync.Map

// FastCaller returns the file name and line number of the calling function.
// It uses a cache to speed up the lookup.
func FastCaller(skip int) (file string, line int) {
	var rpc [1]uintptr
	n := runtime.Callers(skip+2, rpc[:])
	if n < 1 {
		return
	}
	pc := rpc[0]
	if v, ok := frameCache.Load(pc); ok {
		e := v.(runtime.Frame)
		return e.File, e.Line
	}
	frame, _ := runtime.CallersFrames(rpc[:]).Next()
	frameCache.Store(pc, frame)
	return frame.File, frame.Line
}
