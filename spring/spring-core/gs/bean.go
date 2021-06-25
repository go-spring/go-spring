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
	"math"
	"reflect"
	"runtime"
	"strings"

	"github.com/go-spring/spring-core/arg"
	"github.com/go-spring/spring-core/bean"
	"github.com/go-spring/spring-core/cond"
	"github.com/go-spring/spring-core/util"
)

// wireTag 单例模式的 tag 分解式，完整形式是 XXX:XXX? 。
type wireTag struct {
	typeName string
	beanName string
	nullable bool
}

// parseWireTag 解析单例模式的 tag 分解式，完整形式是 XXX:XXX? 。
func parseWireTag(str string) (tag wireTag) {

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

func (tag wireTag) String() string {
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

// toWireTag 将 bean.Selector 转换为对应的 wireTag 。
func toWireTag(selector bean.Selector) wireTag {
	switch s := selector.(type) {
	case string:
		return parseWireTag(s)
	case bean.Definition:
		return parseWireTag(s.ID())
	case *BeanDefinition:
		return parseWireTag(s.ID())
	default:
		return parseWireTag(util.TypeName(s) + ":")
	}
}

const (
	HighestOrder = math.MinInt32
	LowestOrder  = math.MaxInt32
)

type beanStatus int

const (
	Default   = beanStatus(0) // 默认状态
	Resolving = beanStatus(1) // 正在决议
	Resolved  = beanStatus(2) // 已决议
	Wiring    = beanStatus(3) // 正在注入
	Wired     = beanStatus(4) // 注入完成
	Deleted   = beanStatus(5) // 已删除
)

// BeanDefinition 保存 Bean 的各种元数据。
type BeanDefinition struct {

	// 原始类型的全限定名
	typeName string

	v reflect.Value // 值
	t reflect.Type  // 类型
	f *arg.Callable // 工厂函数

	file string // 注册点所在文件
	line int    // 注册点所在行数

	name      string          // 名称
	status    beanStatus      // 状态
	cond      cond.Condition  // 判断条件
	primary   bool            // 是否为主版本
	order     int             // 收集时的顺序
	init      interface{}     // 初始化函数
	destroy   interface{}     // 销毁函数
	dependsOn []bean.Selector // 间接依赖项

	exports map[reflect.Type]struct{} // 导出的接口
}

// newBeanDefinition BeanDefinition 的构造函数，f 是工厂函数，当 v 为对象 Bean 时 f 为空。
func newBeanDefinition(v reflect.Value, f *arg.Callable, file string, line int) *BeanDefinition {

	t := v.Type()
	if !util.IsBeanType(t) {
		panic(errors.New("bean must be ref type"))
	}

	if t.Kind() == reflect.Ptr && !util.IsValueType(t.Elem()) {
		panic(errors.New("bean should be *val but not *ref"))
	}

	return &BeanDefinition{
		t:        t,
		v:        v,
		f:        f,
		name:     t.String(),
		typeName: util.TypeName(t),
		status:   Default,
		order:    LowestOrder,
		file:     file,
		line:     line,
		exports:  make(map[reflect.Type]struct{}),
	}
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
	return d.v.Interface()
}

// ID 返回 Bean 的 ID 。
func (d *BeanDefinition) ID() string {
	return d.typeName + ":" + d.name
}

// Name 返回 Bean 的名称。
func (d *BeanDefinition) Name() string {
	return d.name
}

// TypeName 返回 Bean 的原始类型的全限定名。
func (d *BeanDefinition) TypeName() string {
	return d.typeName
}

// Wired 返回 Bean 是否注入完成。
func (d *BeanDefinition) Wired() bool {
	return d.status == Wired
}

// FileLine 返回 Bean 的注册点。
func (d *BeanDefinition) FileLine() string {
	return fmt.Sprintf("%s:%d", d.file, d.line)
}

// String 返回 Bean 的描述。
func (d *BeanDefinition) String() string {
	return fmt.Sprintf("%s name:%q %s", d.getClass(), d.name, d.FileLine())
}

// getClass 返回 Bean 的类型描述。
func (d *BeanDefinition) getClass() string {
	if d.f == nil {
		return "object bean"
	}
	return "constructor bean"
}

// Match 测试 Bean 的类型全限定名和 Bean 的名称是否都匹配。
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

// Order 设置 Bean 的 order ，值越小顺序越靠前(优先级越高)。
func (d *BeanDefinition) Order(order int) *BeanDefinition {
	d.order = order
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
	return ok && util.IsFuncType(fnType) && util.HasReceiver(fnType, beanType)
}

// Init 设置 Bean 的初始化函数。
func (d *BeanDefinition) Init(fn interface{}) *BeanDefinition {
	if validLifeCycleFunc(reflect.TypeOf(fn), d.Type()) {
		d.init = fn
		return d
	}
	panic(errors.New("init should be func(bean) or func(bean)error"))
}

// Destroy 设置 Bean 的销毁函数。
func (d *BeanDefinition) Destroy(fn interface{}) *BeanDefinition {
	if validLifeCycleFunc(reflect.TypeOf(fn), d.Type()) {
		d.destroy = fn
		return d
	}
	panic(errors.New("destroy should be func(bean) or func(bean)error"))
}

func (d *BeanDefinition) export(exports ...interface{}) error {
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
			return errors.New("should export interface type")
		}

		// resolve bean 的时候才判断是否实现了接口。
	}
	return nil
}

// Export 设置 Bean 的导出接口。
func (d *BeanDefinition) Export(exports ...interface{}) *BeanDefinition {
	err := d.export(exports...)
	util.Panic(err).When(err != nil)
	return d
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
	_, file, line, _ := runtime.Caller(skip)

	// 以 reflect.ValueOf(fn) 形式注册的函数被视为函数对象 bean 。
	if t := v.Type(); !fromValue && t.Kind() == reflect.Func {

		if !util.IsConstructor(t) {
			t1 := "func(...)bean"
			t2 := "func(...)(bean, error)"
			panic(fmt.Errorf("constructor should be %s or %s", t1, t2))
		}

		// 创建 Bean 的值
		out0 := t.Out(0)
		v = reflect.New(out0)

		// 引用类型去掉指针，值类型则刚刚好。
		if util.IsBeanType(out0) {
			v = v.Elem()
		}

		f := arg.Bind(objOrCtor, ctorArgs, skip)
		return newBeanDefinition(v, f, file, line)
	}

	return newBeanDefinition(v, nil, file, line)
}
