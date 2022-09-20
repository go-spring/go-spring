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
	"sort"
	"strings"

	"github.com/go-spring/spring-base/atomic"
	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/expr"
)

// Value 可动态刷新的对象
type Value interface {
	Refresh(prop *conf.Properties, param conf.BindParam) error
	Validate(prop *conf.Properties, param conf.BindParam) error
}

type Field struct {
	value Value
	param conf.BindParam
}

// Properties 动态属性
type Properties struct {
	value  atomic.Value
	fields []*Field
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

func (p *Properties) Update(m map[string]interface{}) error {

	flat := make(map[string]string)
	for key, val := range m {
		err := conf.Flatten(key, val, flat)
		if err != nil {
			return err
		}
	}

	keys := make([]string, 0, len(flat))
	for k := range flat {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	prop := p.load().Copy()
	for _, k := range keys {
		err := prop.Set(k, flat[k])
		if err != nil {
			return err
		}
	}
	return p.refreshKeys(prop, keys)
}

func (p *Properties) Refresh(prop *conf.Properties) (err error) {

	old := p.load()
	oldKeys := old.Keys()
	newKeys := prop.Keys()

	changes := make(map[string]struct{})
	{
		for _, k := range newKeys {
			if !old.Has(k) || old.Get(k) != prop.Get(k) {
				changes[k] = struct{}{}
			}
		}
		for _, k := range oldKeys {
			if _, ok := changes[k]; !ok {
				changes[k] = struct{}{}
			}
		}
	}

	keys := make([]string, 0, len(changes))
	for k := range changes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return p.refreshKeys(prop, keys)
}

func (p *Properties) refreshKeys(prop *conf.Properties, keys []string) (err error) {

	updateIndexes := make(map[int]*Field)
	for _, key := range keys {
		for index, field := range p.fields {
			s := strings.TrimPrefix(key, field.param.Key)
			if len(s) == len(key) {
				continue
			}
			if len(s) == 0 || s[0] == '.' || s[0] == '[' {
				if _, ok := updateIndexes[index]; !ok {
					updateIndexes[index] = field
				}
			}
		}
	}

	updateFields := make([]*Field, 0, len(updateIndexes))
	{
		ints := make([]int, 0, len(updateIndexes))
		for k := range updateIndexes {
			ints = append(ints, k)
		}
		sort.Ints(ints)
		for _, k := range ints {
			updateFields = append(updateFields, updateIndexes[k])
		}
	}

	return p.refreshFields(prop, updateFields)
}

func (p *Properties) refreshFields(prop *conf.Properties, fields []*Field) (err error) {

	err = validateFields(prop, fields)
	if err != nil {
		return
	}

	old := p.load()
	defer func() {
		if r := recover(); err != nil || r != nil {
			if err == nil {
				err = fmt.Errorf("%v", r)
			}
			p.value.Store(old)
			_ = refreshFields(old, fields)
		}
	}()

	p.value.Store(prop)
	return refreshFields(p.load(), fields)
}

func validateFields(prop *conf.Properties, fields []*Field) error {
	for _, f := range fields {
		err := f.value.Validate(prop, f.param)
		if err != nil {
			return err
		}
	}
	return nil
}

func refreshFields(prop *conf.Properties, fields []*Field) error {
	for _, f := range fields {
		err := f.value.Refresh(prop, f.param)
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

	prop := p.load()
	err := v.Validate(prop, param)
	if err != nil {
		return false, err
	}
	err = v.Refresh(prop, param)
	if err != nil {
		return false, err
	}

	p.fields = append(p.fields, &Field{
		value: v,
		param: param,
	})
	return true, nil
}

func GetProperty(prop *conf.Properties, param conf.BindParam) (string, error) {
	key := param.Key
	if !prop.Has(key) && !param.Tag.HasDef {
		return "", fmt.Errorf("property %q not exist", key)
	}
	s := prop.Get(key, conf.Def(param.Tag.Def))
	return s, nil
}

func Validate(val interface{}, param conf.BindParam) error {
	if param.Validate == "" {
		return nil
	}
	if b, err := expr.Eval(param.Validate, val); err != nil {
		return err
	} else if !b {
		return fmt.Errorf("validate failed on %q for value %v", param.Validate, val)
	}
	return nil
}
