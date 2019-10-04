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
	"errors"
	"fmt"
	"reflect"
	"strings"
)

//
// 定义 SpringBean 类型
//
type SpringBean interface{}

//
// SpringBean 初始化接口
//
type SpringBeanInitialization interface {
	InitBean(ctx SpringContext)
}

//
// 定义 SpringBeanDefinition 类型
//
type SpringBeanDefinition struct {
	Bean  SpringBean
	Name  string
	Init  int
	Type  reflect.Type
	Value reflect.Value
}

//
// 定义 SpringContext 接口
//
type SpringContext interface {
	// 注册 SpringBean 使用默认的 Bean 名称
	RegisterBean(bean SpringBean)

	// 注册 SpringBean 使用指定的 Bean 名称
	RegisterNameBean(name string, bean SpringBean)

	// 根据 Bean 类型查找 SpringBean 数组
	FindBeansByType(i interface{})

	// 根据 Bean 类型查找 SpringBeanDefinition 数组
	FindBeanDefinitionsByType(t reflect.Type) []*SpringBeanDefinition

	// 根据 Bean 名称查找 SpringBean
	FindBeanByName(name string) SpringBean

	// 根据 Bean 名称查找 SpringBeanDefinition
	FindBeanDefinitionByName(name string) *SpringBeanDefinition

	// 获取属性值
	GetProperties(name string) interface{}

	// 设置属性值
	SetProperties(name string, value interface{})

	// 获取指定前缀的属性值集合
	GetPrefixProperties(prefix string) map[string]interface{}

	// 获取属性值，如果没有找到则使用指定的默认值
	GetDefaultProperties(name string, defaultValue interface{}) (interface{}, bool)

	// 自动绑定所有的 SpringBean
	AutoWireBeans() error

	// 绑定外部指定的 SpringBean
	WireBean(bean SpringBean) error
}

//
// SpringContext 的默认版本
//
type DefaultSpringContext struct {
	beanDefinitionMap map[string]*SpringBeanDefinition // Bean 集合
	propertiesMap     map[string]interface{}           // 属性值集合
}

//
// 工厂函数
//
func NewDefaultSpringContext() *DefaultSpringContext {
	return &DefaultSpringContext{
		beanDefinitionMap: make(map[string]*SpringBeanDefinition),
		propertiesMap:     make(map[string]interface{}),
	}
}

//
// SpringBean 的初始化状态值
//
const (
	Uninitialized = iota
	Initializing
	Initialized
)

//
// 注册 SpringBean 的定义
//
func (ctx *DefaultSpringContext) registerBeanDefinition(name string, bean SpringBean, t reflect.Type, v reflect.Value) {
	ctx.beanDefinitionMap[name] = &SpringBeanDefinition{
		Init:  Uninitialized,
		Name:  name,
		Bean:  bean,
		Type:  t,
		Value: v,
	}
}

//
// 检查 SpringBean 的类型
//
func checkSpringBeanType(bean SpringBean) (t reflect.Type, v reflect.Value) {
	t = reflect.TypeOf(bean)
	if t.Kind() != reflect.Ptr {
		panic("bean must be pointer")
	}
	v = reflect.ValueOf(bean)
	return
}

//
// 注册 SpringBean 使用默认的 Bean 名称
//
func (ctx *DefaultSpringContext) RegisterBean(bean SpringBean) {
	t, v := checkSpringBeanType(bean)
	ctx.registerBeanDefinition(t.Elem().Name(), bean, t, v)
}

//
// 注册 SpringBean 使用指定的 Bean 名称
//
func (ctx *DefaultSpringContext) RegisterNameBean(name string, bean SpringBean) {
	t, v := checkSpringBeanType(bean)
	ctx.registerBeanDefinition(name, bean, t, v)
}

//
// 根据 Bean 名称查找 SpringBean
//
func (ctx *DefaultSpringContext) FindBeanByName(name string) SpringBean {
	if beanDefinition, ok := ctx.beanDefinitionMap[name]; ok {
		return beanDefinition.Bean
	}
	return nil
}

//
// 根据 Bean 名称查找 SpringBeanDefinition
//
func (ctx *DefaultSpringContext) FindBeanDefinitionByName(name string) *SpringBeanDefinition {
	return ctx.beanDefinitionMap[name]
}

//
// 根据 Bean 类型查找 SpringBean 数组
//
func (ctx *DefaultSpringContext) FindBeansByType(i interface{}) {

	it := reflect.TypeOf(i)
	et := it.Elem()

	if it.Kind() != reflect.Ptr {
		panic("bean must be pointer")
	}

	v := reflect.New(et).Elem()
	t0 := et.Elem()

	for _, beanDefinition := range ctx.beanDefinitionMap {
		if beanDefinition.Type.AssignableTo(t0) {
			v = reflect.Append(v, beanDefinition.Value)
		}
	}

	reflect.ValueOf(i).Elem().Set(v)
}

//
// 根据 Bean 类型查找 SpringBeanDefinition 数组
//
func (ctx *DefaultSpringContext) FindBeanDefinitionsByType(t reflect.Type) []*SpringBeanDefinition {
	result := make([]*SpringBeanDefinition, 0)
	for _, beanDefinition := range ctx.beanDefinitionMap {
		if beanDefinition.Type.AssignableTo(t) {
			result = append(result, beanDefinition)
		}
	}
	return result
}

//
// 获取属性值
//
func (ctx *DefaultSpringContext) GetProperties(name string) interface{} {
	return ctx.propertiesMap[name]
}

//
// 设置属性值
//
func (ctx *DefaultSpringContext) SetProperties(name string, value interface{}) {
	ctx.propertiesMap[name] = value
}

//
// 获取指定前缀的属性值集合
//
func (ctx *DefaultSpringContext) GetPrefixProperties(prefix string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range ctx.propertiesMap {
		if strings.HasPrefix(k, prefix) {
			result[k] = v
		}
	}
	return result
}

//
// 获取属性值，如果没有找到则使用指定的默认值
//
func (ctx *DefaultSpringContext) GetDefaultProperties(name string, defaultValue interface{}) (interface{}, bool) {
	if v, ok := ctx.propertiesMap[name]; ok {
		return v, true
	}
	return defaultValue, false
}

//
// 自动绑定所有的 SpringBean
//
func (ctx *DefaultSpringContext) AutoWireBeans() error {
	for _, beanDefinition := range ctx.beanDefinitionMap {
		if err := ctx.wireBeanByDefinition(beanDefinition); err != nil {
			return err
		}
	}
	return nil
}

//
// 绑定外部指定的 SpringBean
//
func (ctx *DefaultSpringContext) WireBean(bean SpringBean) error {
	t, v := checkSpringBeanType(bean)
	b := &SpringBeanDefinition{
		Name:  t.Elem().Name(),
		Init:  Uninitialized,
		Bean:  bean,
		Type:  t,
		Value: v,
	}
	return ctx.wireBeanByDefinition(b)
}

//
// 绑定 SpringBeanDefinition 指定的 SpringBean
//
func (ctx *DefaultSpringContext) wireBeanByDefinition(beanDefinition *SpringBeanDefinition) error {

	// 确保 SpringBean 还未初始化
	if beanDefinition.Init != Uninitialized {
		return nil
	}

	fmt.Println("wire bean " + beanDefinition.Name)
	defer func() {
		fmt.Println("success wire bean " + beanDefinition.Name)
	}()

	beanDefinition.Init = Initializing

	t := beanDefinition.Type.Elem()
	v := beanDefinition.Value.Elem()

	// 遍历 SpringBean 所有的字段
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)

		// 查找依赖绑定的标签
		if beanName, ok := f.Tag.Lookup("autowire"); ok {
			// TODO 数组绑定

			var definition *SpringBeanDefinition

			if len(beanName) > 0 {
				definition = ctx.FindBeanDefinitionByName(beanName)
			} else {
				definitions := ctx.FindBeanDefinitionsByType(f.Type)
				if len(definitions) > 0 {
					definition = definitions[0]
				}
			}

			if definition != nil {
				ctx.wireBeanByDefinition(definition)
				v.Field(i).Set(definition.Value)
			}

			continue
		}

		// 查找属性绑定的标签
		if value, ok := f.Tag.Lookup("value"); ok && len(value) > 0 {
			// TODO 数组绑定

			if strings.HasPrefix(value, "${") {
				str := value[2 : len(value)-1]
				ss := strings.Split(str, ":=")

				var (
					propName  string
					propValue interface{}
				)

				propName = ss[0]
				if len(ss) > 1 {
					propValue = ss[1]
				}

				if prop, ok := ctx.GetDefaultProperties(propName, ""); ok {
					propValue = prop
				} else {
					if len(ss) < 2 {
						return errors.New("properties " + propName + " not config!")
					}
				}

				vf := v.Field(i)

				switch vf.Kind() {
				case reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8, reflect.Uint:
					vf.SetUint(propValue.(uint64))
				case reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8, reflect.Int:
					vf.SetInt(propValue.(int64))
				case reflect.String:
					vf.SetString(propValue.(string))
				case reflect.Bool:
					vf.SetBool(propValue.(bool))
				default:
					return errors.New("unsupported type " + vf.Type().String())
				}
			}

			continue
		}
	}

	// 执行 SpringBean 的初始化接口
	if c, ok := beanDefinition.Bean.(SpringBeanInitialization); ok {
		c.InitBean(ctx)
	}

	beanDefinition.Init = Initialized
	return nil
}
