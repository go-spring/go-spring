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

package sort

import (
	"container/list"
	"errors"

	"github.com/go-spring/go-spring-parent/spring-utils"
)

// GetBeforeItems 获取 sorting 中排在 current 前面的元素
type GetBeforeItems func(sorting *list.List, current interface{}) *list.List

// TripleSorting 三路排序
func TripleSorting(sorting *list.List, fn GetBeforeItems) *list.List {

	toSort := list.New()     // 待排序列表
	sorted := list.New()     // 已排序列表
	processing := list.New() // 正在处理列表

	toSort.PushBackList(sorting)

	for toSort.Len() > 0 { // 递归选出依赖链条最前端的元素
		tripleSortByAfter(sorting, toSort, sorted, processing, nil, fn)
	}
	return sorted
}

// tripleSortByAfter 递归选出依赖链条最前端的元素
func tripleSortByAfter(sorting *list.List, toSort *list.List, sorted *list.List,
	processing *list.List, current interface{}, fn GetBeforeItems) {

	if current == nil {
		current = toSort.Remove(toSort.Front())
	}

	// 将当前元素标记为正在处理
	processing.PushBack(current)

	// 获取排在当前元素前面的列表项，然后依次对它们进行排序
	for e := fn(sorting, current).Front(); e != nil; e = e.Next() {
		c := e.Value

		// 自己不可能是自己前面的元素，除非出现了循环依赖，因此抛出 Panic
		if _, ok := SpringUtils.FindInList(c, processing); ok {
			panic(errors.New("found sorting cycle"))
		}

		_, inSorted := SpringUtils.FindInList(c, sorted)
		_, inToSort := SpringUtils.FindInList(c, toSort)

		if !inSorted && inToSort { // 如果是待排元素则对其进行排序
			tripleSortByAfter(sorting, toSort, sorted, processing, c, fn)
		}
	}

	if e, ok := SpringUtils.FindInList(current, processing); ok {
		processing.Remove(e)
	}

	if e, ok := SpringUtils.FindInList(current, toSort); ok {
		toSort.Remove(e)
	}

	// 将当前元素标记为已完成
	sorted.PushBack(current)
}
