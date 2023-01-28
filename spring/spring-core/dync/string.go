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

var _ Value = (*String)(nil)

// A String is an atomic string value that can be dynamic refreshed.
type String struct {
	v atomic.String
}

// Value returns the stored string value.
func (x *String) Value() string {
	return x.v.Load()
}

// OnRefresh refreshes the stored value.
func (x *String) OnRefresh(p *conf.Properties, param conf.BindParam) error {
	var s string
	if err := p.Bind(&s, conf.Param(param)); err != nil {
		return err
	}
	x.v.Store(s)
	return nil
}

// MarshalJSON returns the JSON encoding of x.
func (x *String) MarshalJSON() ([]byte, error) {
	return json.Marshal(x.Value())
}
