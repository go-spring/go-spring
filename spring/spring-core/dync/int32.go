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
	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-core/conf"
)

type Int32ValidateFunc func(v int32) error

type Int32 struct {
	Base
	v atomic.Int32
	f Int32ValidateFunc
}

func (x *Int32) Value() int32 {
	return x.v.Load()
}

func (x *Int32) OnValidate(f Int32ValidateFunc) {
	x.f = f
}

func (x *Int32) getInt32(prop *conf.Properties) (string, int32, error) {
	s, err := x.Property(prop)
	if err != nil {
		return "", 0, err
	}
	v, err := cast.ToInt64E(s)
	if err != nil {
		return "", 0, err
	}
	return s, int32(v), nil
}

func (x *Int32) onRefresh(prop *conf.Properties) error {
	_, v, err := x.getInt32(prop)
	if err != nil {
		return err
	}
	x.v.Store(v)
	return nil
}

func (x *Int32) onValidate(prop *conf.Properties) error {
	s, v, err := x.getInt32(prop)
	if err != nil {
		return err
	}
	err = x.Validate(s)
	if err != nil {
		return err
	}
	if x.f != nil {
		return x.f(v)
	}
	return nil
}

func (x *Int32) MarshalJSON() ([]byte, error) {
	return json.Marshal(x.Value())
}
