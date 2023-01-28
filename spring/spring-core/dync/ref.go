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
	"reflect"

	"github.com/go-spring/spring-base/atomic"
	"github.com/go-spring/spring-core/conf"
)

var _ Value = (*Ref)(nil)

// A Ref is an atomic reference value that can be dynamic refreshed.
type Ref struct {
	v       atomic.Value
	OnEvent func() error
}

// Init inits the stored reference value.
func (r *Ref) Init(i interface{}) {
	r.v.Store(i)
}

// Value returns the stored reference value.
func (r *Ref) Value() interface{} {
	return r.v.Load()
}

// OnRefresh refreshes the stored value.
func (r *Ref) OnRefresh(p *conf.Properties, param conf.BindParam) error {
	t := reflect.TypeOf(r.Value())
	v := reflect.New(t)
	err := conf.BindValue(p, v.Elem(), t, param, nil)
	if err != nil {
		return err
	}
	r.v.Store(v.Elem().Interface())
	if r.OnEvent != nil {
		return r.OnEvent()
	}
	return nil
}

// MarshalJSON returns the JSON encoding of x.
func (r *Ref) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.Value())
}
