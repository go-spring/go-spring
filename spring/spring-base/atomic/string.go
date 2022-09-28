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
	"sync/atomic"
)

// A String is an atomic string value.
type String struct {
	_ nocopy
	v atomic.Value
}

// Load atomically loads and returns the value stored in x.
func (x *String) Load() string {
	if r := x.v.Load(); r != nil {
		return r.(string)
	}
	return ""
}

// Store atomically stores val into x.
func (x *String) Store(val string) {
	x.v.Store(val)
}

// MarshalJSON returns the JSON encoding of x.
func (x *String) MarshalJSON() ([]byte, error) {
	return json.Marshal(x.Load())
}
