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

package redis

import (
	"fmt"
	"strconv"
)

func toBool(v interface{}) (bool, error) {
	switch r := v.(type) {
	case int64:
		return r == 1, nil
	case string:
		return r == "OK", nil
	default:
		return false, fmt.Errorf("redis: unexpected type %T for bool", v)
	}
}

func Bool(v interface{}, err error) (bool, error) {
	if err != nil {
		return false, err
	}
	return toBool(v)
}

func toInt64(v interface{}) (int64, error) {
	switch r := v.(type) {
	case int64:
		return r, nil
	//case string:
	//	return r == "OK", nil
	default:
		return 0, fmt.Errorf("redis: unexpected type %T for bool", v)
	}
}

func Int64(v interface{}, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	return toInt64(v)
}

func toFloat64(v interface{}) (float64, error) {
	switch r := v.(type) {
	case nil:
		return 0, nil
	case int64:
		return float64(r), nil
	case string:
		return strconv.ParseFloat(r, 64)
	default:
		return 0, fmt.Errorf("redis: unexpected type=%T for Float64", r)
	}
}

func Float64(v interface{}, err error) (float64, error) {
	if err != nil {
		return 0, err
	}
	return toFloat64(v)
}

func toString(v interface{}) (string, error) {
	switch r := v.(type) {
	case string:
		return r, nil
	default:
		return "", fmt.Errorf("redis: unexpected type %T for bool", v)
	}
}

func String(v interface{}, err error) (string, error) {
	if err != nil {
		return "", err
	}
	return toString(v)
}

func Slice(v interface{}, err error) ([]interface{}, error) {
	if err != nil {
		return nil, err
	}
	switch r := v.(type) {
	case []interface{}:
		return r, nil
	default:
		return nil, fmt.Errorf("redis: unexpected type %T for bool", v)
	}
}

func BoolSlice(v interface{}, err error) ([]bool, error) {
	slice, err := Slice(v, err)
	if err != nil {
		return nil, err
	}
	val := make([]bool, len(slice))
	for i, r := range slice {
		var b bool
		b, err = toBool(r)
		if err != nil {
			return nil, err
		}
		val[i] = b
	}
	return val, nil
}

func Int64Slice(v interface{}, err error) ([]int64, error) {
	slice, err := Slice(v, err)
	if err != nil {
		return nil, err
	}
	val := make([]int64, len(slice))
	for i, r := range slice {
		var n int64
		n, err = toInt64(r)
		if err != nil {
			return nil, err
		}
		val[i] = n
	}
	return val, nil
}

func Float64Slice(v interface{}, err error) ([]float64, error) {
	slice, err := Slice(v, err)
	if err != nil {
		return nil, err
	}
	val := make([]float64, len(slice))
	for i, r := range slice {
		var f float64
		f, err = toFloat64(r)
		if err != nil {
			return nil, err
		}
		val[i] = f
	}
	return val, nil
}

func StringSlice(v interface{}, err error) ([]string, error) {
	slice, err := Slice(v, err)
	if err != nil {
		return nil, err
	}
	val := make([]string, len(slice))
	for i, r := range slice {
		var str string
		str, err = toString(r)
		if err != nil {
			return nil, err
		}
		val[i] = str
	}
	return val, nil
}

func StringMap(v interface{}, err error) (map[string]string, error) {
	slice, err := StringSlice(v, err)
	if err != nil {
		return nil, err
	}
	val := make(map[string]string, len(slice)/2)
	for i := 0; i < len(slice); i += 2 {
		val[slice[i]] = slice[i+1]
	}
	return val, nil
}

func ZItemSlice(v interface{}, err error) ([]ZItem, error) {
	slice, err := StringSlice(v, err)
	if err != nil {
		return nil, err
	}
	val := make([]ZItem, len(slice)/2)
	for i := 0; i < len(val); i++ {
		idx := i * 2
		member := slice[idx]
		score, err := strconv.ParseFloat(slice[idx+1], 64)
		if err != nil {
			return nil, err
		}
		val[i].Member = member
		val[i].Score = score
	}
	return val, nil
}
