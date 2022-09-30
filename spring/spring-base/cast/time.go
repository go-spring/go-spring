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

package cast

import (
	"fmt"
	"time"
)

var unitMap = map[string]int64{
	"ns": int64(time.Nanosecond),
	"μs": int64(time.Microsecond),
	"ms": int64(time.Millisecond),
	"s":  int64(time.Second),
	"m":  int64(time.Minute),
	"h":  int64(time.Hour),
}

// ToTime casts an interface{} to a time.Time.
// When type is clear, it is recommended to use standard library functions.
func ToTime(i interface{}, format ...string) time.Time {
	v, _ := ToTimeE(i, format...)
	return v
}

// ToTimeE casts an interface{} to a time.Time.
// When type is clear, it is recommended to use standard library functions.
func ToTimeE(i interface{}, format ...string) (time.Time, error) {
	switch v := i.(type) {
	case nil:
		return time.Time{}, nil
	case int:
		return parseIntTimestamp(int64(v), format...), nil
	case int8:
		return parseIntTimestamp(int64(v), format...), nil
	case int16:
		return parseIntTimestamp(int64(v), format...), nil
	case int32:
		return parseIntTimestamp(int64(v), format...), nil
	case int64:
		return parseIntTimestamp(v, format...), nil
	case *int:
		return parseIntTimestamp(int64(*v), format...), nil
	case *int8:
		return parseIntTimestamp(int64(*v), format...), nil
	case *int16:
		return parseIntTimestamp(int64(*v), format...), nil
	case *int32:
		return parseIntTimestamp(int64(*v), format...), nil
	case *int64:
		return parseIntTimestamp(*v, format...), nil
	case uint:
		return parseIntTimestamp(int64(v), format...), nil
	case uint8:
		return parseIntTimestamp(int64(v), format...), nil
	case uint16:
		return parseIntTimestamp(int64(v), format...), nil
	case uint32:
		return parseIntTimestamp(int64(v), format...), nil
	case uint64:
		return parseIntTimestamp(int64(v), format...), nil
	case *uint:
		return parseIntTimestamp(int64(*v), format...), nil
	case *uint8:
		return parseIntTimestamp(int64(*v), format...), nil
	case *uint16:
		return parseIntTimestamp(int64(*v), format...), nil
	case *uint32:
		return parseIntTimestamp(int64(*v), format...), nil
	case *uint64:
		return parseIntTimestamp(int64(*v), format...), nil
	case float32:
		return parseFloatTimestamp(float64(v), format...), nil
	case float64:
		return parseFloatTimestamp(v, format...), nil
	case *float32:
		return parseFloatTimestamp(float64(*v), format...), nil
	case *float64:
		return parseFloatTimestamp(*v, format...), nil
	case string:
		return parseFormatTime(v, format...)
	case *string:
		return parseFormatTime(*v, format...)
	case time.Time:
		return v, nil
	case *time.Time:
		return *v, nil
	default:
		return time.Time{}, fmt.Errorf("unable to cast type %T to Time", i)
	}
}

func parseFormatTime(v string, format ...string) (time.Time, error) {
	if d, err := time.ParseDuration(v); err == nil {
		return time.Unix(int64(d/time.Second), int64(d%time.Second)), nil
	}
	layout := "2006-01-02 15:04:05 -0700"
	if len(format) > 0 {
		layout = format[0]
	}
	return time.Parse(layout, v)
}

func parseIntTimestamp(v int64, format ...string) time.Time {
	unitN := int64(time.Nanosecond)
	if len(format) > 0 {
		unitN, _ = unitMap[format[0]]
	}
	v = v * unitN
	return time.Unix(v/int64(time.Second), v%int64(time.Second))
}

func parseFloatTimestamp(v float64, format ...string) time.Time {
	unitN := int64(time.Nanosecond)
	if len(format) > 0 {
		unitN, _ = unitMap[format[0]]
	}
	i := int64(v * float64(unitN))
	return time.Unix(i/int64(time.Second), i%int64(time.Second))
}
