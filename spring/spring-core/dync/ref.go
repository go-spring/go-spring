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
	Base
	v atomic.Value
	f RefValidateFunc
	p *conf.Properties // just for init
}

func (r *Ref) Init(i interface{}) error {
	r.v.Store(i)
	prop := r.p
	if prop == nil {
		return nil
	}
	r.p = nil // release p
	return r.onRefresh(prop)
}

func (r *Ref) Value() interface{} {
	return r.v.Load()
}

func (r *Ref) OnValidate(f RefValidateFunc) {
	r.f = f
}

func (r *Ref) getRef(prop *conf.Properties) (interface{}, error) {
	o := r.Value()
	if o == nil {
		r.p = prop
		return nil, nil
	}
	t := reflect.TypeOf(o)
	v := reflect.New(t)
	err := conf.BindValue(prop, v.Elem(), t, r.param, nil)
	if err != nil {
		return nil, err
	}
	return v.Elem().Interface(), nil
}

func (r *Ref) onRefresh(prop *conf.Properties) error {
	v, err := r.getRef(prop)
	if err != nil {
		return err
	}
	if v == nil {
		return nil
	}
	r.v.Store(v)
	return nil
}

func (r *Ref) onValidate(prop *conf.Properties) error {
	v, err := r.getRef(prop)
	if r.f != nil {
		return r.f(v)
	}
	return err
}

func (r *Ref) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.Value())
}
