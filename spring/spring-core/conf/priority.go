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

import (
	"github.com/go-spring/spring-core/util"
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
func (p *priorityProperties) Bind(key string, i interface{}) error {
	panic(util.UnimplementedMethod)
}

// Get 返回属性值，不能存在返回 nil，属性名称统一转成小写。
func (p *priorityProperties) Get(key string) interface{} {
	if v := p.Properties.Get(key); v == nil {
		return p.next.Get(key)
	} else {
		return v
	}
}

// GetFirst 返回 keys 中第一个存在的属性值，属性名称统一转成小写。
func (p *priorityProperties) GetFirst(keys ...string) interface{} {
	if v := p.Properties.GetFirst(keys...); v == nil {
		return p.next.GetFirst(keys...)
	} else {
		return v
	}
}

// GetDefault 返回属性值，如果没有找到则使用指定的默认值，属性名称统一转成小写。
func (p *priorityProperties) GetDefault(key string, def interface{}) interface{} {
	if v := p.Get(key); v == nil {
		return def
	} else {
		return v
	}
}

// Keys 返回所有键，属性名称统一转成小写。
func (p *priorityProperties) Keys() []string {
	panic(util.UnimplementedMethod)
}

// Range 遍历所有的属性值，属性名称统一转成小写。
func (p *priorityProperties) Range(fn func(string, interface{})) {
	p.Properties.Range(fn)
	p.next.Range(fn)
}

// Fill Fill 填充所有的属性值，属性名称统一转成小写。TODO 实现并不完美。
func (p *priorityProperties) Fill(properties map[string]interface{}) {
	p.Properties.Range(func(key string, val interface{}) {
		if _, ok := properties[key]; !ok {
			properties[key] = val
		}
	})
	p.next.Range(func(key string, val interface{}) {
		if _, ok := properties[key]; !ok {
			properties[key] = val
		}
	})
}

// Prefix 返回指定前缀的属性值集合，属性名称统一转成小写。
func (p *priorityProperties) Prefix(key string) map[string]interface{} {
	panic(util.UnimplementedMethod)
}

// Group 返回指定前缀的属性值集合并进行分组，属性名称统一转成小写。
func (p *priorityProperties) Group(key string) map[string]map[string]interface{} {
	panic(util.UnimplementedMethod)
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
