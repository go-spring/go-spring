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

type Bool struct {
	Base
	v atomic.Bool
}

func (x *Bool) Value() bool {
	return x.v.Load()
}

func (x *Bool) getBool(prop *conf.Properties) (bool, error) {
	s, err := x.Property(prop)
	if err != nil {
		return false, err
	}
	v, err := cast.ToBoolE(s)
	if err != nil {
		return false, err
	}
	return v, nil
}

func (x *Bool) onRefresh(prop *conf.Properties) error {
	v, err := x.getBool(prop)
	if err != nil {
		return err
	}
	x.v.Store(v)
	return nil
}

func (x *Bool) onValidate(prop *conf.Properties) error {
	_, err := x.getBool(prop)
	return err
}

func (x *Bool) MarshalJSON() ([]byte, error) {
	return json.Marshal(x.Value())
}
