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

// Get 获取单例 bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
// 它和 FindBean 的区别是它在调用后能够保证返回的 bean 已经完成了注入和绑定过程。
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

	w := toAssembly(p.c)
	v := reflect.ValueOf(i).Elem()
	return w.getBean(toSingletonTag(a.selector), v)
}

func (p *pandora) Find(selector bean.Selector) (bean.Definition, error) {
	// 如果此处直接返回会触发臭名昭著的 interface{} == nil 返回 false 的问题。
	b, err := p.c.find(selector)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, nil
	}
	return b, nil
}

// Collect 收集数组或指针定义的所有符合条件的 bean，收集到返回 true，否则返
// 回 false。该函数有两种模式:自动模式和指定模式。自动模式是指 selectors 参数为空，
// 这时候不仅会收集符合条件的单例 bean，还会收集符合条件的数组 bean (是指数组的元素
// 符合条件，然后把数组元素拆开一个个放到收集结果里面)。指定模式是指 selectors 参数
// 不为空，这时候只会收集单例 bean，而且要求这些单例 bean 不仅需要满足收集条件，而且
// 必须满足 selector 条件。另外，自动模式下不对收集结果进行排序，指定模式下根据
// selectors 列表的顺序对收集结果进行排序。
func (p *pandora) Collect(i interface{}, selectors ...bean.Selector) error {
	p.c.callAfterRefreshing()

	v := reflect.ValueOf(i)
	if v.Kind() != reflect.Ptr {
		return errors.New("i must be slice ptr")
	}

	var tag collectionTag
	for _, selector := range selectors {
		s := toSingletonTag(selector)
		tag.beanTags = append(tag.beanTags, s)
	}
	return toAssembly(p.c).collectBeans(tag, v.Elem())
}

// Bind 对传入的对象进行属性绑定。
func (p *pandora) Bind(i interface{}, opts ...conf.BindOption) error {
	p.c.callAfterRefreshing()
	return p.c.p.Bind(i, opts...)
}

// Wire 对传入的对象或者构造函数的执行结果进行属性绑定和依赖注入，并返回处理后的对象。
func (p *pandora) Wire(objOrCtor interface{}, ctorArgs ...arg.Arg) (interface{}, error) {
	p.c.callAfterRefreshing()

	b := NewBean(objOrCtor, ctorArgs...)
	err := toAssembly(p.c).wireBean(b)
	if err != nil {
		return nil, err
	}
	return b.Interface(), nil
}

// Invoke 立即执行 fn 并返回执行结果。
func (p *pandora) Invoke(fn interface{}, args ...arg.Arg) ([]interface{}, error) {
	p.c.callAfterRefreshing()

	if !util.IsFuncType(reflect.TypeOf(fn)) {
		return nil, errors.New("fn should be func(...) or func(...)error")
	}

	c := arg.Bind(fn, args, arg.Skip(1))
	ret, err := c.Call(toAssembly(p.c))
	if err != nil {
		return nil, err
	}

	var a []interface{}
	for _, v := range ret {
		a = append(a, v.Interface())
	}
	return a, nil
}
