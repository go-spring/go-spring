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
	"fmt"
	"reflect"

	"github.com/go-spring/spring-base/atomic"
	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-core/conf"
)

type Properties struct {
	value  atomic.Value
	fields []EventObserver
}

func New() *Properties {
	p := &Properties{}
	p.Refresh(conf.New())
	return p
}

func (p *Properties) Value() *conf.Properties {
	return p.value.Load().(*conf.Properties)
}

func (p *Properties) Refresh(prop *conf.Properties) {
	p.value.Store(prop)
	p.fireEvent()
}

func (p *Properties) fireEvent() {
	prop := p.Value()
	for _, field := range p.fields {
		err := field.OnEvent(prop)
		if err != nil { // TODO
			fmt.Println(err)
		}
	}
}

func (p *Properties) Watch(o EventObserver) error {
	p.fields = append(p.fields, o)
	return o.OnEvent(p.Value())
}

type EventObserver interface {
	OnEvent(prop *conf.Properties) error
}

type Value interface {
	EventObserver
	SetParam(param conf.BindParam)
}

type Base struct {
	conf.BindParam
}

func (v *Base) SetParam(param conf.BindParam) {
	v.BindParam = param
}

type Int64 struct {
	Base
	v atomic.Int64
}

func (i *Int64) Value() int64 {
	return i.v.Load()
}

func (i *Int64) OnEvent(prop *conf.Properties) error {
	key := i.BindParam.Key
	if !prop.Has(key) && !i.BindParam.Tag.HasDef {
		return fmt.Errorf("property %q not exist", key)
	}
	s := prop.Get(key, conf.Def(i.BindParam.Tag.Def))
	v, err := cast.ToInt64E(s)
	if err != nil {
		return err
	}
	i.v.Store(v)
	return nil
}

func (i *Int64) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.Value())
}

type Float64 struct {
	Base
	v atomic.Float64
}

func (i *Float64) Value() float64 {
	return i.v.Load()
}

func (i *Float64) OnEvent(prop *conf.Properties) error {
	key := i.BindParam.Key
	if !prop.Has(key) && !i.BindParam.Tag.HasDef {
		return fmt.Errorf("property %q not exist", key)
	}
	s := prop.Get(key, conf.Def(i.BindParam.Tag.Def))
	v, err := cast.ToFloat64E(s)
	if err != nil {
		return err
	}
	i.v.Store(v)
	return nil
}

func (i *Float64) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.Value())
}

type Ref struct {
	Base
	v atomic.Value
	p *conf.Properties // just for init
}

func (r *Ref) Init(i interface{}) error {
	prop := r.p
	r.p = nil // release p
	r.v.Store(i)
	return r.OnEvent(prop)
}

func (r *Ref) Value() interface{} {
	return r.v.Load()
}

func (r *Ref) OnEvent(prop *conf.Properties) error {
	o := r.Value()
	if o == nil {
		r.p = prop
		return nil
	}
	t := reflect.TypeOf(o)
	v := reflect.New(t)
	err := prop.Bind(v.Interface(), conf.Key(r.BindParam.Key))
	if err != nil {
		return err
	}
	r.v.Store(v.Elem().Interface())
	return nil
}

func (r *Ref) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.Value())
}

type EventFunc func(prop *conf.Properties) error

type Event struct {
	Base
	f EventFunc
	p *conf.Properties // just for init
}

func (e *Event) Init(fn EventFunc) error {
	prop := e.p
	e.p = nil // release p
	e.f = fn
	return e.OnEvent(prop)
}

func (e *Event) OnEvent(prop *conf.Properties) error {
	if e.f == nil {
		e.p = prop
		return nil
	}
	return e.f(prop)
}
