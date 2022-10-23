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

package util_test

import (
	"sort"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/util"
)

func TestSortedKeys(t *testing.T) {
	assert.Panic(t, func() {
		util.SortedKeys(3)
	}, "should be a map")
	keys := util.SortedKeys(map[string]interface{}{})
	assert.Nil(t, keys)
	keys = util.SortedKeys(map[string]interface{}{"1": 1, "2": 2, "3": 3, "4": 4})
	assert.Equal(t, keys, []string{"1", "2", "3", "4"})
	keys = util.SortedKeys(map[string]string{})
	assert.Nil(t, keys)
	keys = util.SortedKeys(map[string]string{"1": "a", "2": "b", "3": "c", "4": "d"})
	assert.Equal(t, keys, []string{"1", "2", "3", "4"})
	keys = util.SortedKeys(map[int]string{})
	assert.Nil(t, keys)
	keys = util.SortedKeys(map[int]string{1: "a", 2: "b", 3: "c", 4: "d"})
	assert.Equal(t, keys, []string{"1", "2", "3", "4"})
}

func TestKeys(t *testing.T) {
	assert.Panic(t, func() {
		util.Keys(3)
	}, "should be a map")
	keys := util.Keys(map[string]interface{}{})
	assert.Nil(t, keys)
	keys = util.Keys(map[string]interface{}{"1": 1, "2": 2, "3": 3, "4": 4})
	sort.Strings(keys)
	assert.Equal(t, keys, []string{"1", "2", "3", "4"})
	keys = util.Keys(map[string]string{})
	assert.Nil(t, keys)
	keys = util.Keys(map[string]string{"1": "a", "2": "b", "3": "c", "4": "d"})
	sort.Strings(keys)
	assert.Equal(t, keys, []string{"1", "2", "3", "4"})
	keys = util.Keys(map[int]string{})
	assert.Nil(t, keys)
	keys = util.Keys(map[int]string{1: "a", 2: "b", 3: "c", 4: "d"})
	sort.Strings(keys)
	assert.Equal(t, keys, []string{"1", "2", "3", "4"})
}
