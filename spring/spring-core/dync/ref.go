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

type RefValidateFunc func(v interface{}) error

type Ref struct {
	v    atomic.Value
	f    RefValidateFunc
	init func() (*conf.Properties, conf.BindParam)
}

func (r *Ref) Init(i interface{}) error {
	r.v.Store(i)
	if r.init == nil {
		return nil
	}
	prop, param := r.init()
	r.init = nil
	return r.Refresh(prop, param)
}

func (r *Ref) Value() interface{} {
	return r.v.Load()
}

func (r *Ref) OnValidate(f RefValidateFunc) {
	r.f = f
}

func (r *Ref) getRef(prop *conf.Properties, param conf.BindParam) (interface{}, error) {
	o := r.Value()
	if o == nil {
		r.init = func() (*conf.Properties, conf.BindParam) {
			return prop, param
		}
		return nil, nil
	}
	t := reflect.TypeOf(o)
	v := reflect.New(t)
	err := conf.BindValue(prop, v.Elem(), t, param, nil)
	if err != nil {
		return nil, err
	}
	return v.Elem().Interface(), nil
}

func (r *Ref) Refresh(prop *conf.Properties, param conf.BindParam) error {
	v, err := r.getRef(prop, param)
	if err != nil {
		return err
	}
	if v == nil {
		return nil
	}
	r.v.Store(v)
	return nil
}

func (r *Ref) Validate(prop *conf.Properties, param conf.BindParam) error {
	v, err := r.getRef(prop, param)
	if r.f != nil {
		return r.f(v)
	}
	return err
}

func (r *Ref) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.Value())
}
