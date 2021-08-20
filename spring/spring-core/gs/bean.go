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
	"fmt"
	"math"
	"reflect"
	"runtime"
	"strings"

	"github.com/go-spring/spring-boost/util"
	"github.com/go-spring/spring-core/gs/arg"
	"github.com/go-spring/spring-core/gs/cond"
)

const (
	HighestOrder = math.MinInt32
	LowestOrder  = math.MaxInt32
)

type beanStatus int8

const (
	Deleted   = beanStatus(-1)   // 已删除
	Default   = beanStatus(iota) // 未处理
	Resolving                    // 正在决议
	Resolved                     // 已决议
	Creating                     // 正在创建
	Created                      // 已创建
	Wired                        // 注入完成
)

// BeanDefinition bean 元数据。
type BeanDefinition struct {

	// 原始类型的全限定名
	typeName string

	v reflect.Value // 值
	t reflect.Type  // 类型
	f *arg.Callable // 构造函数

	file string // 注册点所在文件
	line int    // 注册点所在行数

	name      string                    // 名称
	status    beanStatus                // 状态
	cond      cond.Condition            // 判断条件
	primary   bool                      // 是否为主版本
	order     int                       // 收集时的顺序
	init      interface{}               // 初始化函数
	destroy   interface{}               // 销毁函数
	dependsOn []cond.BeanSelector       // 间接依赖项
	exports   map[reflect.Type]struct{} // 导出的接口
}

// Type 返回 bean 的类型。
func (d *BeanDefinition) Type() reflect.Type {
	return d.t
}

// Value 返回 bean 的值。
func (d *BeanDefinition) Value() reflect.Value {
	return d.v
}

// Interface 返回 bean 的真实值。
func (d *BeanDefinition) Interface() interface{} {
	return d.v.Interface()
}

// ID 返回 bean 的 ID 。
func (d *BeanDefinition) ID() string {
	return d.typeName + ":" + d.name
}

// BeanName 返回 bean 的名称。
func (d *BeanDefinition) BeanName() string {
	return d.name
}

// TypeName 返回 bean 的原始类型的全限定名。
func (d *BeanDefinition) TypeName() string {
	return d.typeName
}

// Created 返回是否已创建。
func (d *BeanDefinition) Created() bool {
	return d.status >= Created
}

// Wired 返回 bean 是否已经注入。
func (d *BeanDefinition) Wired() bool {
	return d.status == Wired
}

// FileLine 返回 bean 的注册点。
func (d *BeanDefinition) FileLine() string {
	return fmt.Sprintf("%s:%d", d.file, d.line)
}

// getClass 返回 bean 的类型描述。
func (d *BeanDefinition) getClass() string {
	if d.f == nil {
		return "object bean"
	}
	return "constructor bean"
}

func (d *BeanDefinition) String() string {
	return fmt.Sprintf("%s name:%q %s", d.getClass(), d.name, d.FileLine())
}

// Match 测试 bean 的类型全限定名和 bean 的名称是否都匹配。
func (d *BeanDefinition) Match(typeName string, beanName string) bool {

	typeIsSame := false
	if typeName == "" || d.typeName == typeName {
		typeIsSame = true
	}

	nameIsSame := false
	if beanName == "" || d.name == beanName {
		nameIsSame = true
	}

	return typeIsSame && nameIsSame
}

// Name 设置 bean 的名称。
func (d *BeanDefinition) Name(name string) *BeanDefinition {
	d.name = name
	return d
}

// On 设置 bean 的 Condition。
func (d *BeanDefinition) On(cond cond.Condition) *BeanDefinition {
	d.cond = cond
	return d
}

// Order 设置 bean 的排序序号，值越小顺序越靠前(优先级越高)。
func (d *BeanDefinition) Order(order int) *BeanDefinition {
	d.order = order
	return d
}

// DependsOn 设置 bean 的间接依赖项。
func (d *BeanDefinition) DependsOn(selectors ...cond.BeanSelector) *BeanDefinition {
	d.dependsOn = append(d.dependsOn, selectors...)
	return d
}

// Primary 设置 bean 为主版本。
func (d *BeanDefinition) Primary() *BeanDefinition {
	d.primary = true
	return d
}

// validLifeCycleFunc 判断是否是合法的用于 bean 生命周期控制的函数，生命周期函数
// 的要求：只能有一个入参并且必须是 bean 的类型，没有返回值或者只返回 error 类型值。
func validLifeCycleFunc(fnType reflect.Type, beanType reflect.Type) bool {
	if !util.IsFuncType(fnType) {
		return false
	}
	if fnType.NumIn() != 1 || !util.HasReceiver(fnType, beanType) {
		return false
	}
	return util.ReturnNothing(fnType) || util.ReturnOnlyError(fnType)
}

// Init 设置 bean 的初始化函数。
func (d *BeanDefinition) Init(fn interface{}) *BeanDefinition {
	if validLifeCycleFunc(reflect.TypeOf(fn), d.Type()) {
		d.init = fn
		return d
	}
	panic(errors.New("init should be func(bean) or func(bean)error"))
}

// Destroy 设置 bean 的销毁函数。
func (d *BeanDefinition) Destroy(fn interface{}) *BeanDefinition {
	if validLifeCycleFunc(reflect.TypeOf(fn), d.Type()) {
		d.destroy = fn
		return d
	}
	panic(errors.New("destroy should be func(bean) or func(bean)error"))
}

// Export 设置 bean 的导出接口。
func (d *BeanDefinition) Export(exports ...interface{}) *BeanDefinition {
	err := d.export(exports...)
	util.Panic(err).When(err != nil)
	return d
}

func (d *BeanDefinition) export(exports ...interface{}) error {
	for _, o := range exports {
		var typ reflect.Type
		if t, ok := o.(reflect.Type); ok {
			typ = t
		} else { // 处理 (*error)(nil) 这种导出形式
			typ = util.Indirect(reflect.TypeOf(o))
		}
		if typ.Kind() != reflect.Interface {
			return errors.New("only interface type can be exported")
		}
		d.exports[typ] = struct{}{}
	}
	return nil
}

// NewBean 普通函数注册时需要使用 reflect.ValueOf(fn) 形式以避免和构造函数发生冲突。
func NewBean(objOrCtor interface{}, ctorArgs ...arg.Arg) *BeanDefinition {

	var v reflect.Value
	var fromValue bool

	switch i := objOrCtor.(type) {
	case reflect.Value:
		fromValue = true
		v = i
	default:
		v = reflect.ValueOf(i)
	}

	if !v.IsValid() || util.IsNil(v) {
		panic(errors.New("bean can't be nil"))
	}

	const skip = 2
	var f *arg.Callable
	_, file, line, _ := runtime.Caller(skip)

	// 以 reflect.ValueOf(fn) 形式注册的函数被视为函数对象 bean 。
	if t := v.Type(); !fromValue && t.Kind() == reflect.Func {

		if !util.IsConstructor(t) {
			t1 := "func(...)bean"
			t2 := "func(...)(bean, error)"
			panic(fmt.Errorf("constructor should be %s or %s", t1, t2))
		}

		var err error
		f, err = arg.Bind(objOrCtor, ctorArgs, skip)
		util.Panic(err).When(err != nil)

		out0 := t.Out(0)
		v = reflect.New(out0)

		// 引用类型去掉指针，值类型则刚刚好。
		if util.IsBeanType(out0) {
			v = v.Elem()
		}
	}

	t := v.Type()
	if !util.IsBeanType(t) {
		panic(errors.New("bean must be ref type"))
	}

	if t.Kind() == reflect.Ptr && !util.IsValueType(t.Elem()) {
		panic(errors.New("bean should be *val but not *ref"))
	}

	// Type.String() 一般返回 *pkg.Type 形式的字符串，
	// 我们只取最后的类型名，如有需要请自定义 bean 名称。
	s := strings.Split(t.String(), ".")
	name := s[len(s)-1]

	return &BeanDefinition{
		t:        t,
		v:        v,
		f:        f,
		name:     name,
		typeName: util.TypeName(t),
		status:   Default,
		order:    LowestOrder,
		file:     file,
		line:     line,
		exports:  make(map[reflect.Type]struct{}),
	}
}
