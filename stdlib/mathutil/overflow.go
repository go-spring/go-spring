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

package mathutil

import (
	"math"
)

// OverflowInt checks whether an int64 value exceeds the bounds of the target integer type T.
func OverflowInt[T ~int | ~int8 | ~int16 | ~int32 | ~int64](v int64) bool {
	var z T
	switch any(z).(type) {
	case int:
		return v > math.MaxInt || v < math.MinInt
	case int8:
		return v > math.MaxInt8 || v < math.MinInt8
	case int16:
		return v > math.MaxInt16 || v < math.MinInt16
	case int32:
		return v > math.MaxInt32 || v < math.MinInt32
	case int64:
	}
	return false
}

// OverflowUint checks whether a uint64 value exceeds the bounds of the target unsigned type T.
func OverflowUint[T ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64](v uint64) bool {
	var z T
	switch any(z).(type) {
	case uint:
		return v > math.MaxUint
	case uint8:
		return v > math.MaxUint8
	case uint16:
		return v > math.MaxUint16
	case uint32:
		return v > math.MaxUint32
	}
	return false
}

// OverflowFloat checks whether a float64 value exceeds the bounds of the target float type T.
func OverflowFloat[T ~float32 | ~float64](v float64) bool {
	var z T
	switch any(z).(type) {
	case float32:
		return v > math.MaxFloat32 || v < -math.MaxFloat32
	}
	return false
}
