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
	"container/list"
)

// ContainsInt 在一个 int 数组中进行查找，找不到返回 -1。
func ContainsInt(array []int, val int) int {
	for i := 0; i < len(array); i++ {
		if array[i] == val {
			return i
		}
	}
	return -1
}

// ContainsString 在一个 string 数组中进行查找，找不到返回 -1。
func ContainsString(array []string, val string) int {
	for i := 0; i < len(array); i++ {
		if array[i] == val {
			return i
		}
	}
	return -1
}

// ContainsList 在列表中查询指定元素，存在则返回列表项指针，不存在返回 nil。
func ContainsList(l *list.List, v interface{}) *list.Element {
	for e := l.Front(); e != nil; e = e.Next() {
		if e.Value == v {
			return e
		}
	}
	return nil
}
