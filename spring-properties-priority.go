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
	"io"
	"time"

	"github.com/go-spring/spring-const"
	"github.com/spf13/cast"
)

// priorityProperties 基于优先级的 Properties 版本
type priorityProperties struct {
	curr Properties // 高优先级
	next Properties // 低优先级
}

// NewPriorityProperties priorityProperties 的构造函数
func NewPriorityProperties(curr Properties, next Properties) *priorityProperties {
	return &priorityProperties{curr: curr, next: next}
}

// LoadProperties 加载属性配置文件，支持 properties、yaml 和 toml 三种文件格式。
func (p *priorityProperties) LoadProperties(filename string) {
	p.curr.LoadProperties(filename)
}

// ReadProperties 读取属性配置文件，支持 properties、yaml 和 toml 三种文件格式。
func (p *priorityProperties) ReadProperties(reader io.Reader, configType string) {
	p.curr.ReadProperties(reader, configType)
}

// GetProperty 返回 keys 中第一个存在的属性值，属性名称统一转成小写。
func (p *priorityProperties) GetProperty(keys ...string) interface{} {
	if v := p.curr.GetProperty(keys...); v == nil {
		return p.next.GetProperty(keys...)
	} else {
		return v
	}
}

// GetBoolProperty 返回 keys 中第一个存在的布尔型属性值，属性名称统一转成小写。
func (p *priorityProperties) GetBoolProperty(keys ...string) bool {
	return cast.ToBool(p.GetProperty(keys...))
}

// GetIntProperty 返回 keys 中第一个存在的有符号整型属性值，属性名称统一转成小写。
func (p *priorityProperties) GetIntProperty(keys ...string) int64 {
	return cast.ToInt64(p.GetProperty(keys...))
}

// GetUintProperty 返回 keys 中第一个存在的无符号整型属性值，属性名称统一转成小写。
func (p *priorityProperties) GetUintProperty(keys ...string) uint64 {
	return cast.ToUint64(p.GetProperty(keys...))
}

// GetFloatProperty 返回 keys 中第一个存在的浮点型属性值，属性名称统一转成小写。
func (p *priorityProperties) GetFloatProperty(keys ...string) float64 {
	return cast.ToFloat64(p.GetProperty(keys...))
}

// GetStringProperty 返回 keys 中第一个存在的字符串型属性值，属性名称统一转成小写。
func (p *priorityProperties) GetStringProperty(keys ...string) string {
	return cast.ToString(p.GetProperty(keys...))
}

// GetDurationProperty 返回 keys 中第一个存在的 Duration 类型属性值，属性名称统一转成小写。
func (p *priorityProperties) GetDurationProperty(keys ...string) time.Duration {
	return cast.ToDuration(p.GetProperty(keys...))
}

// GetTimeProperty 返回 keys 中第一个存在的 Time 类型的属性值，属性名称统一转成小写。
func (p *priorityProperties) GetTimeProperty(keys ...string) time.Time {
	return cast.ToTime(p.GetProperty(keys...))
}

// SetProperty 设置属性值，属性名称统一转成小写。
func (p *priorityProperties) SetProperty(key string, value interface{}) {
	p.curr.SetProperty(key, value)
}

// GetDefaultProperty 返回属性值，如果没有找到则使用指定的默认值，属性名称统一转成小写。
func (p *priorityProperties) GetDefaultProperty(key string, def interface{}) (interface{}, bool) {
	if v, ok := p.curr.GetDefaultProperty(key, def); !ok {
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

// GetProperties 返回所有的属性值，属性名称统一转成小写。
func (p *priorityProperties) GetProperties() map[string]interface{} {
	properties := p.curr.GetProperties()
	for key, val := range p.next.GetProperties() {
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

// BindPropertyIf 根据类型获取属性值，属性名称统一转成小写。
func (p *priorityProperties) BindPropertyIf(key string, i interface{}, allAccess bool) {
	panic(SpringConst.UnimplementedMethod)
}

// InsertBefore 在 next 之前增加一层属性值列表
func (p *priorityProperties) InsertBefore(curr Properties, next Properties) bool {

	// 如果插在最前面
	if p.curr == next {
		nxt := NewPriorityProperties(p.curr, p.next)
		p.curr = curr
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
