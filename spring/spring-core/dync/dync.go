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
	"reflect"
	"sort"
	"strings"

	"github.com/go-spring/spring-base/atomic"
	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-core/conf"
)

// A Value represents a refreshable type.
type Value interface {
	OnRefresh(p *conf.Properties, param conf.BindParam) error
}

// A Field represents a refreshable struct field.
type Field struct {
	value Value
	param conf.BindParam
}

// Properties refreshes registered fields dynamically and concurrently.
type Properties struct {
	value  atomic.Value
	fields []*Field
}

// New returns a Properties.
func New() *Properties {
	p := &Properties{}
	p.value.Store(conf.New())
	return p
}

func (p *Properties) load() *conf.Properties {
	return p.value.Load().(*conf.Properties)
}

// Keys returns all sorted keys.
func (p *Properties) Keys() []string {
	return p.load().Keys()
}

// Has returns whether key exists.
func (p *Properties) Has(key string) bool {
	return p.load().Has(key)
}

// Get returns key's value.
func (p *Properties) Get(key string, opts ...conf.GetOption) string {
	return p.load().Get(key, opts...)
}

// Resolve resolves string value.
func (p *Properties) Resolve(s string) (string, error) {
	return p.load().Resolve(s)
}

// Bind binds properties to a value.
func (p *Properties) Bind(i interface{}, args ...conf.BindArg) error {
	return p.load().Bind(i, args...)
}

// Refresh refreshes new Properties atomically.
func (p *Properties) Refresh(prop *conf.Properties) (err error) {

	old := p.load()
	p.value.Store(prop)

	if len(p.fields) == 0 {
		return nil
	}

	oldKeys := old.Keys()
	newKeys := prop.Keys()

	changes := make(map[string]struct{})
	{
		// property value has changed.
		for _, k := range newKeys {
			if !old.Has(k) || old.Get(k) != prop.Get(k) {
				changes[k] = struct{}{}
			}
		}
		// property key has deleted.
		for _, k := range oldKeys {
			if _, ok := changes[k]; !ok {
				changes[k] = struct{}{}
			}
		}
	}

	keys := util.SortedKeys(changes)
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

func (p *Properties) refreshFields(prop *conf.Properties, fields []*Field) error {
	for _, f := range fields {
		err := f.value.OnRefresh(prop, f.param)
		if err != nil {
			return err
		}
	}
	return nil
}

// BindValue binds properties to a value.
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

	err := v.OnRefresh(p.load(), param)
	if err != nil {
		return false, err
	}

	p.fields = append(p.fields, &Field{
		value: v,
		param: param,
	})
	return true, nil
}
