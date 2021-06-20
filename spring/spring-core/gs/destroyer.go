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

package gs

import (
	"container/list"
	"reflect"
)

// destroyer 保存具有销毁函数的 Bean 以及销毁函数的调用顺序。
type destroyer struct {
	current *BeanDefinition
	earlier []*BeanDefinition
}

// after 添加一个需要在该 Bean 之前调用销毁函数的 Bean。
func (d *destroyer) after(b *BeanDefinition) {
	if d.foundEarlier(b) {
		return
	}
	d.earlier = append(d.earlier, b)
}

func (d *destroyer) foundEarlier(b *BeanDefinition) bool {
	for _, f := range d.earlier {
		if f == b {
			return true
		}
	}
	return false
}

// getBeforeDestroyers 获取排在 i 前面的 destroyer，用于 sort.Triple 排序。
func getBeforeDestroyers(destroyers *list.List, i interface{}) *list.List {
	d := i.(*destroyer)
	result := list.New()
	for e := destroyers.Front(); e != nil; e = e.Next() {
		c := e.Value.(*destroyer)
		if d.foundEarlier(c.current) {
			result.PushBack(c)
		}
	}
	return result
}

type destroyer0 struct {
	fn interface{}
	v  reflect.Value
}
