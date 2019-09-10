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
	"reflect"
	"errors"
	"strconv"
	"fmt"
	"strings"
)

//
// SpringBean 需要实现的接口
//
type SpringBean interface{}

//
// 如果 SpringBean 需要初始化则实现该接口
//
type SpringBeanInitialization interface {
	InitBean(ctx SpringContext) error
}

const (
	Uninitialized = iota
	Initializing
	Initialized
)

//
// SpringBean 的定义
//
type SpringBeanDefinition struct {
	Bean  SpringBean
	Name  string
	Init  int
	Type  reflect.Type
	Value reflect.Value
}

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

func checkSpringBean(bean SpringBean) (t reflect.Type, v reflect.Value) {
	t = reflect.TypeOf(bean)
	if t.Kind() != reflect.Ptr {
		panic("bean must be pointer")
	}
	v = reflect.ValueOf(bean)
	return
}

//
// 注册 SpringBean
//
func (ctx *DefaultSpringContext) RegisterNameBean(name string, bean SpringBean) {
	t, v := checkSpringBean(bean)
	ctx.registerBeanDefinition(name, bean, t, v)
}

//
// 注册 SpringBean
//
func (ctx *DefaultSpringContext) RegisterBean(bean SpringBean) {
	t, v := checkSpringBean(bean)
	ctx.registerBeanDefinition(t.Elem().Name(), bean, t, v)
}

//
// 根据名称查找 SpringBean 的定义
//
func (ctx *DefaultSpringContext) FindBeanDefinitionByName(name string) *SpringBeanDefinition {
	return ctx.beanDefinitionMap[name]
}

//
// 根据名称查找 SpringBean
//
func (ctx *DefaultSpringContext) FindBeanByName(name string) SpringBean {
	if beanDefinition, ok := ctx.beanDefinitionMap[name]; ok {
		return beanDefinition.Bean
	}
	return nil
}

//
// 根据类型查找 SpringBean 的定义
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
// 根据类型查找 SpringBean
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
// 自动绑定
//
func (ctx *DefaultSpringContext) AutoWireBeans() error {
	for _, beanDefinition := range ctx.beanDefinitionMap {
		if err := ctx.WireBean(beanDefinition); err != nil {
			return err
		}
	}
	return nil
}

//
// 遍历所有 SpringBean 使用的回调函数
//
func (ctx *DefaultSpringContext) WireBean(beanDefinition *SpringBeanDefinition) error {

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

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)

		if beanName, ok := f.Tag.Lookup("autowire"); ok {
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
				ctx.WireBean(definition)
				v.Field(i).Set(definition.Value)
			}

			continue
		}

		if value, ok := f.Tag.Lookup("value"); ok && len(value) > 0 {

			if strings.HasPrefix(value, "${") {
				str := value[2 : len(value)-1]
				ss := strings.Split(str, ":=")

				var (
					propName  string
					propValue string
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
					if u64, err := strconv.ParseUint(propValue, 10, vf.Type().Bits()); err != nil {
						return err
					} else {
						vf.SetUint(u64)
					}
				case reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8, reflect.Int:
					if i64, err := strconv.ParseInt(propValue, 10, vf.Type().Bits()); err != nil {
						return err
					} else {
						vf.SetInt(i64)
					}
				case reflect.String:
					vf.SetString(propValue)
				case reflect.Bool:
					if b, err := strconv.ParseBool(propValue); err == nil {
						vf.SetBool(b)
					} else {
						return errors.New("error bool value " + propValue)
					}
				default:
					return errors.New("unsupported type " + vf.Type().String())
				}
			}

			continue
		}
	}

	// 初始化当前的 SpringBean
	if c, ok := beanDefinition.Bean.(SpringBeanInitialization); ok {
		if err := c.InitBean(ctx); err != nil {
			return err
		}
	}

	beanDefinition.Init = Initialized
	return nil
}

//
// 根据类型获取 SpringBean，确保 Bean 已经初始化。
//
func (ctx *DefaultSpringContext) GetBeanByType(i interface{}) {

	it := reflect.TypeOf(i)
	et := it.Elem()

	if it.Kind() != reflect.Ptr {
		panic("bean must be pointer")
	}

	v := reflect.ValueOf(i).Elem()

	for _, beanDefinition := range ctx.beanDefinitionMap {
		if beanDefinition.Type.AssignableTo(et) {

			if beanDefinition.Init == Initializing {
				panic("循环依赖")
			}

			if beanDefinition.Init == Uninitialized {
				ctx.WireBean(beanDefinition)
			}

			v.Set(beanDefinition.Value)
			return
		}
	}
}
