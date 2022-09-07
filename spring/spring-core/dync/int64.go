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

type Int64ValidateFunc func(v int64) error

type Int64 struct {
	Base
	v atomic.Int64
	f Int64ValidateFunc
}

func (x *Int64) Value() int64 {
	return x.v.Load()
}

func (x *Int64) OnValidate(f Int64ValidateFunc) {
	x.f = f
}

func (x *Int64) getInt64(prop *conf.Properties) (string, int64, error) {
	s, err := x.Property(prop)
	if err != nil {
		return "", 0, err
	}
	v, err := cast.ToInt64E(s)
	if err != nil {
		return "", 0, err
	}
	return s, v, nil
}

func (x *Int64) onRefresh(prop *conf.Properties) error {
	_, v, err := x.getInt64(prop)
	if err != nil {
		return err
	}
	x.v.Store(v)
	return nil
}

func (x *Int64) onValidate(prop *conf.Properties) error {
	s, v, err := x.getInt64(prop)
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

func (x *Int64) MarshalJSON() ([]byte, error) {
	return json.Marshal(x.Value())
}
