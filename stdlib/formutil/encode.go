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
	"net/url"
	"strconv"

	"github.com/go-spring/stdlib/jsonflow"
)

// EncodeBool encodes a boolean value into url.Values.
func EncodeBool(m url.Values, key string, val bool) error {
	m.Add(key, strconv.FormatBool(val))
	return nil
}

// EncodeBoolPtr encodes a boolean pointer value into url.Values.
// If the pointer is nil, the field is omitted.
func EncodeBoolPtr(m url.Values, key string, val *bool) error {
	if val != nil {
		m.Add(key, strconv.FormatBool(*val))
	}
	return nil
}

// EncodeInt encodes a signed integer value into url.Values.
func EncodeInt[T ~int64 | ~int32 | ~int16 | ~int8 | ~int](m url.Values, key string, val T) error {
	m.Add(key, strconv.FormatInt(int64(val), 10))
	return nil
}

// EncodeIntPtr encodes a signed integer pointer value into url.Values.
// If the pointer is nil, the field is omitted.
func EncodeIntPtr[T ~int64 | ~int32 | ~int16 | ~int8 | ~int](m url.Values, key string, val *T) error {
	if val != nil {
		m.Add(key, strconv.FormatInt(int64(*val), 10))
	}
	return nil
}

// EncodeUint encodes an unsigned integer value into url.Values.
func EncodeUint[T ~uint64 | ~uint32 | ~uint16 | ~uint8 | ~uint](m url.Values, key string, val T) error {
	m.Add(key, strconv.FormatUint(uint64(val), 10))
	return nil
}

// EncodeUintPtr encodes an unsigned integer pointer value into url.Values.
// If the pointer is nil, the field is omitted.
func EncodeUintPtr[T ~uint64 | ~uint32 | ~uint16 | ~uint8 | ~uint](m url.Values, key string, val *T) error {
	if val != nil {
		m.Add(key, strconv.FormatUint(uint64(*val), 10))
	}
	return nil
}

// EncodeFloat encodes a floating-point value into url.Values.
func EncodeFloat[T ~float64 | ~float32](m url.Values, key string, val T) error {
	m.Add(key, strconv.FormatFloat(float64(val), 'f', -1, 64))
	return nil
}

// EncodeFloatPtr encodes a floating-point pointer value into url.Values.
// If the pointer is nil, the field is omitted.
func EncodeFloatPtr[T ~float64 | ~float32](m url.Values, key string, val *T) error {
	if val != nil {
		m.Add(key, strconv.FormatFloat(float64(*val), 'f', -1, 64))
	}
	return nil
}

// EncodeString encodes a string value into url.Values.
func EncodeString(m url.Values, key string, val string) error {
	m.Add(key, val)
	return nil
}

// EncodeStringPtr encodes a string pointer value into url.Values.
// If the pointer is nil, the field is omitted.
func EncodeStringPtr(m url.Values, key string, val *string) error {
	if val != nil {
		m.Add(key, *val)
	}
	return nil
}

// EncodeBytes encodes a byte slice into url.Values using base64 encoding.
func EncodeBytes(m url.Values, key string, val []byte) error {
	m.Add(key, base64.StdEncoding.EncodeToString(val))
	return nil
}

// EncodeJSON encodes a value into JSON and stores it in url.Values.
func EncodeJSON[T any](m url.Values, key string, val T) error {
	b, err := jsonflow.Marshal(val)
	if err != nil {
		return err
	}
	m.Add(key, string(b))
	return nil
}

// Encoder is a function type that encodes a value into url.Values.
type Encoder[T any] func(m url.Values, key string, val T) error

// EncodeList encodes a list of values into url.Values using the provided encoder function.
// Each element in the slice is encoded independently.
func EncodeList[T any](m url.Values, key string, values []T, fn Encoder[T]) error {
	for _, val := range values {
		if err := fn(m, key, val); err != nil {
			return err
		}
	}
	return nil
}
