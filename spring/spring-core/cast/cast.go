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
	"time"

	"github.com/spf13/cast"
)

// ToBool casts an interface to a bool type.
func ToBool(i interface{}) bool {
	v, _ := ToBoolE(i)
	return v
}

// ToBoolE casts an interface to a bool type.
func ToBoolE(i interface{}) (bool, error) {
	return cast.ToBoolE(i)
}

// ToInt casts an interface to an int type.
func ToInt(i interface{}) int {
	v, _ := ToInt64E(i)
	return int(v)
}

// ToInt64 casts an interface to an int64 type.
func ToInt64(i interface{}) int64 {
	v, _ := ToInt64E(i)
	return v
}

// ToInt64E casts an interface to an int64 type.
func ToInt64E(i interface{}) (int64, error) {
	return cast.ToInt64E(i)
}

// ToUint64 casts an interface to a uint64 type.
func ToUint64(i interface{}) uint64 {
	v, _ := ToUint64E(i)
	return v
}

// ToUint64E casts an interface to a uint64 type.
func ToUint64E(i interface{}) (uint64, error) {
	return cast.ToUint64E(i)
}

// ToFloat64 casts an interface to a float64 type.
func ToFloat64(i interface{}) float64 {
	v, _ := ToFloat64E(i)
	return v
}

// ToFloat64E casts an interface to a float64 type.
func ToFloat64E(i interface{}) (float64, error) {
	return cast.ToFloat64E(i)
}

// ToString casts an interface to a string type.
func ToString(i interface{}) string {
	v, err := ToStringE(i)
	if err != nil {
		return err.Error()
	}
	return v
}

// ToStringE casts an interface to a string type.
func ToStringE(i interface{}) (string, error) {
	return cast.ToStringE(i)
}

// ToDuration casts an interface to a time.Duration type.
func ToDuration(i interface{}) time.Duration {
	v, _ := ToDurationE(i)
	return v
}

// ToDurationE casts an interface to a time.Duration type.
func ToDurationE(i interface{}) (d time.Duration, err error) {
	return cast.ToDurationE(i)
}

// ToTime casts an interface to a time.Time type.
func ToTime(i interface{}) time.Time {
	v, _ := ToTimeE(i)
	return v
}

// ToTimeE casts an interface to a time.Time type.
func ToTimeE(i interface{}) (tim time.Time, err error) {
	return cast.ToTimeE(i)
}
