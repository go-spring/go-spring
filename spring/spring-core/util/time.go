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

package util

import (
	"time"
)

// CurrentMilliSeconds 返回当前的毫秒时间
func CurrentMilliSeconds() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

// MilliSeconds 返回对应的毫秒时长
func MilliSeconds(d time.Duration) int64 {
	return d.Nanoseconds() / int64(time.Millisecond)
}
