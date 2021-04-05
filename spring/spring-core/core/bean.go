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

package core

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"

	"github.com/go-spring/spring-core/arg"
	"github.com/go-spring/spring-core/bean"
	"github.com/go-spring/spring-core/cond"
	"github.com/go-spring/spring-core/util"
)

// toSingletonTag 将 bean.Selector 转换为 singletonTag 形式。
func toSingletonTag(selector bean.Selector) singletonTag {
	switch s := selector.(type) {
	case string:
		return parseSingletonTag(s)
	case *BeanDefinition:
		return parseSingletonTag(s.BeanId())
	default:
		return parseSingletonTag(util.TypeName(s) + ":")
	}
}

// singletonTag 单例模式注入 Tag 对应的分解形式
type singletonTag struct {
	typeName string
	beanName string
	nullable bool
}

func (tag singletonTag) String() string {
	b := bytes.NewBuffer(nil)
	if tag.typeName != "" {
		b.WriteString(tag.typeName)
		b.WriteString(":")
	}
	b.WriteString(tag.beanName)
	if tag.nullable {
		b.WriteString("?")
	}
	return b.String()
}

// parseSingletonTag 解析单例模式注入 Tag 字符串
func parseSingletonTag(str string) (tag singletonTag) {
	if len(str) > 0 {

		// 字符串结尾是否有可空标记
		if n := len(str) - 1; str[n] == '?' {
			tag.nullable = true
			str = str[:n]
		}

		if i := strings.Index(str, ":"); i > -1 { // 完整形式
			tag.beanName = str[i+1:]
			tag.typeName = str[:i]
		} else { // 简化形式
			tag.beanName = str
		}
	}
	return
}

// collectionTag 收集模式注入 Tag 对应的分解形式
type collectionTag struct {
	beanTags []singletonTag
	nullable bool
}

func (tag collectionTag) String() string {
	b := bytes.NewBuffer(nil)
	b.WriteString("[")
	for i, t := range tag.beanTags {
		b.WriteString(t.String())
		if i < len(tag.beanTags)-1 {
			b.WriteString(",")
		}
	}
	b.WriteString("]")
	if tag.nullable {
		b.WriteString("?")
	}
	return b.String()
}

// CollectionMode 返回是否是收集模式
func CollectionMode(str string) bool {
	return len(str) > 0 && str[0] == '['
}

// ParseCollectionTag 解析收集模式注入 Tag 字符串
func parseCollectionTag(str string) (tag collectionTag) {
	tag.beanTags = make([]singletonTag, 0)

	// 字符串结尾是否有可空标记
	if n := len(str) - 1; str[n] == '?' {
		tag.nullable = true
		str = str[:n]
	}

	if str[len(str)-1] != ']' {
		panic(errors.New("error collection tag"))
	}

	if str = str[1 : len(str)-1]; len(str) > 0 {
		for _, s := range strings.Split(str, ",") {
			tag.beanTags = append(tag.beanTags, parseSingletonTag(s))
		}
	}
	return
}

// beanStatus Bean 的状态值
type beanStatus int

const (
	Default   = beanStatus(0) // 默认状态
	Resolving = beanStatus(1) // 正在决议
	Resolved  = beanStatus(2) // 已决议
	Wiring    = beanStatus(3) // 正在注入
	Wired     = beanStatus(4) // 注入完成
	Deleted   = beanStatus(5) // 已删除
)

type beanDefinition interface {
	bean.Definition

	getFactory() arg.Callable
	getStatus() beanStatus         // 返回 Bean 的状态值
	getDependsOn() []bean.Selector // 返回 Bean 的间接依赖项
	getInit() arg.Runnable         // 返回 Bean 的初始化函数
	getDestroy() arg.Runnable      // 返回 Bean 的销毁函数
	getFile() string               // 返回 Bean 注册点所在文件的名称
	getLine() int                  // 返回 Bean 注册点所在文件的行数
	getClass() string

	setStatus(status beanStatus) // 设置 Bean 的状态值
}

// BeanDefinition 用于存储 Bean 的各种元数据
type BeanDefinition struct {
	f arg.Callable  // 构造函数
	v reflect.Value // 值
	t reflect.Type  // 类型

	typeName string // 原始类型的全限定名

	name   string     // Bean 的名称，请勿直接使用该字段!
	status beanStatus // Bean 的状态

	file string // 注册点所在文件
	line int    // 注册点所在行数

	cond      cond.Condition  // 判断条件
	primary   bool            // 是否为主版本
	dependsOn []bean.Selector // 间接依赖项

	init    arg.Runnable // 初始化函数
	destroy arg.Runnable // 销毁函数

	exports map[reflect.Type]struct{} // 严格导出的接口类型
}

// newBeanDefinition BeanDefinition 的构造函数
func newBeanDefinition(v reflect.Value, ctor arg.Callable, file string, line int) *BeanDefinition {
	t := v.Type()
	if !util.IsRefType(t.Kind()) {
		panic(errors.New("bean must be ref type"))
	}
	return &BeanDefinition{
		t:        t,
		v:        v,
		f:        ctor,
		typeName: util.TypeName(t),
		status:   Default,
		file:     file,
		line:     line,
		exports:  make(map[reflect.Type]struct{}),
	}
}

// Type 返回 Bean 的类型
func (d *BeanDefinition) Type() reflect.Type {
	return d.t
}

// Value 返回 Bean 的值
func (d *BeanDefinition) Value() reflect.Value {
	return d.v
}

// Bean 返回 Bean 的源
func (d *BeanDefinition) Interface() interface{} {
	return d.Value().Interface()
}

// BeanId 返回 Bean 的唯一 ID
func (d *BeanDefinition) BeanId() string {
	return d.TypeName() + ":" + d.BeanName()
}

// Name 返回 Bean 的名称
func (d *BeanDefinition) BeanName() string {
	if d.name == "" {
		// 统一使用类型字符串作为默认名称!
		d.name = d.t.String()
	}
	return d.name
}

// TypeName 返回 Bean 的原始类型的全限定名
func (d *BeanDefinition) TypeName() string {
	return d.typeName
}

// FileLine 返回 Bean 的注册点
func (d *BeanDefinition) FileLine() string {
	return fmt.Sprintf("%s:%d", d.file, d.line)
}

// Description 返回 Bean 的详细描述
func (d *BeanDefinition) Description() string {
	return fmt.Sprintf("%s %q %s", d.getClass(), d.BeanName(), d.FileLine())
}

func (d *BeanDefinition) getFactory() arg.Callable {
	return d.f
}

// getStatus 返回 Bean 的状态值
func (d *BeanDefinition) getStatus() beanStatus {
	return d.status
}

// setStatus 设置 Bean 的状态值
func (d *BeanDefinition) setStatus(status beanStatus) {
	d.status = status
}

// getDependsOn 返回 Bean 的间接依赖项
func (d *BeanDefinition) getDependsOn() []bean.Selector {
	return d.dependsOn
}

// getInit 返回 Bean 的初始化函数
func (d *BeanDefinition) getInit() arg.Runnable {
	return d.init
}

// getDestroy 返回 Bean 的销毁函数
func (d *BeanDefinition) getDestroy() arg.Runnable {
	return d.destroy
}

// getFile 返回 Bean 注册点所在文件的名称
func (d *BeanDefinition) getFile() string {
	return d.file
}

// getLine 返回 Bean 注册点所在文件的行数
func (d *BeanDefinition) getLine() int {
	return d.line
}

func (d *BeanDefinition) getClass() string {
	if d.f == nil {
		return "object bean"
	}
	return "constructor bean"
}

// Match 测试 Bean 的类型全限定名和 Bean 的名称是否都匹配
func (d *BeanDefinition) Match(typeName string, beanName string) bool {

	typeIsSame := false
	if typeName == "" || d.TypeName() == typeName {
		typeIsSame = true
	}

	nameIsSame := false
	if beanName == "" || d.BeanName() == beanName {
		nameIsSame = true
	}

	return typeIsSame && nameIsSame
}

// WithName 设置 Bean 的名称
func (d *BeanDefinition) WithName(name string) *BeanDefinition {
	d.name = name
	return d
}

// WithCond 为 Bean 设置一个 Condition
func (d *BeanDefinition) WithCond(cond cond.Condition) *BeanDefinition {
	d.cond = cond
	return d
}

// DependsOn 设置 Bean 的间接依赖项
func (d *BeanDefinition) DependsOn(selectors ...bean.Selector) *BeanDefinition {
	d.dependsOn = append(d.dependsOn, selectors...)
	return d
}

// primary 设置 Bean 为主版本
func (d *BeanDefinition) Primary(primary bool) *BeanDefinition {
	d.primary = primary
	return d
}

// validLifeCycleFunc 判断是否是合法的用于 Bean 生命周期控制的函数，生命周期函数的要求：
// 至少一个参数，且第一个参数的类型必须是 Bean 的类型，没有返回值或者只能返回 error 类型值。
func validLifeCycleFunc(fn interface{}, beanType reflect.Type) (reflect.Type, bool) {
	fnType := reflect.TypeOf(fn)
	if util.FuncType(fnType) && util.WithReceiver(fnType, beanType) {
		if util.ReturnNothing(fnType) || util.ReturnOnlyError(fnType) {
			return fnType, true
		}
	}
	return nil, false
}

// Init 设置 Bean 的初始化函数，args 是初始化函数的一般参数绑定
func (d *BeanDefinition) Init(fn interface{}, args ...arg.Arg) *BeanDefinition {
	if _, ok := validLifeCycleFunc(fn, d.Type()); ok {
		d.init = arg.Runner(fn, true, args)
		return d
	}
	panic(errors.New("init should be func(bean) or func(bean)error"))
}

// Destroy 设置 Bean 的销毁函数，args 是销毁函数的一般参数绑定
func (d *BeanDefinition) Destroy(fn interface{}, args ...arg.Arg) *BeanDefinition {
	if _, ok := validLifeCycleFunc(fn, d.Type()); ok {
		d.destroy = arg.Runner(fn, true, args)
		return d
	}
	panic(errors.New("destroy should be func(bean) or func(bean)error"))
}

// Export 显式指定 Bean 的导出接口
func (d *BeanDefinition) Export(exports ...interface{}) *BeanDefinition {
	for _, o := range exports { // 使用 map 进行排重

		var typ reflect.Type
		if t, ok := o.(reflect.Type); ok {
			typ = t
		} else { // 处理 (*error)(nil) 这种导出形式
			typ = util.Indirect(reflect.TypeOf(o))
		}

		if typ.Kind() == reflect.Interface {
			d.exports[typ] = struct{}{}
		} else {
			panic(errors.New("must export interface type"))
		}
	}
	return d
}

// NewBean 普通函数注册时需要使用 reflect.ValueOf(fn) 的方式避免和构造函数发生冲突。
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

	var (
		file string
		line int
	)

	for i := 2; i < 10; i++ {
		_, f, l, _ := runtime.Caller(i)
		if strings.Contains(f, "/spring-core/") {
			if !strings.HasSuffix(f, "_test.go") {
				continue
			}
		}
		file = f
		line = l
		break
	}

	// 以 reflect.ValueOf(fn) 方式注册的函数被视为对象 Bean 。
	if t := v.Type(); !fromValue && t.Kind() == reflect.Func {

		// 检查 Bean 的注册函数是否合法
		if !bean.IsFactoryType(t) {
			t1 := "func(...)bean"
			t2 := "func(...)(bean, error)"
			panic(fmt.Errorf("func bean must be %s or %s", t1, t2))
		}

		// 创建 Bean 的值
		out0 := t.Out(0)
		v := reflect.New(out0)

		// 引用类型去掉一层指针
		if util.IsRefType(out0.Kind()) {
			v = v.Elem()
		}

		ctor := arg.Caller(objOrCtor, false, ctorArgs)
		return newBeanDefinition(v, ctor, file, line)
	}

	return newBeanDefinition(v, nil, file, line)
}
