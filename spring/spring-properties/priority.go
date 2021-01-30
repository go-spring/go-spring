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

package SpringProperties

import (
	"github.com/go-spring/spring-const"
)

// priorityProperties 基于优先级的 Properties 版本
type priorityProperties struct {
	Properties // 高优先级

	next Properties // 低优先级
}

// Priority priorityProperties 的构造函数
func Priority(curr Properties, next Properties) *priorityProperties {
	return &priorityProperties{Properties: curr, next: next}
}

// Has 查询属性值是否存在，属性名称统一转成小写。
func (p *priorityProperties) Has(key string) bool {
	return p.Properties.Has(key) || p.next.Has(key)
}

// Bind 根据类型获取属性值，属性名称统一转成小写。
func (p *priorityProperties) Bind(key string, i interface{}) {
	panic(SpringConst.UnimplementedMethod)
}

// Get 返回属性值，没有找到则返回默认值，属性名称统一转成小写。
func (p *priorityProperties) Get(key string, def ...interface{}) interface{} {
	if v := p.Properties.Get(key, def...); v == nil {
		return p.next.Get(key, def...)
	} else {
		return v
	}
}

// Map 返回所有的属性值，属性名称统一转成小写。
func (p *priorityProperties) Map() map[string]interface{} {
	properties := map[string]interface{}{}
	for key, val := range p.Properties.Map() {
		if _, ok := properties[key]; !ok {
			properties[key] = val
		}
	}
	for key, val := range p.next.Map() {
		if _, ok := properties[key]; !ok {
			properties[key] = val
		}
	}
	return properties
}

// Prefix 返回指定前缀的属性值集合，属性名称统一转成小写。
func (p *priorityProperties) Prefix(key string) map[string]interface{} {
	panic(SpringConst.UnimplementedMethod)
}

// Group 返回指定前缀的属性值集合并进行分组，属性名称统一转成小写。
func (p *priorityProperties) Group(key string) map[string]map[string]interface{} {
	panic(SpringConst.UnimplementedMethod)
}

// InsertBefore 在 next 之前增加一层属性值列表
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

// Depth 返回深度值
func (p *priorityProperties) Depth() int {
	if nxt, ok := p.next.(*priorityProperties); ok {
		return nxt.Depth() + 1
	} else {
		return 2
	}
}
