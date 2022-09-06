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
	"time"

	"github.com/go-spring/spring-base/atomic"
	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-core/conf"
)

type TimeValidateFunc func(v time.Time) error

type Time struct {
	Base
	v atomic.Time
	f TimeValidateFunc
}

func (x *Time) Value() time.Time {
	return x.v.Load()
}

func (x *Time) OnValidate(f TimeValidateFunc) {
	x.f = f
}

func (x *Time) getTime(prop *conf.Properties) (time.Time, error) {
	s, err := x.Property(prop)
	if err != nil {
		return time.Time{}, err
	}
	v, err := cast.ToTimeE(s)
	if err != nil {
		return time.Time{}, err
	}
	return v, nil
}

func (x *Time) onRefresh(prop *conf.Properties) error {
	v, err := x.getTime(prop)
	if err != nil {
		return err
	}
	x.v.Store(v)
	return nil
}

func (x *Time) onValidate(prop *conf.Properties) error {
	v, err := x.getTime(prop)
	if err != nil {
		return err
	}
	if x.f != nil {
		return x.f(v)
	}
	return nil
}

func (x *Time) MarshalJSON() ([]byte, error) {
	return json.Marshal(x.Value())
}
