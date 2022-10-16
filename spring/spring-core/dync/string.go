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
	"github.com/go-spring/spring-core/validate"
)

type StringValidateFunc func(v string) error

type String struct {
	v atomic.String
	f StringValidateFunc
}

func (x *String) Value() string {
	return x.v.Load()
}

func (x *String) OnValidate(f StringValidateFunc) {
	x.f = f
}

func (x *String) getString(prop *conf.Properties, param conf.BindParam) (string, error) {
	return GetProperty(prop, param)
}

func (x *String) Refresh(prop *conf.Properties, param conf.BindParam) error {
	v, err := x.getString(prop, param)
	if err != nil {
		return err
	}
	x.v.Store(v)
	return nil
}

func (x *String) Validate(prop *conf.Properties, param conf.BindParam) error {
	v, err := x.getString(prop, param)
	if err != nil {
		return err
	}
	err = validate.Field(v, param.Validate)
	if err != nil {
		return err
	}
	if x.f != nil {
		return x.f(v)
	}
	return nil
}

func (x *String) MarshalJSON() ([]byte, error) {
	return json.Marshal(x.Value())
}
