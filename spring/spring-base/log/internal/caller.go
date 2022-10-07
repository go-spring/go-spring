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

package internal

import (
	"runtime"
	"sync"
)

// frameMap 用于缓存调用点，基准测试表明使用缓存大约有 50% 的性能提升。
var frameMap sync.Map

// Caller 获取调用点的文件及行号信息，fast 为 true 时使用缓存进行加速。
func Caller(skip int, fast bool) (file string, line int, loaded bool) {

	if !fast {
		_, file, line, loaded = runtime.Caller(skip + 1)
		return
	}

	rpc := make([]uintptr, 1)
	n := runtime.Callers(skip+2, rpc[:])
	if n < 1 {
		return
	}
	pc := rpc[0]
	if v, ok := frameMap.Load(pc); ok {
		e := v.(*runtime.Frame)
		return e.File, e.Line, true
	}
	frame, _ := runtime.CallersFrames(rpc).Next()
	frameMap.Store(pc, &frame)
	return frame.File, frame.Line, false
}
