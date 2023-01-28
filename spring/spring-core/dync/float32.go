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

	"github.com/go-spring/spring-base/atomic"
	"github.com/go-spring/spring-core/conf"
)

var _ Value = (*Float32)(nil)

// A Float32 is an atomic float32 value that can be dynamic refreshed.
type Float32 struct {
	v atomic.Float32
}

// Value returns the stored float32 value.
func (x *Float32) Value() float32 {
	return x.v.Load()
}

// OnRefresh refreshes the stored value.
func (x *Float32) OnRefresh(p *conf.Properties, param conf.BindParam) error {
	var f float32
	if err := p.Bind(&f, conf.Param(param)); err != nil {
		return err
	}
	x.v.Store(f)
	return nil
}

// MarshalJSON returns the JSON encoding of x.
func (x *Float32) MarshalJSON() ([]byte, error) {
	return json.Marshal(x.Value())
}
