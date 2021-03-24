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

// NewList 使用输入的元素创建列表
func NewList(v ...interface{}) *list.List {
	l := list.New()
	for _, val := range v {
		l.PushBack(val)
	}
	return l
}

// FindInList 在列表中查询指定元素，存在则返回列表项指针，不存在返回 nil
func FindInList(v interface{}, l *list.List) (*list.Element, bool) {
	for e := l.Front(); e != nil; e = e.Next() {
		if e.Value == v {
			return e, true
		}
	}
	return nil, false
}
