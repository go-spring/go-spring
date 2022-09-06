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

type Uint32ValidateFunc func(v uint32) error

type Uint32 struct {
	Base
	v atomic.Uint32
	f Uint32ValidateFunc
}

func (x *Uint32) Value() uint32 {
	return x.v.Load()
}

func (x *Uint32) OnValidate(f Uint32ValidateFunc) {
	x.f = f
}

func (x *Uint32) getUint32(prop *conf.Properties) (uint32, error) {
	s, err := x.Property(prop)
	if err != nil {
		return 0, err
	}
	v, err := cast.ToUint64E(s)
	if err != nil {
		return 0, err
	}
	return uint32(v), nil
}

func (x *Uint32) onRefresh(prop *conf.Properties) error {
	v, err := x.getUint32(prop)
	if err != nil {
		return err
	}
	x.v.Store(v)
	return nil
}

func (x *Uint32) onValidate(prop *conf.Properties) error {
	v, err := x.getUint32(prop)
	if err != nil {
		return err
	}
	if x.f != nil {
		return x.f(v)
	}
	return nil
}

func (x *Uint32) MarshalJSON() ([]byte, error) {
	return json.Marshal(x.Value())
}
