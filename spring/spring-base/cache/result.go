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

package cache

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/go-spring/spring-base/json"
)

// Result stores a value.
type Result interface {
	JSON() (string, error)
	Load(v interface{}) error
}

// ValueResult stores a `reflect.Value`.
type ValueResult struct {
	v reflect.Value
	t reflect.Type
}

// NewValueResult returns a Result which stores a `reflect.Value`.
func NewValueResult(v interface{}) Result {
	return &ValueResult{
		v: reflect.ValueOf(v),
		t: reflect.TypeOf(v),
	}
}

// JSON returns the JSON encoding of the stored value.
func (r *ValueResult) JSON() (string, error) {
	b, err := json.Marshal(r.v.Interface())
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Load injects the saved value to v.
func (r *ValueResult) Load(v interface{}) error {
	outVal := reflect.ValueOf(v)
	if outVal.Kind() != reflect.Ptr || outVal.IsNil() {
		return errors.New("value should be ptr and not nil")
	}
	if outVal.Type().Elem() != r.t {
		return fmt.Errorf("load type (%s) but expect type (%s)", outVal.Elem().Type(), r.t.String())
	}
	outVal.Elem().Set(r.v)
	return nil
}

// JSONResult stores a JSON string.
type JSONResult struct {
	v string
}

// NewJSONResult returns a Result which stores a JSON string.
func NewJSONResult(v string) Result {
	return &JSONResult{
		v: v,
	}
}

// JSON returns the JSON encoding of the stored value.
func (r *JSONResult) JSON() (string, error) {
	return r.v, nil
}

// Load injects the saved value to v.
func (r *JSONResult) Load(v interface{}) error {
	return json.Unmarshal([]byte(r.v), v)
}
