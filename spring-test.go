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

package SpringTest

import (
	"reflect"
	"testing"

	"github.com/go-spring/spring-utils"
)

// Diff 比较任意两个变量的内容是否相同
func Diff(t *testing.T, v1 interface{}, v2 interface{}) {

	m1, ok1 := v1.(map[string]interface{})
	m2, ok2 := v2.(map[string]interface{})
	if ok1 || ok2 {
		DiffMap(t, m1, m2)
		return
	}

	ma1, ok1 := v1.([]map[string]interface{})
	ma2, ok2 := v2.([]map[string]interface{})
	if ok1 || ok2 {
		DiffMapArray(t, ma1, ma2)
		return
	}

	a1, ok1 := v1.([]interface{})
	a2, ok2 := v2.([]interface{})
	if ok1 || ok2 {
		DiffArray(t, a1, a2)
		return
	}

	s1, ok1 := SpringUtils.DefaultString(v1)
	s2, ok2 := SpringUtils.DefaultString(v2)
	if (ok1 && ok2) && (s1 == s2) {
		return
	}

	b1, ok1 := SpringUtils.DefaultBool(v1)
	b2, ok2 := SpringUtils.DefaultBool(v2)
	if (ok1 && ok2) && (b1 == b2) {
		return
	}

	if !reflect.DeepEqual(v1, v2) {
		t.Errorf("%v -> %v\n", v1, v2)
	}
}

// DiffMap 比较两个 map 的内容是否相同
func DiffMap(t *testing.T, v1 map[string]interface{}, v2 map[string]interface{}) {
	if len(v1) > len(v2) {
		for k := range v1 {
			Diff(t, v1[k], v2[k])
		}
	} else {
		for k := range v2 {
			Diff(t, v1[k], v2[k])
		}
	}
}

// DiffMapArray 比较两个 map 数组的内容是否相同，顺序也必须相同
func DiffMapArray(t *testing.T, v1 []map[string]interface{}, v2 []map[string]interface{}) {
	if len(v1) != len(v2) {
		t.Errorf("%v -> %v\n", v1, v2)
	}
	for i := range v1 {
		DiffMap(t, v1[i], v2[i])
	}
}

// DiffArray 比较两个数组的内容是否相同，顺序也必须相同
func DiffArray(t *testing.T, v1 []interface{}, v2 []interface{}) {
	if len(v1) != len(v2) {
		t.Errorf("%v -> %v\n", v1, v2)
	}
	for i := range v1 {
		Diff(t, v1[i], v2[i])
	}
}
