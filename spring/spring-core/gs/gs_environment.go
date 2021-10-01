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
	"context"
	"errors"
	"reflect"

	"github.com/go-spring/spring-base/conf"
	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-core/gs/arg"
	"github.com/go-spring/spring-core/gs/cond"
)

// Environment 提供了一些在 IoC 容器启动后基于反射获取和使用 property 与 bean 的接
// 口。因为很多人会担心在运行时大量使用反射会降低程序性能，所以命名为 Environment，取
// 其诱人但危险的含义。事实上，这些在 IoC 容器启动后使用属性绑定和依赖注入的方案，
// 都可以转换为启动阶段的方案以提高程序的性能。
// 另一方面，为了统一 Container 和 App 两种启动方式下这些方法的使用方式，需要提取
// 出一个可共用的接口来，也就是说，无论程序是 Container 方式启动还是 App 方式启动，
// 都可以在需要使用这些方法的地方注入一个 Environment 对象而不是 Container 对象或者
// App 对象，从而实现使用方式的统一。
type Environment interface {
	Context() context.Context
	HasProperty(key string) bool
	GetProperty(key string, opts ...conf.GetOption) string
	Bind(i interface{}, opts ...conf.BindOption) error
	GetBean(i interface{}, selectors ...BeanSelector) error
	Wire(objOrCtor interface{}, ctorArgs ...arg.Arg) (interface{}, error)
	Invoke(fn interface{}, args ...arg.Arg) ([]interface{}, error)
	Go(fn func(ctx context.Context))
}

func (c *container) HasProperty(key string) bool {
	return c.p.Has(key)
}

func (c *container) GetProperty(key string, opts ...conf.GetOption) string {
	return c.p.Get(key, opts...)
}

func (c *container) Bind(i interface{}, opts ...conf.BindOption) error {
	return c.p.Bind(i, opts...)
}

// FindBean 查找符合条件的 bean 对象，注意该函数只能保证返回的 bean 是有效的，即未被
// 标记为删除的，而不能保证已经完成属性绑定和依赖注入。
func (c *container) FindBean(selector BeanSelector) ([]cond.BeanDefinition, error) {
	beans, err := c.findBean(selector)
	if err != nil {
		return nil, err
	}
	var ret []cond.BeanDefinition
	for _, b := range beans {
		ret = append(ret, b)
	}
	return ret, nil
}

// GetBean 根据类型和选择器获取符合条件的 bean 对象。当 i 是一个基础类型的 bean 接收
// 者时，表示符合条件的 bean 对象只能有一个，没有找到或者多于一个时会返回 error。
// 当 i 是一个 map 类型的 bean 接收者时，表示获取任意数量的 bean 对象，map 的
// key 是 bean 的名称，map 的 value 是 bean 的地址。当 i 是一个 array 或者
// slice 时，也表示获取任意数量的 bean 对象，但是它会对获取到的 bean 对象进行排序，
// 如果没有传入选择器或者传入的选择器是 * ，则根据 bean 的 order 值进行排序，这种
// 工作模式称为自动模式，否则根据传入的选择器列表进行排序，这种工作模式成为指派模式。
// 该方法和 Find 方法的区别是该方法保证返回的所有 bean 对象都已经完成属性绑定和依
// 赖注入，而 Find 方法只能保证返回的 bean 对象是有效的，即未被标记为删除的。
func (c *container) GetBean(i interface{}, selectors ...BeanSelector) error {

	if i == nil {
		return errors.New("i can't be nil")
	}

	v := reflect.ValueOf(i)
	if v.Kind() != reflect.Ptr {
		return errors.New("i must be pointer")
	}

	stack := newWiringStack()

	defer func() {
		if len(stack.beans) > 0 {
			log.Infof("wiring path %s", stack.path())
		}
	}()

	var tags []wireTag
	for _, s := range selectors {
		tags = append(tags, toWireTag(s))
	}
	return c.autowire(v.Elem(), tags, stack)
}

// Wire 如果传入的是 bean 对象，则对 bean 对象进行属性绑定和依赖注入，如果传入的
// 是构造函数，则立即执行该构造函数，然后对返回的结果进行属性绑定和依赖注入。无论哪
// 种方式，该函数执行完后都会返回 bean 对象的真实值。
func (c *container) Wire(objOrCtor interface{}, ctorArgs ...arg.Arg) (interface{}, error) {

	stack := newWiringStack()

	defer func() {
		if len(stack.beans) > 0 {
			log.Infof("wiring path %s", stack.path())
		}
	}()

	b := NewBean(objOrCtor, ctorArgs...)
	err := c.wireBean(b, stack)
	if err != nil {
		return nil, err
	}
	return b.Interface(), nil
}

func (c *container) Invoke(fn interface{}, args ...arg.Arg) ([]interface{}, error) {

	if !util.IsFuncType(reflect.TypeOf(fn)) {
		return nil, errors.New("fn should be func type")
	}

	stack := newWiringStack()

	defer func() {
		if len(stack.beans) > 0 {
			log.Infof("wiring path %s", stack.path())
		}
	}()

	r, err := arg.Bind(fn, args, 1)
	if err != nil {
		return nil, err
	}

	ret, err := r.Call(&argContext{c: c, stack: stack})
	if err != nil {
		return nil, err
	}

	var a []interface{}
	for _, v := range ret {
		a = append(a, v.Interface())
	}
	return a, nil
}
