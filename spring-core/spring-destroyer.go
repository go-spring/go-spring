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

// getBeforeDestroyers 获取当前 Destroyer 依赖的 Destroyer 列表
func getBeforeDestroyers(destroyers *list.List, i interface{}) *list.List {
	result := list.New()
	current := i.(*Destroyer)
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
