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

type MarshalValue func(interface{}) ([]byte, error)

// A Value provides an atomic load and store of a consistently typed value.
type Value struct {
	_ nocopy
	atomic.Value

	marshalJSON MarshalValue
}

// SetMarshalJSON sets the JSON encoding handler for x.
func (x *Value) SetMarshalJSON(fn MarshalValue) {
	x.marshalJSON = fn
}

// MarshalJSON returns the JSON encoding of x.
func (x *Value) MarshalJSON() ([]byte, error) {
	if x.marshalJSON != nil {
		return x.marshalJSON(x.Load())
	}
	return json.Marshal(x.Load())
}
