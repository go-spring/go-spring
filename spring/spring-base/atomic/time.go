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

import "time"

type Time struct {
	v Value
}

func NewTime(val time.Time) *Time {
	t := &Time{}
	t.Store(val)
	return t
}

func (t *Time) Load() time.Time {
	if x, ok := t.v.Load().(time.Time); ok {
		return x
	}
	return time.Time{}
}

func (t *Time) Store(x time.Time) {
	t.v.Store(x)
}
