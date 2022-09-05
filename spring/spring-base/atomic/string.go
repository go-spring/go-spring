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

package atomic

import (
	"encoding/json"
)

type String struct {
	_ nocopy
	v Value
}

// Load wrapper for atomic.LoadInt64.
func (x *String) Load() string {
	if r := x.v.Load(); r != nil {
		return r.(string)
	}
	return ""
}

// Store wrapper for atomic.StoreInt64.
func (x *String) Store(val string) {
	x.v.Store(val)
}

// Swap wrapper for atomic.SwapInt64.
func (x *String) Swap(new string) string {
	return x.v.Swap(new).(string)
}

// CompareAndSwap wrapper for atomic.CompareAndSwapInt64.
func (x *String) CompareAndSwap(old, new string) bool {
	return x.v.CompareAndSwap(old, new)
}

func (x *String) MarshalJSON() ([]byte, error) {
	return json.Marshal(x.Load())
}
