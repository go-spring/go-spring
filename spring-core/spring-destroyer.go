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

package SpringCore

import (
	"container/list"
	"fmt"
	"strings"
)

// Destroyer 包含销毁函数的 BeanDefinition
type Destroyer struct {
	bean  *BeanDefinition
	after []*BeanDefinition
}

func (d *Destroyer) After(b *BeanDefinition) *Destroyer {

	for _, f := range d.after {
		if f == b {
			return d
		}
	}

	d.after = append(d.after, b)
	return d
}

// sortDestroyers 对 Destroyer 列表进行排序
func sortDestroyers(destroyers *list.List) *list.List {

	// 待排列表
	toSort := list.New()
	toSort.PushBackList(destroyers)

	// 已排序列表
	sorted := list.New()

	// 正在处理的列表
	processing := list.New()

	for toSort.Len() > 0 { // 每次循环选出依赖链条最前端的元素
		sortDestroyersByAfter(destroyers, toSort, sorted, processing, nil)
	}
	return sorted
}

// sortDestroyersByAfter 选出依赖链条最前端的元素
func sortDestroyersByAfter(destroyers *list.List, toSort *list.List,
	sorted *list.List, processing *list.List, current *Destroyer) {

	// 选出待排元素
	if current == nil {
		current = (toSort.Remove(toSort.Front())).(*Destroyer)
	}

	processing.PushBack(current)

	// 遍历当前 Destroyer 依赖的 Destroyer 列表
	for e := getBeforeDestroyers(destroyers, current).Front(); e != nil; e = e.Next() {
		c := e.Value.(*Destroyer)

		// 自己不可能是自己前面的元素，除非出现了循环依赖，因此抛出 Panic
		for p := processing.Front(); p != nil; p = p.Next() {
			if pc := p.Value.(*Destroyer); pc == c {
				// 打印循环依赖的路径
				sb := strings.Builder{}
				for t := p; t != nil; t = t.Next() {
					sb.WriteString(t.Value.(*Destroyer).bean.BeanId())
					sb.WriteString(" -> ")
				}
				sb.WriteString(pc.bean.BeanId())
				panic(fmt.Errorf("found cycle destroyer: %s", sb.String()))
			}
		}

		inSorted := false
		for p := sorted.Front(); p != nil; p = p.Next() {
			if pc := p.Value.(*Destroyer); pc == c {
				inSorted = true
				break
			}
		}

		inToSort := false
		for p := toSort.Front(); p != nil; p = p.Next() {
			if pc := p.Value.(*Destroyer); pc == c {
				inToSort = true
				break
			}
		}

		if !inSorted && inToSort { // 递归处理当前 Destroyer 的依赖并进行排序
			sortDestroyersByAfter(destroyers, toSort, sorted, processing, c)
		}
	}

	// 排序完成，从正在排序、待排序列表删除，然后添加到已排序列表
	{
		for p := processing.Front(); p != nil; p = p.Next() {
			if pc := p.Value.(*Destroyer); pc == current {
				processing.Remove(p)
				break
			}
		}

		for p := toSort.Front(); p != nil; p = p.Next() {
			if pc := p.Value.(*Destroyer); pc == current {
				toSort.Remove(p)
				break
			}
		}

		sorted.PushBack(current)
	}
}

// getBeforeDestroyers 获取当前 Destroyer 依赖的 Destroyer 列表
func getBeforeDestroyers(destroyers *list.List, current *Destroyer) *list.List {
	result := list.New()
	for e := destroyers.Front(); e != nil; e = e.Next() {
		c := e.Value.(*Destroyer)
		// 检查是否在当前 Configer 的前面
		for _, bean := range current.after {
			if c.bean == bean {
				result.PushBack(c)
			}
		}
	}
	return result
}
