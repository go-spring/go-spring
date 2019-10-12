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
	"github.com/spf13/cast"
	"reflect"
	"strings"
)

//
// SpringContext 的默认版本
//
type DefaultSpringContext struct {
	beanDefinitionMap map[string]*SpringBeanDefinition // Bean 集合
	propertiesMap     map[string]interface{}           // 属性值集合

	nextBeanId int // 目前支持两种bean的命名方式：1、匿名注册使用 "bean#id" 作为 bean 的名称 2、采用 RegisterSingletonBean 注册的bean，以 PkgPath + struct name 命名
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

func (ctx *DefaultSpringContext) RegisterSliceBean(bean SpringBean) {
	t := reflect.TypeOf(bean)
	v := reflect.ValueOf(bean)

	name := fmt.Sprintf("bean#%d", ctx.nextBeanId)
	ctx.nextBeanId++

	beanDefinition := &SpringBeanDefinition{
		Init:  Uninitialized,
		Name:  name,
		Bean:  bean,
		Type:  t,
		Value: v,
	}

	ctx.RegisterBeanDefinition(beanDefinition)
}

//
// SpringBean 转换为 SpringBeanDefinition 对象
//
func (ctx *DefaultSpringContext) ToSpringBeanDefinition(name string, bean SpringBean) *SpringBeanDefinition {

	t := checkBeanType(bean)
	v := reflect.ValueOf(bean)

	// 未指定名称的情况，按照默认规则生成名称
	if name == "" {
		name = getBeanUnameByType(t)
	}

	return &SpringBeanDefinition{
		Init:  Uninitialized,
		Name:  name,
		Bean:  bean,
		Type:  t,
		Value: v,
	}
}

//
// 使用默认的名称注册 SpringBean 对象
//
func (ctx *DefaultSpringContext) RegisterBean(bean SpringBean) {

	name := fmt.Sprintf("bean#%d", ctx.nextBeanId)
	ctx.nextBeanId++

	beanDefinition := ctx.ToSpringBeanDefinition(name, bean)
	ctx.RegisterBeanDefinition(beanDefinition)
}

//
// 使用默认的名称注册 SpringBean 对象
//
func (ctx *DefaultSpringContext) RegisterSingletonBean(bean SpringBean) {
	beanDefinition := ctx.ToSpringBeanDefinition("", bean)
	if ctx.beanDefinitionMap[beanDefinition.Name] != nil {
		panic(fmt.Sprintf("Singleton bean do not allow duplicate register"))
	}
	ctx.RegisterBeanDefinition(beanDefinition)
}

//
// 使用指定的名称注册 SpringBean 单例对象
//
func (ctx *DefaultSpringContext) RegisterSingletonNameBean(name string, bean SpringBean) {
	beanDefinition := ctx.ToSpringBeanDefinition(name, bean)
	ctx.RegisterBeanDefinition(beanDefinition)
}

//
// 使用指定的名称注册 SpringBean 对象
//
func (ctx *DefaultSpringContext) RegisterNameBean(name string, bean SpringBean) {
	beanDefinition := ctx.ToSpringBeanDefinition(name, bean)
	ctx.RegisterBeanDefinition(beanDefinition)
}

//
// 通过 SpringBeanDefinition 注册 SpringBean 对象
//
func (ctx *DefaultSpringContext) RegisterBeanDefinition(d *SpringBeanDefinition) {
	ctx.beanDefinitionMap[d.Name] = d
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
// 根据 Bean 类型查找 SpringBean
//
func (ctx *DefaultSpringContext) FindBeanByType(i interface{}) SpringBean {
	return ctx.beanDefinitionMap[getBeanUnameByType(checkBeanType(i))].Bean
}

func (ctx *DefaultSpringContext) FindSliceBeanByType(i interface{}) {
	it := checkBeanType(i)
	for _, beanDefinition := range ctx.beanDefinitionMap {
		if beanDefinition.Type.AssignableTo(it.Elem()) {
			v := reflect.ValueOf(i)
			v.Elem().Set(beanDefinition.Value)
		}
	}
}

//
// 根据 Bean 类型查找 SpringBean 数组
//
func (ctx *DefaultSpringContext) FindBeansByType(i interface{}) {

	it := checkBeanType(i)
	et := it.Elem()

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
// 获取所有的bean name
//
func (ctx *DefaultSpringContext) GetAllBeanNames() (names []string) {
	for name, _ := range ctx.beanDefinitionMap {
		names = append(names, name)
	}
	return
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
	beanDefinition := ctx.ToSpringBeanDefinition("", bean)
	return ctx.wireBeanByDefinition(beanDefinition)
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

	if beanDefinition.Type.Kind() == reflect.Ptr {
		t := beanDefinition.Type.Elem()
		v := beanDefinition.Value.Elem()
		if err := ctx.wireStructBeanByDefinition(t, v); err != nil {
			return err
		}
	}

	// 执行 SpringBean 的初始化接口
	if c, ok := beanDefinition.Bean.(SpringBeanInitialization); ok {
		c.InitBean(ctx)
	}

	beanDefinition.Init = Initialized
	return nil
}

//
// 为结构体做自动注入
//
func (ctx *DefaultSpringContext) wireStructBeanByDefinition(t reflect.Type, v reflect.Value) error {

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)

		// 查找依赖绑定的标签
		if beanName, ok := f.Tag.Lookup("autowire"); ok {

			var definition *SpringBeanDefinition
			var definitions []*SpringBeanDefinition

			if len(beanName) > 0 {
				definition = ctx.FindBeanDefinitionByName(beanName)
			} else {
				// 判断注入字段为是否为一个接口slice
				if f.Type.Kind() == reflect.Slice && f.Type.Elem().Kind() == reflect.Interface {
					definitions = ctx.FindBeanDefinitionsByType(f.Type.Elem())
					slice := reflect.MakeSlice(f.Type, 0, 0)
					for _, dv := range definitions {
						slice = reflect.Append(slice, dv.Value)
					}
					v.Field(i).Set(slice)
				} else {
					definitions = ctx.FindBeanDefinitionsByType(f.Type)
					if len(definitions) > 0 {
						definition = definitions[0]
					}
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
					u := cast.ToUint64(propValue)
					vf.SetUint(u)
				case reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8, reflect.Int:
					i := cast.ToInt64(propValue)
					vf.SetInt(i)
				case reflect.String:
					s := cast.ToString(propValue)
					vf.SetString(s)
				case reflect.Bool:
					b := cast.ToBool(propValue)
					vf.SetBool(b)
				default:
					return errors.New("unsupported type " + vf.Type().String())
				}
			}

			continue
		}
	}

	return nil
}
