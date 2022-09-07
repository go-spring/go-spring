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
	"fmt"
	"reflect"

	"github.com/go-spring/spring-base/atomic"
	"github.com/go-spring/spring-core/conf"
)

// Value 可动态刷新的对象
type Value interface {
	setParam(param conf.BindParam)
	onRefresh(prop *conf.Properties) error
	onValidate(prop *conf.Properties) error
}

// Properties 动态属性
type Properties struct {
	value  atomic.Value
	fields []Value
}

func New() *Properties {
	p := &Properties{}
	p.value.Store(conf.New())
	return p
}

func (p *Properties) load() *conf.Properties {
	return p.value.Load().(*conf.Properties)
}

func (p *Properties) Keys() []string {
	return p.load().Keys()
}

func (p *Properties) Has(key string) bool {
	return p.load().Has(key)
}

func (p *Properties) Get(key string, opts ...conf.GetOption) string {
	return p.load().Get(key, opts...)
}

func (p *Properties) Resolve(s string) (string, error) {
	return p.load().Resolve(s)
}

func (p *Properties) Bind(i interface{}, opts ...conf.BindOption) error {
	return p.load().Bind(i, opts...)
}

func (p *Properties) Refresh(prop *conf.Properties) (err error) {

	if err = p.validate(prop); err != nil {
		return
	}

	old := p.load()
	defer func() {
		if r := recover(); err != nil || r != nil {
			if err == nil {
				err = fmt.Errorf("%v", r)
			}
			p.value.Store(old)
			_ = p.refresh(old)
		}
	}()

	p.value.Store(prop)
	return p.refresh(p.load())
}

func (p *Properties) validate(prop *conf.Properties) error {
	for _, field := range p.fields {
		err := field.onValidate(prop)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Properties) refresh(prop *conf.Properties) error {
	for _, field := range p.fields {
		err := field.onRefresh(prop)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Properties) BindValue(v reflect.Value, param conf.BindParam) error {
	if v.Kind() == reflect.Ptr {
		ok, err := p.bindValue(v.Interface(), param)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
	return conf.BindValue(p.load(), v.Elem(), v.Elem().Type(), param, p.bindValue)
}

func (p *Properties) bindValue(i interface{}, param conf.BindParam) (bool, error) {

	v, ok := i.(Value)
	if !ok {
		return false, nil
	}
	v.setParam(param)

	prop := p.load()
	err := v.onValidate(prop)
	if err != nil {
		return false, err
	}
	err = v.onRefresh(prop)
	if err != nil {
		return false, err
	}

	p.fields = append(p.fields, v)
	return true, nil
}
