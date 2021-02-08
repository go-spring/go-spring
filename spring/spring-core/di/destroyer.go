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

package di

import (
	"container/list"

	"github.com/go-spring/spring-core/bean"
)

// destroyer 保存具有销毁函数的 Bean 以及销毁函数的调用顺序
type destroyer struct {
	bean  *bean.BeanDefinition
	after []*bean.BeanDefinition
}

// After 添加一个在此之前调用的销毁函数
func (d *destroyer) After(b *bean.BeanDefinition) *destroyer {

	for _, f := range d.after {
		if f == b {
			return d
		}
	}

	d.after = append(d.after, b)
	return d
}

// getBeforeDestroyers 获取排在当前 destroyer 前面的 destroyer 项
func getBeforeDestroyers(destroyers *list.List, i interface{}) *list.List {
	result := list.New()
	current := i.(*destroyer)
	for e := destroyers.Front(); e != nil; e = e.Next() {
		c := e.Value.(*destroyer)
		// 检查是否在当前 destroyer 的前面
		for _, b := range current.after {
			if c.bean == b {
				result.PushBack(c)
			}
		}
	}
	return result
}
