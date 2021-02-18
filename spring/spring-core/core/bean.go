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
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"

	"github.com/go-spring/spring-core/arg"
	"github.com/go-spring/spring-core/bean"
	"github.com/go-spring/spring-core/util"
)

// errorType error 的反射类型
var errorType = reflect.TypeOf((*error)(nil)).Elem()

// ToSingletonTag 将 Bean 选择器转换为 SingletonTag 形式。注意该函数仅用
// 于精确匹配的场景下，也就是说通过类型选择的时候类型必须是具体的，而不能是接口。
func ToSingletonTag(selector bean.Selector) SingletonTag {
	switch s := selector.(type) {
	case string:
		return parseSingletonTag(s)
	case *BeanDefinition:
		return parseSingletonTag(s.BeanId())
	default:
		return parseSingletonTag(bean.TypeName(s) + ":")
	}
}

// SingletonTag 单例模式注入 Tag 对应的分解形式
type SingletonTag struct {
	TypeName string
	BeanName string
	Nullable bool
}

func (tag SingletonTag) String() (str string) {
	if tag.TypeName != "" {
		str = tag.TypeName + ":"
	}
	str += tag.BeanName
	if tag.Nullable {
		str += "?"
	}
	return
}

// parseSingletonTag 解析单例模式注入 Tag 字符串
func parseSingletonTag(str string) (tag SingletonTag) {
	if len(str) > 0 {

		// 字符串结尾是否有可空标记
		if n := len(str) - 1; str[n] == '?' {
			tag.Nullable = true
			str = str[:n]
		}

		if i := strings.Index(str, ":"); i > -1 { // 完整形式
			tag.BeanName = str[i+1:]
			tag.TypeName = str[:i]
		} else { // 简化形式
			tag.BeanName = str
		}
	}
	return
}

// collectionTag 收集模式注入 Tag 对应的分解形式
type collectionTag struct {
	Items    []SingletonTag
	Nullable bool
}

func (tag collectionTag) String() (str string) {
	str += "["
	for i, t := range tag.Items {
		str += t.String()
		if i < len(tag.Items)-1 {
			str += ","
		}
	}
	str += "]"
	if tag.Nullable {
		str += "?"
	}
	return
}

// CollectionMode 返回是否是收集模式
func CollectionMode(str string) bool {
	return len(str) > 0 && str[0] == '['
}

// ParseCollectionTag 解析收集模式注入 Tag 字符串
func ParseCollectionTag(str string) (tag collectionTag) {
	tag.Items = make([]SingletonTag, 0)

	// 字符串结尾是否有可空标记
	if n := len(str) - 1; str[n] == '?' {
		tag.Nullable = true
		str = str[:n]
	}

	if str[len(str)-1] != ']' {
		panic(errors.New("error collection tag"))
	}

	if str = str[1 : len(str)-1]; len(str) > 0 {
		for _, s := range strings.Split(str, ",") {
			tag.Items = append(tag.Items, parseSingletonTag(s))
		}
	}
	return
}

type beanFactory interface {
	beanClass() string
	newValue() reflect.Value
	beanType() reflect.Type
}

type objBeanFactory struct {
	v reflect.Value
}

func (b *objBeanFactory) newValue() reflect.Value {
	return b.v
}

func (b *objBeanFactory) beanType() reflect.Type {
	return b.v.Type()
}

func (b *objBeanFactory) beanClass() string {
	return "object bean"
}

// IsFuncBeanType 返回以函数形式注册 Bean 的函数是否合法。一个合法
// 的注册函数需要以下条件：入参可以有任意多个，支持一般形式和 Option
// 形式，返回值只能有一个或者两个，第一个返回值必须是 Bean 源，它可以是
// 结构体等值类型也可以是指针等引用类型，为值类型时内部会自动转换为引用类
// 型（获取可引用的地址），如果有第二个返回值那么它必须是 error 类型。
func IsFuncBeanType(fnType reflect.Type) bool {

	// 必须是函数
	if fnType.Kind() != reflect.Func {
		return false
	}

	// 返回值必须是 1 个或者 2 个
	if fnType.NumOut() < 1 || fnType.NumOut() > 2 {
		return false
	}

	// 如果有第 2 个返回值则它必须是 error 类型
	if fnType.NumOut() == 2 && !fnType.Out(1).Implements(errorType) {
		return false
	}

	return true
}

type ctorBeanFactory struct {
	fnType reflect.Type
	fn     interface{}
	arg    *arg.ArgList
}

func (b *ctorBeanFactory) newValue() reflect.Value {

	// 创建 Bean 的值
	out0 := b.fnType.Out(0)
	v := reflect.New(out0)

	// 引用类型去掉一层指针
	if util.IsRefType(out0.Kind()) {
		v = v.Elem()
	}
	return v
}

func (b *ctorBeanFactory) beanType() reflect.Type {
	return b.newValue().Type() // TODO
}

func (b *ctorBeanFactory) beanClass() string {
	return "constructor bean"
}

// beanStatus Bean 的状态值
type beanStatus int

const (
	BeanStatus_Default   = beanStatus(0) // 默认状态
	BeanStatus_Resolving = beanStatus(1) // 正在决议
	BeanStatus_Resolved  = beanStatus(2) // 已决议
	BeanStatus_Wiring    = beanStatus(3) // 正在注入
	BeanStatus_Wired     = beanStatus(4) // 注入完成
	BeanStatus_Deleted   = beanStatus(5) // 已删除
)

// beanInstance BeanInstance 的抽象接口
type beanInstance interface {
	Bean() interface{}    // 源
	Type() reflect.Type   // 类型
	Value() reflect.Value // 值
	setVal(reflect.Value) //
	TypeName() string     // 原始类型的全限定名

	Name() string        // 返回 Bean 的名称
	BeanId() string      // 返回 Bean 的唯一 ID
	FileLine() string    // 返回 Bean 的注册点
	Description() string // 返回 Bean 的详细描述

	beanFactory() beanFactory
	getStatus() beanStatus         // 返回 Bean 的状态值
	setStatus(status beanStatus)   // 设置 Bean 的状态值
	getDependsOn() []bean.Selector // 返回 Bean 的间接依赖项
	getInit() *runnable            // 返回 Bean 的初始化函数
	getDestroy() *runnable         // 返回 Bean 的销毁函数
	getFile() string               // 返回 Bean 注册点所在文件的名称
	getLine() int                  // 返回 Bean 注册点所在文件的行数
}

type BeanDefinition struct {
	factory beanFactory

	RType    reflect.Type // 类型
	typeName string       // 原始类型的全限定名

	name string // Bean 的名称，请勿直接使用该字段!

	file string // 注册点所在文件
	line int    // 注册点所在行数

	cond      bean.Condition  // 判断条件
	primary   bool            // 是否为主版本
	dependsOn []bean.Selector // 间接依赖项

	init    *runnable // 初始化函数
	destroy *runnable // 销毁函数

	exports map[reflect.Type]struct{} // 严格导出的接口类型
}

func newBeanDefinition(factory beanFactory, file string, line int) *BeanDefinition {
	t := factory.beanType()
	if !util.IsRefType(t.Kind()) {
		panic(errors.New("bean must be ref type"))
	}
	return &BeanDefinition{
		RType:    t,
		typeName: bean.TypeName(t),
		factory:  factory,
		file:     file,
		line:     line,
		exports:  make(map[reflect.Type]struct{}),
	}
}

// Type 返回 Bean 的类型
func (f *BeanDefinition) Type() reflect.Type {
	return f.RType
}

// TypeName 返回 Bean 的原始类型的全限定名
func (f *BeanDefinition) TypeName() string {
	return f.typeName
}

// Name 返回 Bean 的名称
func (f *BeanDefinition) Name() string {
	if f.name == "" {
		// 统一使用类型字符串作为默认名称!
		f.name = f.RType.String()
	}
	return f.name
}

// BeanId 返回 Bean 的唯一 ID
func (f *BeanDefinition) BeanId() string {
	return f.TypeName() + ":" + f.Name()
}

// FileLine 返回 Bean 的注册点
func (f *BeanDefinition) FileLine() string {
	return fmt.Sprintf("%s:%d", f.file, f.line)
}

func (f *BeanDefinition) beanFactory() beanFactory {
	return f.factory
}

// getDependsOn 返回 Bean 的间接依赖项
func (f *BeanDefinition) getDependsOn() []bean.Selector {
	return f.dependsOn
}

// getInit 返回 Bean 的初始化函数
func (f *BeanDefinition) getInit() *runnable {
	return f.init
}

// getDestroy 返回 Bean 的销毁函数
func (f *BeanDefinition) getDestroy() *runnable {
	return f.destroy
}

// getFile 返回 Bean 注册点所在文件的名称
func (f *BeanDefinition) getFile() string {
	return f.file
}

// getLine 返回 Bean 注册点所在文件的行数
func (f *BeanDefinition) getLine() int {
	return f.line
}

// Description 返回 Bean 的详细描述
func (f *BeanDefinition) Description() string {
	return fmt.Sprintf("%s \"%s\" %s", f.factory.beanClass(), f.Name(), f.FileLine())
}

func (f *BeanDefinition) newValue() reflect.Value {
	return f.factory.newValue()
}

// WithName 设置 Bean 的名称
func (f *BeanDefinition) WithName(name string) *BeanDefinition {
	f.name = name
	return f
}

// WithCondition 为 Bean 设置一个 Condition
func (f *BeanDefinition) WithCondition(cond bean.Condition) *BeanDefinition {
	f.cond = cond
	return f
}

// DependsOn 设置 Bean 的间接依赖项
func (f *BeanDefinition) DependsOn(selectors ...bean.Selector) *BeanDefinition {
	f.dependsOn = append(f.dependsOn, selectors...)
	return f
}

// primary 设置 Bean 为主版本
func (f *BeanDefinition) SetPrimary(primary bool) *BeanDefinition {
	f.primary = primary
	return f
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
func (f *BeanDefinition) Init(fn interface{}, args ...arg.Arg) *BeanDefinition {
	if fnType, ok := validLifeCycleFunc(fn, f.Type()); ok {
		f.init = newRunnable(fn, arg.NewArgList(fnType, true, args))
		return f
	}
	panic(errors.New("init should be func(bean) or func(bean)error"))
}

// Destroy 设置 Bean 的销毁函数，args 是销毁函数的一般参数绑定
func (f *BeanDefinition) Destroy(fn interface{}, args ...arg.Arg) *BeanDefinition {
	if fnType, ok := validLifeCycleFunc(fn, f.Type()); ok {
		f.destroy = newRunnable(fn, arg.NewArgList(fnType, true, args))
		return f
	}
	panic(errors.New("destroy should be func(bean) or func(bean)error"))
}

// Export 显式指定 Bean 的导出接口
func (f *BeanDefinition) Export(exports ...bean.TypeOrPtr) *BeanDefinition {
	for _, o := range exports { // 使用 map 进行排重

		var typ reflect.Type
		if t, ok := o.(reflect.Type); ok {
			typ = t
		} else { // 处理 (*error)(nil) 这种导出形式
			typ = util.Indirect(reflect.TypeOf(o))
		}

		if typ.Kind() == reflect.Interface {
			f.exports[typ] = struct{}{}
		} else {
			panic(errors.New("must export interface type"))
		}
	}
	return f
}

// Match 测试 Bean 的类型全限定名和 Bean 的名称是否都匹配
func (f *BeanDefinition) Match(typeName string, beanName string) bool {

	typeIsSame := false
	if typeName == "" || f.TypeName() == typeName {
		typeIsSame = true
	}

	nameIsSame := false
	if beanName == "" || f.Name() == beanName {
		nameIsSame = true
	}

	return typeIsSame && nameIsSame
}

type BeanInstance struct {
	*BeanDefinition

	RValue reflect.Value // 值
	status beanStatus    // Bean 的状态
}

// NewBeanInstance BeanInstance 的构造函数
func NewBeanInstance(factory *BeanDefinition) *BeanInstance {
	return &BeanInstance{
		BeanDefinition: factory,
		RValue:         factory.newValue(),
		status:         BeanStatus_Default,
	}
}

// Bean 返回 Bean 的源
func (d *BeanInstance) Bean() interface{} {
	return d.RValue.Interface()
}

func (d *BeanInstance) setVal(v reflect.Value) {
	d.RValue = v
}

// Value 返回 Bean 的值
func (d *BeanInstance) Value() reflect.Value {
	return d.RValue
}

// getStatus 返回 Bean 的状态值
func (d *BeanInstance) getStatus() beanStatus {
	return d.status
}

// setStatus 设置 Bean 的状态值
func (d *BeanInstance) setStatus(status beanStatus) {
	d.status = status
}

func getFileLine() (file string, line int) {

	// 获取注册点信息
	for i := 2; i < 10; i++ {
		_, file0, line0, _ := runtime.Caller(i)

		// 排除 spring-core 包下面所有的非 test 文件
		if strings.Contains(file0, "/spring-core/") {
			if !strings.HasSuffix(file0, "_test.go") {
				continue
			}
		}

		// 排除 spring-boot 包下面的 spring-boot-singlet.go 文件
		if strings.Contains(file0, "/spring-boot/") {
			if strings.HasSuffix(file0, "spring-boot-singlet.go") {
				continue
			}
		}

		file = file0
		line = line0
		break
	}
	return
}

func valueBean(v reflect.Value, file string, line int) *BeanDefinition {
	if !v.IsValid() || util.IsNil(v) {
		panic(errors.New("bean can't be nil"))
	}
	return newBeanDefinition(&objBeanFactory{v: v}, file, line)
}

// ObjBean 将 Bean 转换为 BeanDefinition 对象
func ObjBean(i interface{}) *BeanDefinition {
	file, line := getFileLine()
	return valueBean(reflect.ValueOf(i), file, line)
}

// CtorBean 将构造函数转换为 BeanDefinition 对象
func CtorBean(fn interface{}, args ...arg.Arg) *BeanDefinition {

	file, line := getFileLine()
	fnType := reflect.TypeOf(fn)

	// 检查 Bean 的注册函数是否合法
	if !IsFuncBeanType(fnType) {
		t1 := "func(...)bean"
		t2 := "func(...)(bean, error)"
		panic(fmt.Errorf("func bean must be %s or %s", t1, t2))
	}

	b := &ctorBeanFactory{
		fnType: fnType,
		fn:     fn,
		arg:    arg.NewArgList(fnType, false, args), // TODO 支持 receiver 构造函数
	}

	return newBeanDefinition(b, file, line)
}
