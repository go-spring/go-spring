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
	"github.com/go-spring/spring-const"
)

// priorityProperties 基于优先级的 Properties 版本
type priorityProperties struct {
	Properties // 高优先级

	next Properties // 低优先级
}

// NewPriorityProperties priorityProperties 的构造函数
func NewPriorityProperties(curr Properties, next Properties) *priorityProperties {
	return &priorityProperties{Properties: curr, next: next}
}

// GetProperty 返回 keys 中第一个存在的属性值，属性名称统一转成小写。
func (p *priorityProperties) GetProperty(keys ...string) interface{} {
	if v := p.Properties.GetProperty(keys...); v == nil {
		return p.next.GetProperty(keys...)
	} else {
		return v
	}
}

// SetProperty 设置属性值，属性名称统一转成小写。
func (p *priorityProperties) SetProperty(key string, value interface{}) {
	p.Properties.SetProperty(key, value)
}

// GetDefaultProperty 返回属性值，如果没有找到则使用指定的默认值，属性名称统一转成小写。
func (p *priorityProperties) GetDefaultProperty(key string, def interface{}) (interface{}, bool) {
	if v, ok := p.Properties.GetDefaultProperty(key, def); !ok {
		return p.next.GetDefaultProperty(key, def)
	} else {
		return v, ok
	}
}

// GetPrefixProperties 返回指定前缀的属性值集合，属性名称统一转成小写。
func (p *priorityProperties) GetPrefixProperties(prefix string) map[string]interface{} {
	panic(SpringConst.UnimplementedMethod)
}

// GetGroupedProperties 返回指定前缀的属性值集合并进行分组，属性名称统一转成小写。
func (p *priorityProperties) GetGroupedProperties(prefix string) map[string]map[string]interface{} {
	panic(SpringConst.UnimplementedMethod)
}

// GetProperties 返回指定前缀的属性值集合，不传值返回全部属性值，属性名称统一转成小写。
func (p *priorityProperties) GetProperties(prefix ...string) map[string]interface{} {
	properties := p.Properties.GetProperties(prefix...)
	for key, val := range p.next.GetProperties(prefix...) {
		if _, ok := properties[key]; !ok {
			properties[key] = val
		}
	}
	return properties
}

// BindProperty 根据类型获取属性值，属性名称统一转成小写。
func (p *priorityProperties) BindProperty(key string, i interface{}) {
	panic(SpringConst.UnimplementedMethod)
}

// InsertBefore 在 next 之前增加一层属性值列表
func (p *priorityProperties) InsertBefore(curr Properties, next Properties) bool {

	// 如果插在最前面
	if p.Properties == next {
		nxt := NewPriorityProperties(p.Properties, p.next)
		p.Properties = curr
		p.next = nxt
		return true
	}

	// 如果插在中间
	if p.next == next {
		nxt := NewPriorityProperties(curr, p.next)
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
