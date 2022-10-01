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
	"sync/atomic"
	"time"

	"github.com/go-spring/spring-base/json"
)

type MarshalTime func(time.Time) ([]byte, error)

// A Time is an atomic time.Time value.
type Time struct {
	_ nocopy
	v atomic.Value

	marshalJSON MarshalTime
}

// Load atomically loads and returns the value stored in x.
func (x *Time) Load() time.Time {
	if x, ok := x.v.Load().(time.Time); ok {
		return x
	}
	return time.Time{}
}

// Store atomically stores val into x.
func (x *Time) Store(val time.Time) {
	x.v.Store(val)
}

// SetMarshalJSON sets the JSON encoding handler for x.
func (x *Time) SetMarshalJSON(fn MarshalTime) {
	x.marshalJSON = fn
}

// MarshalJSON returns the JSON encoding of x.
func (x *Time) MarshalJSON() ([]byte, error) {
	if x.marshalJSON != nil {
		return x.marshalJSON(x.Load())
	}
	return json.Marshal(x.Load().Format(time.UnixDate))
}
