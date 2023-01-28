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

package dync

import (
	"encoding/json"
	"time"

	"github.com/go-spring/spring-base/atomic"
	"github.com/go-spring/spring-core/conf"
)

var _ Value = (*Time)(nil)

// A Time is an atomic time.Time value that can be dynamic refreshed.
type Time struct {
	v atomic.Time
}

// Value returns the stored time.Time value.
func (x *Time) Value() time.Time {
	return x.v.Load()
}

// OnRefresh refreshes the stored value.
func (x *Time) OnRefresh(p *conf.Properties, param conf.BindParam) error {
	var t time.Time
	if err := p.Bind(&t, conf.Param(param)); err != nil {
		return err
	}
	x.v.Store(t)
	return nil
}

// MarshalJSON returns the JSON encoding of x.
func (x *Time) MarshalJSON() ([]byte, error) {
	return json.Marshal(x.Value())
}
