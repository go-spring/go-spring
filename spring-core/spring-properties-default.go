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
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cast"
	"github.com/spf13/viper"
)

//
// 定义 Properties 的默认版本
//
type DefaultProperties struct {
	Properties map[string]interface{}
}

//
// 工厂函数
//
func NewDefaultProperties() *DefaultProperties {
	return &DefaultProperties{
		Properties: make(map[string]interface{}),
	}
}

//
// 加载属性配置文件
//
func (p *DefaultProperties) LoadProperties(filename string) {
	fmt.Println("load properties from", filename)

	v := viper.New()
	v.SetConfigFile(filename)
	v.ReadInConfig()

	keys := v.AllKeys()
	sort.Strings(keys)

	for _, k := range keys {
		v := v.Get(k)
		p.SetProperty(k, v)
		fmt.Printf("%s=%v\n", k, v)
	}
}

//
// 获取属性值，属性名称不支持大小写。
//
func (p *DefaultProperties) GetProperty(name string) interface{} {
	return p.Properties[name]
}

//
// 获取布尔型属性值，属性名称不支持大小写。
//
func (p *DefaultProperties) GetBoolProperty(name string) bool {
	return cast.ToBool(p.GetProperty(name))
}

//
// 获取有符号整型属性值，属性名称不支持大小写。
//
func (p *DefaultProperties) GetIntProperty(name string) int64 {
	return cast.ToInt64(p.GetProperty(name))
}

//
// 获取无符号整型属性值，属性名称不支持大小写。
//
func (p *DefaultProperties) GetUintProperty(name string) uint64 {
	return cast.ToUint64(p.GetProperty(name))
}

//
// 获取浮点型属性值，属性名称不支持大小写。
//
func (p *DefaultProperties) GetFloatProperty(name string) float64 {
	return cast.ToFloat64(p.GetProperty(name))
}

//
// 获取字符串型属性值，属性名称不支持大小写。
//
func (p *DefaultProperties) GetStringProperty(name string) string {
	return cast.ToString(p.GetProperty(name))
}

//
// 获取字符串数组属性值，属性名称不支持大小写。
//
func (p *DefaultProperties) GetStringSliceProperty(name string) []string {
	m := p.GetPrefixProperties(name)
	if v, ok := m[name]; ok {
		return cast.ToStringSlice(v)
	} else {
		panic(fmt.Sprintf("property %s not found or use yaml", name))
	}
}

//
// 获取哈希表数组属性值，属性名称不支持大小写。
//
func (p *DefaultProperties) GetMapSliceProperty(name string) []map[string]interface{} {

	m := p.GetPrefixProperties(name)
	if v, ok := m[name]; ok {

		if s, ok := v.([]interface{}); ok {
			var result []map[string]interface{}

			for _, si := range s {
				if sv, ok := si.(map[interface{}]interface{}); ok {
					result = append(result, cast.ToStringMap(sv))
				} else {
					panic(fmt.Sprintf("property %s isn't []map[string]interface{}", name))
				}
			}

			return result
		} else {
			panic(fmt.Sprintf("property %s isn't []map[string]interface{}", name))
		}
	} else {
		panic(fmt.Sprintf("property %s not found or use yaml", name))
	}
}

//
// 设置属性值，属性名称不支持大小写。
//
func (p *DefaultProperties) SetProperty(name string, value interface{}) {
	p.Properties[name] = value
}

//
// 获取属性值，如果没有找到则使用指定的默认值
//
func (p *DefaultProperties) GetDefaultProperty(name string, defaultValue interface{}) (interface{}, bool) {
	if v, ok := p.Properties[name]; ok {
		return v, true
	}
	return defaultValue, false
}

//
// 获取指定前缀的属性值集合
//
func (p *DefaultProperties) GetPrefixProperties(prefix string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range p.Properties {
		if strings.HasPrefix(k, prefix) {
			result[k] = v
		}
	}
	return result
}

//
// 获取所有的属性值
//
func (p *DefaultProperties) GetAllProperties() map[string]interface{} {
	return p.Properties
}
