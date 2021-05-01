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

// toSingletonTag 将 bean.Selector 转换为对应的 singletonTag 。
func toSingletonTag(selector bean.Selector) singletonTag {
	switch s := selector.(type) {
	case string:
		return parseSingletonTag(s)
	case bean.Definition:
		return parseSingletonTag(s.ID())
	case *BeanDefinition:
		return parseSingletonTag(s.ID())
	default:
		return parseSingletonTag(util.TypeName(s) + ":")
	}
}

// singletonTag 单例模式的 tag 分解式，完整形式是 XXX:XXX? 。
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

// parseSingletonTag 解析单例模式的 tag 分解式，完整形式是 XXX:XXX? 。
func parseSingletonTag(str string) (tag singletonTag) {

	if str == "" {
		return
	}

	// 检查字符串结尾是否有可空标记。
	if n := len(str) - 1; str[n] == '?' {
		tag.nullable = true
		str = str[:n]
	}

	// tag 的完整形式，形如 XXX:XXX? 。
	if i := strings.Index(str, ":"); i >= 0 {
		tag.beanName = str[i+1:]
		tag.typeName = str[:i]
		return
	}

	// tag 的简化形式，形如 XXX? 。
	tag.beanName = str
	return
}

// collectionTag 收集模式的 tag 分解式，完整形式是 [XXX:XXX?,*]? 。
type collectionTag struct {
	beanTags []singletonTag
	nullable bool
}

func (tag collectionTag) String() string {
	b := bytes.NewBuffer(nil)
	b.WriteString("[")
	n := len(tag.beanTags)
	for i, t := range tag.beanTags {
		b.WriteString(t.String())
		if i < n-1 {
			b.WriteString(",")
		}
	}
	b.WriteString("]")
	if tag.nullable {
		b.WriteString("?")
	}
	return b.String()
}

// collectionMode 返回是否为收集模式。
func collectionMode(str string) bool {
	return len(str) > 0 && str[0] == '['
}

// parseCollectionTag 解析收集模式的 tag 分解式，完整形式是 [XXX:XXX?,*]? 。
func parseCollectionTag(str string) (tag collectionTag) {

	if str == "" {
		panic(errors.New("invalid collection tag"))
	}

	tag.beanTags = make([]singletonTag, 0)

	// 检查字符串结尾是否有可空标记。
	if n := len(str) - 1; str[n] == '?' {
		tag.nullable = true
		str = str[:n]
	}

	if str[0] != '[' || str[len(str)-1] != ']' {
		panic(errors.New("invalid collection tag"))
	}

	if str = str[1 : len(str)-1]; len(str) > 0 {
		for _, s := range strings.Split(str, ",") {
			tag.beanTags = append(tag.beanTags, parseSingletonTag(s))
		}
	}
	return
}

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

	getFactory() arg.Callable      // 返回 Bean 的工厂函数
	getClass() string              // 返回 Bean 的类型描述
	getFile() string               // 返回 Bean 注册点所在文件
	getLine() int                  // 返回 Bean 注册点所在行数
	getStatus() beanStatus         // 返回 Bean 的状态值
	setStatus(status beanStatus)   // 设置 Bean 的状态值
	getInit() arg.Callable         // 返回 Bean 的初始化函数
	getDestroy() arg.Callable      // 返回 Bean 的销毁函数
	getDependsOn() []bean.Selector // 返回 Bean 的间接依赖项
}

// BeanDefinition 保存 Bean 的各种元数据。
type BeanDefinition struct {

	// 原始类型的全限定名
	typeName string

	v reflect.Value // 值
	t reflect.Type  // 类型
	f arg.Callable  // 工厂函数

	file string // 注册点所在文件
	line int    // 注册点所在行数

	name      string          // 名称
	status    beanStatus      // 状态
	cond      cond.Condition  // 判断条件
	primary   bool            // 是否为主版本
	init      arg.Callable    // 初始化函数
	destroy   arg.Callable    // 销毁函数
	dependsOn []bean.Selector // 间接依赖项

	exports map[reflect.Type]struct{} // 导出的接口
}

// newBeanDefinition BeanDefinition 的构造函数，f 是工厂函数，当 v 为对象 Bean 时 f 为空。
func newBeanDefinition(v reflect.Value, f arg.Callable, file string, line int) *BeanDefinition {
	if t := v.Type(); util.RefType(t.Kind()) {
		return &BeanDefinition{
			t:        t,
			v:        v,
			f:        f,
			typeName: util.TypeName(t),
			status:   Default,
			file:     file,
			line:     line,
			exports:  make(map[reflect.Type]struct{}),
		}
	}
	panic(errors.New("bean must be ref type"))
}

// Type 返回 Bean 的类型。
func (d *BeanDefinition) Type() reflect.Type {
	return d.t
}

// Value 返回 Bean 的值。
func (d *BeanDefinition) Value() reflect.Value {
	return d.v
}

// Interface 返回 Bean 的对象。
func (d *BeanDefinition) Interface() interface{} {
	return d.Value().Interface()
}

// ID 返回 Bean 的 ID 。
func (d *BeanDefinition) ID() string {
	return d.TypeName() + ":" + d.Name()
}

// Name 返回 Bean 的名称。
func (d *BeanDefinition) Name() string {
	// 没有为 Bean 设置名称时使用类型名作为它的名称。
	if d.name == "" {
		d.name = d.t.String()
	}
	return d.name
}

// TypeName 返回 Bean 的原始类型的全限定名。
func (d *BeanDefinition) TypeName() string {
	return d.typeName
}

// FileLine 返回 Bean 的注册点。
func (d *BeanDefinition) FileLine() string {
	return fmt.Sprintf("%s:%d", d.file, d.line)
}

// Description 返回 Bean 的详细描述。
func (d *BeanDefinition) Description() string {
	return fmt.Sprintf("%s name:%q %s", d.getClass(), d.Name(), d.FileLine())
}

// getFactory 返回 Bean 的工厂函数。
func (d *BeanDefinition) getFactory() arg.Callable {
	return d.f
}

// getClass 返回 Bean 的类型描述。
func (d *BeanDefinition) getClass() string {
	if d.f == nil {
		return "object bean"
	}
	return "constructor bean"
}

// getFile 返回 Bean 注册点所在文件。
func (d *BeanDefinition) getFile() string {
	return d.file
}

// getLine 返回 Bean 注册点所在行数。
func (d *BeanDefinition) getLine() int {
	return d.line
}

// getStatus 返回 Bean 的状态值。
func (d *BeanDefinition) getStatus() beanStatus {
	return d.status
}

// setStatus 设置 Bean 的状态值。
func (d *BeanDefinition) setStatus(status beanStatus) {
	d.status = status
}

// getInit 返回 Bean 的初始化函数。
func (d *BeanDefinition) getInit() arg.Callable {
	return d.init
}

// getDestroy 返回 Bean 的销毁函数。
func (d *BeanDefinition) getDestroy() arg.Callable {
	return d.destroy
}

// getDependsOn 返回 Bean 的间接依赖项。
func (d *BeanDefinition) getDependsOn() []bean.Selector {
	return d.dependsOn
}

// Match 测试 Bean 的类型全限定名和 Bean 的名称是否都匹配。
func (d *BeanDefinition) Match(typeName string, beanName string) bool {

	typeIsSame := false
	if typeName == "" || d.TypeName() == typeName {
		typeIsSame = true
	}

	nameIsSame := false
	if beanName == "" || d.Name() == beanName {
		nameIsSame = true
	}

	return typeIsSame && nameIsSame
}

// WithName 设置 Bean 的名称。
func (d *BeanDefinition) WithName(name string) *BeanDefinition {
	d.name = name
	return d
}

// WithCond 设置 Bean 的 Condition。
func (d *BeanDefinition) WithCond(cond cond.Condition) *BeanDefinition {
	d.cond = cond
	return d
}

// DependsOn 设置 Bean 的间接依赖项。
func (d *BeanDefinition) DependsOn(selectors ...bean.Selector) *BeanDefinition {
	d.dependsOn = append(d.dependsOn, selectors...)
	return d
}

// Primary 设置 Bean 为主版本。
func (d *BeanDefinition) Primary(primary bool) *BeanDefinition {
	d.primary = primary
	return d
}

// validLifeCycleFunc 判断是否是合法的用于 Bean 生命周期控制的函数，生命周期函数的要求：
// 至少一个参数，且第一个参数的类型必须是 Bean 的类型，没有返回值或者只能返回 error 类型值。
func validLifeCycleFunc(fnType reflect.Type, beanType reflect.Type) bool {
	ok := util.ReturnNothing(fnType) || util.ReturnOnlyError(fnType)
	return ok && util.FuncType(fnType) && util.WithReceiver(fnType, beanType)
}

// Init 设置 Bean 的初始化函数。
func (d *BeanDefinition) Init(fn interface{}, args ...arg.Arg) *BeanDefinition {
	if validLifeCycleFunc(reflect.TypeOf(fn), d.Type()) {
		d.init = arg.Bind(fn, args, arg.WithReceiver(), arg.Skip(1))
		return d
	}
	panic(errors.New("init should be func(bean) or func(bean)error"))
}

// Destroy 设置 Bean 的销毁函数。
func (d *BeanDefinition) Destroy(fn interface{}, args ...arg.Arg) *BeanDefinition {
	if validLifeCycleFunc(reflect.TypeOf(fn), d.Type()) {
		d.destroy = arg.Bind(fn, args, arg.WithReceiver(), arg.Skip(1))
		return d
	}
	panic(errors.New("destroy should be func(bean) or func(bean)error"))
}

// Export 设置 Bean 的导出接口。
func (d *BeanDefinition) Export(exports ...interface{}) *BeanDefinition {
	for _, o := range exports {

		var typ reflect.Type
		if t, ok := o.(reflect.Type); ok {
			typ = t
		} else { // 处理 (*error)(nil) 这种导出形式
			typ = util.Indirect(reflect.TypeOf(o))
		}

		if typ.Kind() == reflect.Interface {
			d.exports[typ] = struct{}{}
		} else {
			panic(errors.New("should export interface type"))
		}
	}
	return d
}

// NewBean 普通函数注册需要使用 reflect.ValueOf(fn) 这种方式避免和构造函数发生冲突。
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
		skip int
	)

	for skip = 1; skip < 10; skip++ {
		_, f, l, _ := runtime.Caller(skip)
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
		v = reflect.New(out0)

		// 引用类型去掉一层指针
		if util.RefType(out0.Kind()) {
			v = v.Elem()
		}

		ctor := arg.Bind(objOrCtor, ctorArgs, arg.Skip(skip))
		return newBeanDefinition(v, ctor, file, line)
	}

	return newBeanDefinition(v, nil, file, line)
}
