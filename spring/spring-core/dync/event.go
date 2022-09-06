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

	"github.com/go-spring/spring-core/conf"
)

type EventFunc func(prop *conf.Properties) error
type EventValidateFunc func(prop *conf.Properties) error

type Event struct {
	Base
	f EventFunc
	h EventValidateFunc
	p *conf.Properties // just for init
}

func (e *Event) OnValidate(h EventValidateFunc) {
	e.h = h
}

func (e *Event) OnEvent(f EventFunc) error {
	prop := e.p
	if prop == nil {
		return nil
	}
	e.p = nil // release p
	e.f = f
	return e.onRefresh(prop)
}

func (e *Event) onRefresh(prop *conf.Properties) error {
	if e.f == nil {
		e.p = prop
		return nil
	}
	return e.f(prop)
}

func (e *Event) onValidate(prop *conf.Properties) error {
	if e.h != nil {
		return e.h(prop)
	}
	return nil
}

func (e *Event) MarshalJSON() ([]byte, error) {
	return json.Marshal(make(map[string]string))
}
