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

package util

import (
	"reflect"
	"sort"

	"github.com/go-spring/spring-base/cast"
)

// SortedKeys returns the keys of the map m in sorted order.
func SortedKeys(i interface{}) []string {
	keys := Keys(i)
	if len(keys) == 0 {
		return nil
	}
	sort.Strings(keys)
	return keys
}

// Keys returns the keys of the map m in indeterminate order.
func Keys(i interface{}) []string {
	switch m := i.(type) {
	case map[string]interface{}:
		if len(m) == 0 {
			return nil
		}
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		return keys
	case map[string]string:
		if len(m) == 0 {
			return nil
		}
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		return keys
	default:
		v := reflect.ValueOf(i)
		if v.Kind() != reflect.Map {
			panic("should be a map")
		}
		if v.Len() == 0 {
			return nil
		}
		keys := make([]string, 0, v.Len())
		for _, k := range v.MapKeys() {
			keys = append(keys, cast.ToString(k.Interface()))
		}
		return keys
	}
}
