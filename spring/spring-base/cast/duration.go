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

const (
	Nanosecond  = "ns" // 纳秒
	Microsecond = "μs" // 微秒
	Millisecond = "ms" // 毫秒
	Second      = "s"  // 秒
	Minute      = "m"  // 分
	Hour        = "h"  // 小时
)

var unitMap = map[string]int64{
	"ns": int64(time.Nanosecond),
	"μs": int64(time.Microsecond),
	"ms": int64(time.Millisecond),
	"s":  int64(time.Second),
	"m":  int64(time.Minute),
	"h":  int64(time.Hour),
}

// ToDuration casts an interface{} to a time.Duration.
func ToDuration(i interface{}, unit ...string) time.Duration {
	v, _ := ToDurationE(i, unit...)
	return v
}

// ToDurationE casts an interface{} to a time.Duration.
func ToDurationE(i interface{}, unit ...string) (time.Duration, error) {
	switch s := i.(type) {
	case nil:
		return 0, nil
	case int:
		return parseIntDuration(int64(s), unit...), nil
	case int8:
		return parseIntDuration(int64(s), unit...), nil
	case int16:
		return parseIntDuration(int64(s), unit...), nil
	case int32:
		return parseIntDuration(int64(s), unit...), nil
	case int64:
		return parseIntDuration(s, unit...), nil
	case *int:
		return parseIntDuration(int64(*s), unit...), nil
	case *int8:
		return parseIntDuration(int64(*s), unit...), nil
	case *int16:
		return parseIntDuration(int64(*s), unit...), nil
	case *int32:
		return parseIntDuration(int64(*s), unit...), nil
	case *int64:
		return parseIntDuration(*s, unit...), nil
	case uint:
		return parseIntDuration(int64(s), unit...), nil
	case uint8:
		return parseIntDuration(int64(s), unit...), nil
	case uint16:
		return parseIntDuration(int64(s), unit...), nil
	case uint32:
		return parseIntDuration(int64(s), unit...), nil
	case uint64:
		return parseIntDuration(int64(s), unit...), nil
	case *uint:
		return parseIntDuration(int64(*s), unit...), nil
	case *uint8:
		return parseIntDuration(int64(*s), unit...), nil
	case *uint16:
		return parseIntDuration(int64(*s), unit...), nil
	case *uint32:
		return parseIntDuration(int64(*s), unit...), nil
	case *uint64:
		return parseIntDuration(int64(*s), unit...), nil
	case float32:
		return parseFloatDuration(float64(s), unit...), nil
	case float64:
		return parseFloatDuration(s, unit...), nil
	case *float32:
		return parseFloatDuration(float64(*s), unit...), nil
	case *float64:
		return parseFloatDuration(*s, unit...), nil
	case string:
		return time.ParseDuration(s)
	case *string:
		return time.ParseDuration(*s)
	case time.Duration:
		return s, nil
	default:
		return 0, fmt.Errorf("unable to cast %#v of type %T to time.Duration", i, i)
	}
}

func parseIntDuration(v int64, unit ...string) time.Duration {
	unitN := int64(time.Nanosecond)
	if len(unit) > 0 {
		unitN, _ = unitMap[unit[0]]
	}
	return time.Duration(v * unitN)
}

func parseFloatDuration(v float64, unit ...string) time.Duration {
	unitN := int64(time.Nanosecond)
	if len(unit) > 0 {
		unitN, _ = unitMap[unit[0]]
	}
	return time.Duration(v * float64(unitN))
}
