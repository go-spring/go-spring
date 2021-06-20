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
	"errors"
	"reflect"

	"github.com/go-spring/spring-core/arg"
	"github.com/go-spring/spring-core/bean"
	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/log"
	"github.com/go-spring/spring-core/util"
)

// Pandora 提供了一些在 IoC 容器启动后基于反射获取和使用 property 与 bean 的接
// 口。因为很多人会担心在运行时大量使用反射会降低程序性能，所以命名为 Pandora，取
// 其诱人但危险的含义。事实上，这些在 IoC 容器启动后使用属性绑定和依赖注入的方案，
// 都可以转换为启动阶段的方案以提高程序的性能。
// 另一方面，为了统一 Container 和 App 两种启动方式下这些方法的使用方式，需要提取
// 出一个可共用的接口来，也就是说，无论程序是 Container 方式启动还是 App 方式启动，
// 都可以在需要使用这些方法的地方注入一个 Pandora 对象而不是 Container 对象或者
// App 对象，从而实现使用方式的统一。
type Pandora interface {
	Prop(key string, opts ...conf.GetOption) interface{}
	Get(i interface{}, opts ...GetOption) error
	Collect(i interface{}, selectors ...bean.Selector) error
	Bind(i interface{}, opts ...conf.BindOption) error
	Wire(objOrCtor interface{}, ctorArgs ...arg.Arg) (interface{}, error)
	Invoke(fn interface{}, args ...arg.Arg) ([]interface{}, error)
}

type pandora struct {
	c *Container
}

// Prop 返回 key 转为小写后精确匹配的属性值。默认情况下属性不存在时返回 nil ，但
// 是可以通过 conf.WithDefault 选项在属性不存在时返回一个默认值。另外，默认情况
// 下该方法会对返回值进行解引用，就是说如果 key 对应的属性值是一个引用，例如 ${a}，
// 那么默认情况下该方法会返回 key 为 a 的属性值，如果 a 的属性值不存在则返回 nil。
// 如果你不想对返回值进行解引用，可以通过 conf.DisableResolve 选项来关闭此功能。
func (p *pandora) Prop(key string, opts ...conf.GetOption) interface{} {
	p.c.callAfterRefreshing()
	return p.c.p.Get(key, opts...)
}

type getArg struct {
	selector bean.Selector
}

type GetOption func(arg *getArg)

func Use(s bean.Selector) GetOption {
	return func(arg *getArg) {
		arg.selector = s
	}
}

// Get 根据类型和选择器获取符合条件的 bean 对象，该方法用于精确查找某个 bean 。
// 如果没有找到或者找到多个都会返回 error。另外，这个方法和 Find 方法的区别在于
// Get 方法返回的 bean 对象能够确保已经完成属性绑定和依赖注入，而 Find 则不能。
func (p *pandora) Get(i interface{}, opts ...GetOption) error {
	p.c.callAfterRefreshing()

	if i == nil {
		return errors.New("i can't be nil")
	}

	// 使用指针才能够对外赋值
	if reflect.TypeOf(i).Kind() != reflect.Ptr {
		return errors.New("i must be pointer")
	}

	a := getArg{selector: bean.Selector("")}
	for _, opt := range opts {
		opt(&a)
	}

	stack := newWiringStack()

	defer func() {
		if len(stack.beans) > 0 {
			log.Infof("wiring path %s", stack.path())
		}
	}()

	v := reflect.ValueOf(i).Elem()
	return p.c.getBean(v, toSingletonTag(a.selector), stack)
}

func (p *pandora) Find(selector bean.Selector) ([]bean.Definition, error) {
	beans, err := p.c.find(selector)
	if err != nil {
		return nil, err
	}
	var ret []bean.Definition
	for _, b := range beans {
		ret = append(ret, b)
	}
	return ret, nil
}

// Collect 根据类型和选择器收集符合条件的 bean 对象，该方法和 Get 方法的区别
// 在于它能够返回多个和类型匹配的 bean 对象，并且符合条件的不仅仅只是单例形式的
// bean 对象，还可能包含集合形式注册的 bean 对象，该方法将集合形式 bean 对象
// 的元素当成单例 bean 对象。
// 另外，该函数有两种使用模式:自动模式和指定模式。自动模式是指 selectors 参数
// 为空，这时候不仅会收集符合条件的单例 bean，还会收集符合条件的数组 bean (将
// 数组元素拆开后一个个按照顺序放到收集结果里面)。指定模式是指 selectors 参数
// 不为空，这时候只会收集符合条件的单例 bean，原因是该模式下会根据 selectors
// 参数的顺序对收集结果进行排序。
func (p *pandora) Collect(i interface{}, selectors ...bean.Selector) error {
	p.c.callAfterRefreshing()

	v := reflect.ValueOf(i)
	if v.Kind() != reflect.Ptr {
		return errors.New("i must be slice ptr")
	}

	stack := newWiringStack()

	defer func() {
		if len(stack.beans) > 0 {
			log.Infof("wiring path %s", stack.path())
		}
	}()

	var tag collectionTag
	for _, selector := range selectors {
		s := toSingletonTag(selector)
		tag.beanTags = append(tag.beanTags, s)
	}
	return p.c.collectBeans(v.Elem(), tag, stack)
}

// Bind 对传入的对象进行属性绑定，注意该方法不会进行依赖注入，支持基本数据类型
// 及结构体类型。
func (p *pandora) Bind(i interface{}, opts ...conf.BindOption) error {
	p.c.callAfterRefreshing()
	return p.c.p.Bind(i, opts...)
}

// Wire 如果传入的是 bean 对象，则对 bean 对象进行属性绑定和依赖注入，如果传
// 入的是构造函数，则立即执行构造函数，然后对返回的结果进行属性绑定和依赖注入。
// 无论哪种方式，该函数执行完后都会返回被处理的对象。
func (p *pandora) Wire(objOrCtor interface{}, ctorArgs ...arg.Arg) (interface{}, error) {
	p.c.callAfterRefreshing()

	stack := newWiringStack()

	defer func() {
		if len(stack.beans) > 0 {
			log.Infof("wiring path %s", stack.path())
		}
	}()

	b := NewBean(objOrCtor, ctorArgs...)
	err := p.c.wireBean(b, stack)
	if err != nil {
		return nil, err
	}
	return b.Interface(), nil
}

// Invoke fn 形似配置函数，但是可以返回多个值，fn 的执行结果以数组的形式返回。
func (p *pandora) Invoke(fn interface{}, args ...arg.Arg) ([]interface{}, error) {
	p.c.callAfterRefreshing()

	if !util.IsFuncType(reflect.TypeOf(fn)) {
		return nil, errors.New("fn should be function")
	}

	stack := newWiringStack()

	defer func() {
		if len(stack.beans) > 0 {
			log.Infof("wiring path %s", stack.path())
		}
	}()

	c := arg.Bind(fn, args, 1)
	ret, err := c.Call(newArgContext(p.c, stack))
	if err != nil {
		return nil, err
	}

	var a []interface{}
	for _, v := range ret {
		a = append(a, v.Interface())
	}
	return a, nil
}
