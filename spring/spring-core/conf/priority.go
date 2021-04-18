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

package conf

// priorityProperties 基于优先级的 Properties 实现。
type priorityProperties struct {
	Properties // 高优先级

	next Properties // 低优先级
}

// Priority priorityProperties 的构造函数
func Priority(curr Properties, next Properties) *priorityProperties {
	return &priorityProperties{Properties: curr, next: next}
}

// Depth 返回深度值
func (p *priorityProperties) Depth() int {
	if nxt, ok := p.next.(*priorityProperties); ok {
		return nxt.Depth() + 1
	} else {
		return 2
	}
}

// Get 返回 key 转为小写后精确匹配的属性值，不存在返回 nil。
func (p *priorityProperties) Get(key string) interface{} {
	if v := p.Properties.Get(key); v == nil {
		return p.next.Get(key)
	} else {
		return v
	}
}

// InsertBefore 在 next 之前增加一层属性列表
func (p *priorityProperties) InsertBefore(curr Properties, next Properties) bool {

	// 如果插在最前面
	if p.Properties == next {
		nxt := Priority(p.Properties, p.next)
		p.Properties = curr
		p.next = nxt
		return true
	}

	// 如果插在中间
	if p.next == next {
		nxt := Priority(curr, p.next)
		p.next = nxt
		return true
	}

	// 否则只能插在尾部
	if nxt, ok := p.next.(*priorityProperties); ok {
		return nxt.InsertBefore(curr, next)
	}

	// 找不到插入点
	return false
}
