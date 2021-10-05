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

// Package cast 提供了很多类型之间相互转换的函数。
// Thanks the github.com/spf13/cast project.
package cast

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/spf13/cast"
)

// ToBool casts an interface{} to a bool.
func ToBool(i interface{}) bool {
	v, _ := ToBoolE(i)
	return v
}

// ToBoolE casts an interface{} to a bool.
func ToBoolE(i interface{}) (bool, error) {
	return cast.ToBoolE(i)
}

// ToInt casts an interface{} to an int.
func ToInt(i interface{}) int {
	v, _ := ToInt64E(i)
	return int(v)
}

// ToInt64 casts an interface{} to an int64.
func ToInt64(i interface{}) int64 {
	v, _ := ToInt64E(i)
	return v
}

// ToInt64E casts an interface{} to an int64.
func ToInt64E(i interface{}) (int64, error) {
	return cast.ToInt64E(i)
}

// ToUint64 casts an interface{} to a uint64.
func ToUint64(i interface{}) uint64 {
	v, _ := ToUint64E(i)
	return v
}

// ToUint64E casts an interface{} to a uint64.
func ToUint64E(i interface{}) (uint64, error) {
	return cast.ToUint64E(i)
}

// ToFloat64 casts an interface{} to a float64.
func ToFloat64(i interface{}) float64 {
	v, _ := ToFloat64E(i)
	return v
}

// ToFloat64E casts an interface{} to a float64.
func ToFloat64E(i interface{}) (float64, error) {
	return cast.ToFloat64E(i)
}

// ToString casts an interface{} to a string.
func ToString(i interface{}) string {
	v, err := ToStringE(i)
	if err != nil {
		return err.Error()
	}
	return v
}

// ToStringE casts an interface{} to a string.
func ToStringE(i interface{}) (string, error) {
	str, err := cast.ToStringE(i)
	if err == nil {
		return str, nil
	}
	b, err := json.Marshal(i)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ToDuration casts an interface{} to a time.Duration.
func ToDuration(i interface{}) time.Duration {
	v, _ := ToDurationE(i)
	return v
}

// ToDurationE casts an interface{} to a time.Duration.
func ToDurationE(i interface{}) (time.Duration, error) {
	return cast.ToDurationE(i)
}

// ToTime casts an interface{} to a time.Time.
func ToTime(i interface{}) time.Time {
	v, _ := ToTimeE(i)
	return v
}

// ToTimeE casts an interface{} to a time.Time.
func ToTimeE(i interface{}) (time.Time, error) {
	return cast.ToTimeE(i)
}

// ToStringSlice casts an interface to a []string type.
func ToStringSlice(i interface{}) []string {
	v, _ := ToStringSliceE(i)
	return v
}

// ToStringSliceE casts an interface to a []string type.
func ToStringSliceE(i interface{}) ([]string, error) {
	// TODO 使用具体的类型判断，看看是否有更好的性能。
	switch v := reflect.ValueOf(i); v.Kind() {
	case reflect.Slice, reflect.Array:
		var slice []string
		for j := 0; j < v.Len(); j++ {
			s := ToString(v.Index(j).Interface())
			slice = append(slice, s)
		}
		return slice, nil
	}
	return nil, fmt.Errorf("unable to cast %#v of type %T to []string", i, i)
}
