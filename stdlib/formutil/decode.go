/*
 * Copyright 2025 The Go-Spring Authors.
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

package formutil

import (
	"encoding/base64"
	"strconv"

	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/jsonflow"
	"go-spring.org/stdlib/mathutil"
)

// DecodeBool decodes a boolean value from form values.
func DecodeBool(key string, values []string) (bool, error) {
	if len(values) > 1 {
		return false, errutil.Explain(nil, "too many values for form field %s", key)
	}
	return strconv.ParseBool(values[0])
}

// DecodeBoolPtr decodes a boolean value and returns a pointer to it.
func DecodeBoolPtr(key string, values []string) (*bool, error) {
	b, err := DecodeBool(key, values)
	if err != nil {
		return nil, err
	}
	return new(b), nil
}

// DecodeInt decodes a signed integer value from form values.
func DecodeInt[T ~int64 | ~int32 | ~int16 | ~int8 | ~int](key string, values []string) (T, error) {
	if len(values) > 1 {
		return 0, errutil.Explain(nil, "too many values for form field %s", key)
	}
	i, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil {
		return 0, err
	}
	if mathutil.OverflowInt[T](i) {
		return 0, errutil.Explain(nil, "overflow for form field %s", key)
	}
	return T(i), nil
}

// DecodeIntPtr decodes a signed integer value and returns a pointer to it.
func DecodeIntPtr[T ~int64 | ~int32 | ~int16 | ~int8 | ~int](key string, values []string) (*T, error) {
	i, err := DecodeInt[T](key, values)
	if err != nil {
		return nil, err
	}
	return new(i), nil
}

// DecodeUint decodes an unsigned integer value from form values.
func DecodeUint[T ~uint64 | ~uint32 | ~uint16 | ~uint8 | ~uint](key string, values []string) (T, error) {
	if len(values) > 1 {
		return 0, errutil.Explain(nil, "too many values for form field %s", key)
	}
	u, err := strconv.ParseUint(values[0], 10, 64)
	if err != nil {
		return 0, err
	}
	if mathutil.OverflowUint[T](u) {
		return 0, errutil.Explain(nil, "overflow for form field %s", key)
	}
	return T(u), nil
}

// DecodeUintPtr decodes an unsigned integer value and returns a pointer to it.
func DecodeUintPtr[T ~uint64 | ~uint32 | ~uint16 | ~uint8 | ~uint](key string, values []string) (*T, error) {
	u, err := DecodeUint[T](key, values)
	if err != nil {
		return nil, err
	}
	return new(u), nil
}

// DecodeFloat decodes a floating-point value from form values.
func DecodeFloat[T ~float64 | ~float32](key string, values []string) (T, error) {
	if len(values) > 1 {
		return 0, errutil.Explain(nil, "too many values for form field %s", key)
	}
	f, err := strconv.ParseFloat(values[0], 64)
	if err != nil {
		return 0, err
	}
	if mathutil.OverflowFloat[T](f) {
		return 0, errutil.Explain(nil, "overflow for form field %s", key)
	}
	return T(f), nil
}

// DecodeFloatPtr decodes a floating-point value and returns a pointer to it.
func DecodeFloatPtr[T ~float64 | ~float32](key string, values []string) (*T, error) {
	f, err := DecodeFloat[T](key, values)
	if err != nil {
		return nil, err
	}
	return new(f), nil
}

// DecodeString decodes a string value from form values.
func DecodeString(key string, values []string) (string, error) {
	if len(values) > 1 {
		return "", errutil.Explain(nil, "too many values for form field %s", key)
	}
	return values[0], nil
}

// DecodeStringPtr decodes a string value and returns a pointer to it.
func DecodeStringPtr(key string, values []string) (*string, error) {
	if len(values) > 1 {
		return nil, errutil.Explain(nil, "too many values for form field %s", key)
	}
	return new(values[0]), nil
}

// DecodeBytes decodes a base64-encoded string into a byte slice.
func DecodeBytes(key string, values []string) ([]byte, error) {
	if len(values) > 1 {
		return nil, errutil.Explain(nil, "too many values for form field %s", key)
	}
	return base64.StdEncoding.DecodeString(values[0])
}

// DecodeJSON decodes a JSON-encoded value into the target type.
func DecodeJSON[T any](key string, values []string) (T, error) {
	var v T
	if len(values) > 1 {
		return v, errutil.Explain(nil, "too many values for form field %s", key)
	}
	if err := jsonflow.Unmarshal([]byte(values[0]), &v); err != nil {
		return v, err
	}
	return v, nil
}

// Decoder defines a generic decoder function for a single form value.
type Decoder[T any] func(key string, values []string) (T, error)

// DecodeList decodes multiple form values into a slice using the provided decoder.
func DecodeList[T any](key string, values []string, fn Decoder[T]) ([]T, error) {
	arr := make([]T, 0, len(values))
	for i := range len(values) {
		v, err := fn(key, values[i:i+1])
		if err != nil {
			return nil, err
		}
		arr = append(arr, v)
	}
	return arr, nil
}
