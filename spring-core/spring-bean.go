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
	"reflect"
	"runtime"
	"strings"

	"github.com/go-spring/go-spring-parent/spring-utils"
)

// errorType error 的反射类型
var errorType = reflect.TypeOf((*error)(nil)).Elem()

const (
	valType = 1 // 值类型
	refType = 2 // 引用类型
)

var kindTypes = []uint8{
	0,       // Invalid
	valType, // Bool
	valType, // Int
	valType, // Int8
	valType, // Int16
	valType, // Int32
	valType, // Int64
	valType, // Uint
	valType, // Uint8
	valType, // Uint16
	valType, // Uint32
	valType, // Uint64
	0,       // Uintptr
	valType, // Float32
	valType, // Float64
	valType, // Complex64
	valType, // Complex128
	valType, // Array
	refType, // Chan
	refType, // Func
	refType, // Interface
	refType, // Map
	refType, // Ptr
	refType, // Slice
	valType, // String
	valType, // Struct
	0,       // UnsafePointer
}

// IsRefType 返回是否是引用类型
func IsRefType(k reflect.Kind) bool {
	return kindTypes[k] == refType
}

// IsValueType 返回是否是值类型
func IsValueType(k reflect.Kind) bool {
	return kindTypes[k] == valType
}

// TypeOrPtr 可以是 reflect.Type 对象或者形如 (*error)(nil) 的对象指针。
type TypeOrPtr interface{}

// TypeName 返回原始类型的全限定名，Go 允许不同的路径下存在相同的包，故此有全限定名的需求，
// 形如 "github.com/go-spring/go-spring/spring-core/SpringCore.BeanDefinition"。
func TypeName(typOrPtr TypeOrPtr) string {

	if typOrPtr == nil {
		panic(errors.New("shouldn't be nil"))
	}

	var typ reflect.Type

	switch t := typOrPtr.(type) {
	case reflect.Type:
		typ = t
	default:
		typ = reflect.TypeOf(t)
	}

	for { // 去掉指针和数组的包装，以获得原始类型
		if k := typ.Kind(); k == reflect.Ptr || k == reflect.Slice {
			typ = typ.Elem()
		} else {
			break
		}
	}

	if pkgPath := typ.PkgPath(); pkgPath != "" {
		return pkgPath + "/" + typ.String()
	} else { // 内置类型的路径为空
		return typ.String()
	}
}

// BeanSelector Bean 选择器，可以是 BeanId 字符串，可以是 reflect.Type
// 对象或者形如 (*error)(nil) 的对象指针，还可以是 *BeanDefinition 对象。
type BeanSelector interface{}

// BeanSelectorToString 将 BeanSelector 转换为字符串格式。Bean 选择器
// 字符串和 BeanId 字符串格式一样，但 TypeName、BeanName 中的一个可以为空。
func BeanSelectorToString(selector BeanSelector) string {
	if selector == nil || selector == "" {
		panic(errors.New("selector can't be nil or empty"))
	}
	switch s := selector.(type) {
	case string:
		return s
	case *BeanDefinition:
		return s.BeanId()
	}
	return TypeName(selector) + ":"
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

// ParseSingletonTag 解析单例模式注入 Tag 字符串
func ParseSingletonTag(str string) (tag SingletonTag) {
	if len(str) > 0 {

		// 字符串结尾是否有可空标记
		if str[len(str)-1] == '?' {
			tag.Nullable = true
			str = str[:len(str)-1]
		}

		if ss := strings.Split(str, ":"); len(ss) > 1 { // 完整形式
			tag.TypeName = ss[0]
			tag.BeanName = ss[1]
		} else { // 简化形式
			tag.BeanName = ss[0]
		}
	}
	return
}

// CollectionTag 收集模式注入 Tag 对应的分解形式
type CollectionTag struct {
	Items    []SingletonTag
	Nullable bool
}

func (tag CollectionTag) String() (str string) {
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

// CollectMode 返回是否是收集模式
func CollectMode(str string) bool {
	return len(str) > 0 && str[0] == '['
}

// ParseCollectionTag 解析收集模式注入 Tag 字符串
func ParseCollectionTag(str string) (tag CollectionTag) {
	tag.Items = make([]SingletonTag, 0)

	// 字符串结尾是否有可空标记
	if str[len(str)-1] == '?' {
		tag.Nullable = true
		str = str[:len(str)-1]
	}

	if str[len(str)-1] != ']' {
		panic(errors.New("error collection tag"))
	}

	if str = str[1 : len(str)-1]; len(str) > 0 {
		for _, s := range strings.Split(str, ",") {
			tag.Items = append(tag.Items, ParseSingletonTag(s))
		}
	}
	return
}

// springBean 封装一个 Bean 的源、类型和值，根据注册形式的不同，Bean 分为对象 Bean
// (objectBean)、构造函数 Bean (constructorBean) 和成员方法 Bean (methodBean)。
type springBean interface {
	Bean() interface{}    // 源
	Type() reflect.Type   // 类型
	Value() reflect.Value // 值
	TypeName() string     // 原始类型的全限定名
	beanClass() string    // springBean 具体类型的名称
}

// objectBean 以对象形式注册的 Bean
type objectBean struct {
	rType    reflect.Type  // 类型
	rValue   reflect.Value // 值
	typeName string        // 原始类型的全限定名
}

// newObjectBean objectBean 的构造函数，入参必须是一个引用类型的值。
func newObjectBean(v reflect.Value) *objectBean {
	if t := v.Type(); IsRefType(t.Kind()) {
		return &objectBean{
			rType:    t,
			rValue:   v,
			typeName: TypeName(t),
		}
	} else {
		panic(errors.New("bean must be ref type"))
	}
}

// Bean 返回 Bean 的源
func (b *objectBean) Bean() interface{} {
	return b.rValue.Interface()
}

// Type 返回 Bean 的类型
func (b *objectBean) Type() reflect.Type {
	return b.rType
}

// Value 返回 Bean 的值
func (b *objectBean) Value() reflect.Value {
	return b.rValue
}

// TypeName 返回 Bean 原始类型的全限定名
func (b *objectBean) TypeName() string {
	return b.typeName
}

// beanClass 返回 objectBean 的类型名称
func (b *objectBean) beanClass() string {
	return "object bean"
}

// functionBean 以函数形式（构造函数或者成员方法）注册的 Bean
type functionBean struct {
	objectBean

	stringArg *fnStringBindingArg // 非 Option 参数的绑定
	optionArg *fnOptionBindingArg // Option 参数的绑定
}

// IsFuncBeanType 返回以函数形式注册 Bean 的函数是否合法，一个合法
// 的注册函数需要以下条件：入参可以有任意多个，而且支持 Option 形式和
// 非 Option 形式，返回值只能有一个或者两个，第一个返回值必须是 Bean
// 源，它可以是结构体等值类型也可以是指针等引用类型，为值类型时内部会自
// 动转换为引用类型（获取可引用的地址），第二个返回值必须是 error 类型。
func IsFuncBeanType(fnType reflect.Type) bool {

	// 必须是函数
	if fnType.Kind() != reflect.Func {
		return false
	}

	// 返回值必须是 1 个或者 2 个
	if fnType.NumOut() < 1 || fnType.NumOut() > 2 {
		return false
	}

	// 第 2 个返回值必须是 error 类型
	if fnType.NumOut() == 2 && !fnType.Out(1).Implements(errorType) {
		return false
	}

	return true
}

// newFunctionBean functionBean 的构造函数，当 withReceiver 为 true 时 fnType 对应的函数指针
// 其第一个参数视为接收者，也就是 fnType 对应的成员函数可以形如 Server.Consumer 的形式来表示。
// tags 是 Bean 函数的非 Option 模式参数的绑定值，这使得函数也具有了和结构体 Tag 机制类似的能力。
// tags 的值是结构体 tag 值的简化版，去掉了 tag 的名字，另外所有的 tag 必须同时有或者同时没有序号。
func newFunctionBean(fnType reflect.Type, withReceiver bool, tags []string) functionBean {

	// 检查 Bean 的注册函数是否合法
	if !IsFuncBeanType(fnType) {
		t1 := "func(...)bean"
		t2 := "func(...)(bean, error)"
		panic(fmt.Errorf("func bean must be %s or %s", t1, t2))
	}

	// 创建 Bean 的值
	out := fnType.Out(0)
	v := reflect.New(out)

	// 引用类型去掉一层指针
	if IsRefType(out.Kind()) {
		v = v.Elem()
	}

	// 获取 Bean 的类型
	t := v.Type()

	return functionBean{
		objectBean: objectBean{
			rType:    t,
			rValue:   v,
			typeName: TypeName(t),
		},
		stringArg: newFnStringBindingArg(fnType, withReceiver, tags),
	}
}

// constructorBean 以构造函数形式注册的 Bean
type constructorBean struct {
	functionBean

	fn interface{} // 构造函数的函数指针
}

// newConstructorBean constructorBean 的构造函数，所有 tag 必须同时有或者同时没有序号。
func newConstructorBean(fn interface{}, tags []string) *constructorBean {
	return &constructorBean{
		functionBean: newFunctionBean(reflect.TypeOf(fn), false, tags),
		fn:           fn,
	}
}

// beanClass 返回 constructorBean 的类型名称
func (b *constructorBean) beanClass() string {
	return "constructor bean"
}

// methodBean 以成员方法形式注册的 Bean
type methodBean struct {
	functionBean

	parent *BeanDefinition // 父对象的定义
	method string          // 成员方法名称
}

// newMethodBean methodBean 的构造函数，所有 tag 必须同时有或者同时没有序号。
func newMethodBean(parent *BeanDefinition, methodName string, tags []string) *methodBean {

	parentType := parent.Type()
	method, ok := parentType.MethodByName(methodName)
	if !ok {
		panic(fmt.Errorf("can't find method:%s on type:%s", methodName, TypeName(parentType)))
	}

	withReceiver := false

	// 接口的函数指针不包含接受者类型
	if parentType.Kind() != reflect.Interface {
		withReceiver = true
	}

	return &methodBean{
		functionBean: newFunctionBean(method.Type, withReceiver, tags),
		parent:       parent,
		method:       methodName,
	}
}

// beanClass 返回 methodBean 的类型名称
func (b *methodBean) beanClass() string {
	return "method bean"
}

// fakeMethodBean 成员方法 Bean 不是在注册时就立即生成 methodBean
// 对象，而是在所有 Bean 注册完成并且自动注入开始后才开始生成，在此之前
// 使用 fakeMethodBean 对象进行占位。
type fakeMethodBean struct {
	// 父 Bean 的 BeanId
	parent string

	// 成员方法名称
	method string

	// 成员方法标签
	tags []string
}

// newFakeMethodBean fakeMethodBean 的构造函数，parent 是父 Bean 的 BeanId。
func newFakeMethodBean(parent string, method string, tags []string) *fakeMethodBean {
	return &fakeMethodBean{
		parent: parent,
		method: method,
		tags:   tags,
	}
}

func (b *fakeMethodBean) Bean() interface{} {
	panic(errors.New("shouldn't call this method"))
}

func (b *fakeMethodBean) Type() reflect.Type {
	panic(errors.New("shouldn't call this method"))
}

func (b *fakeMethodBean) Value() reflect.Value {
	panic(errors.New("shouldn't call this method"))
}

func (b *fakeMethodBean) TypeName() string {
	panic(errors.New("shouldn't call this method"))
}

// beanClass 返回 SpringBean 的实现类型
func (b *fakeMethodBean) beanClass() string {
	return "fake method bean"
}

// beanStatus Bean 的状态值
type beanStatus int

const (
	beanStatus_Default   = beanStatus(0) // 默认状态
	beanStatus_Resolving = beanStatus(1) // 正在决议
	beanStatus_Resolved  = beanStatus(2) // 已决议
	beanStatus_Wiring    = beanStatus(3) // 正在绑定
	beanStatus_Wired     = beanStatus(4) // 绑定完成
	beanStatus_Deleted   = beanStatus(5) // 已删除
)

// beanDefinition BeanDefinition 的抽象接口
type beanDefinition interface {
	Bean() interface{}    // 源
	Type() reflect.Type   // 类型
	Value() reflect.Value // 值
	TypeName() string     // 原始类型的全限定名

	Name() string        // 返回 Bean 的名称
	BeanId() string      // 返回 Bean 的 BeanId
	FileLine() string    // 返回 Bean 的注册点
	Description() string // 返回 Bean 的详细描述

	springBean() springBean      // 返回 SpringBean 对象
	getStatus() beanStatus       // 返回 Bean 的状态值
	setStatus(status beanStatus) // 设置 Bean 的状态值
	getDependsOn() []string      // 返回 Bean 的非直接依赖项
	getInit() *runnable          // 返回 Bean 的初始化函数
	getDestroy() *runnable       // 返回 Bean 的销毁函数
	getFile() string             // 返回 Bean 注册点所在文件的名称
	getLine() int                // 返回 Bean 注册点所在文件的行数
}

// BeanDefinition Bean 的详细定义
type BeanDefinition struct {
	bean   springBean
	name   string     // Bean 的名称
	status beanStatus // Bean 的状态

	file string // 注册点所在文件
	line int    // 注册点所在行数

	cond      *Conditional // 判断条件
	primary   bool         // 是否为主版本
	dependsOn []string     // 间接依赖

	init    *runnable // 初始化时的回调
	destroy *runnable // 销毁时的回调

	exports map[reflect.Type]struct{} // 导出的接口类型
}

// newBeanDefinition BeanDefinition 的构造函数
func newBeanDefinition(name string, bean springBean) *BeanDefinition {

	var (
		file string
		line int
	)

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

	// 如果不是成员方法 Bean 当名称为空时生成默认的 Bean 名称
	if _, ok := bean.(*fakeMethodBean); !ok && name == "" {
		name = bean.Type().String()
	}

	return &BeanDefinition{
		bean:    bean,
		name:    name,
		status:  beanStatus_Default,
		file:    file,
		line:    line,
		cond:    NewConditional(),
		exports: make(map[reflect.Type]struct{}),
	}
}

// Bean 返回 Bean 的源
func (d *BeanDefinition) Bean() interface{} {
	return d.bean.Bean()
}

// Type 返回 Bean 的类型
func (d *BeanDefinition) Type() reflect.Type {
	return d.bean.Type()
}

// Value 返回 Bean 的值
func (d *BeanDefinition) Value() reflect.Value {
	return d.bean.Value()
}

// TypeName 返回 Bean 的原始类型的全限定名
func (d *BeanDefinition) TypeName() string {
	return d.bean.TypeName()
}

// Name 返回 Bean 的名称
func (d *BeanDefinition) Name() string {
	return d.name
}

// BeanId 返回 Bean 的 BeanId
func (d *BeanDefinition) BeanId() string {
	return fmt.Sprintf("%s:%s", d.TypeName(), d.name)
}

// FileLine 返回 Bean 注册点的文件和行号
func (d *BeanDefinition) FileLine() string {
	return fmt.Sprintf("%s:%d", d.file, d.line)
}

// springBean 返回 SpringBean 对象
func (d *BeanDefinition) springBean() springBean {
	return d.bean
}

// getStatus 返回 Bean 的状态值
func (d *BeanDefinition) getStatus() beanStatus {
	return d.status
}

// setStatus 设置 Bean 的状态值
func (d *BeanDefinition) setStatus(status beanStatus) {
	d.status = status
}

// getDependsOn 返回 Bean 的间接依赖
func (d *BeanDefinition) getDependsOn() []string {
	return d.dependsOn
}

// getInit 返回 Bean 的初始化函数
func (d *BeanDefinition) getInit() *runnable {
	return d.init
}

// getDestroy 返回 Bean 的销毁函数
func (d *BeanDefinition) getDestroy() *runnable {
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

// Description 返回 Bean 的详细描述
func (d *BeanDefinition) Description() string {
	return fmt.Sprintf("%s \"%s\" %s", d.bean.beanClass(), d.name, d.FileLine())
}

// Match 测试 Bean 的类型全限定名和 Bean 的名称是否都匹配
func (d *BeanDefinition) Match(typeName string, beanName string) bool {

	typeIsSame := false
	if typeName == "" || d.TypeName() == typeName {
		typeIsSame = true
	}

	nameIsSame := false
	if beanName == "" || d.name == beanName {
		nameIsSame = true
	}

	return typeIsSame && nameIsSame
}

// Or c=a||b
func (d *BeanDefinition) Or() *BeanDefinition {
	d.cond.Or()
	return d
}

// And c=a&&b
func (d *BeanDefinition) And() *BeanDefinition {
	d.cond.And()
	return d
}

// ConditionOn 为 Bean 设置一个 Condition
func (d *BeanDefinition) ConditionOn(cond Condition) *BeanDefinition {
	d.cond.OnCondition(cond)
	return d
}

// ConditionNot 为 Bean 设置一个取反的 Condition
func (d *BeanDefinition) ConditionNot(cond Condition) *BeanDefinition {
	d.cond.OnConditionNot(cond)
	return d
}

// ConditionOnProperty 为 Bean 设置一个 PropertyCondition
func (d *BeanDefinition) ConditionOnProperty(name string) *BeanDefinition {
	d.cond.OnProperty(name)
	return d
}

// ConditionOnMissingProperty 为 Bean 设置一个 MissingPropertyCondition
func (d *BeanDefinition) ConditionOnMissingProperty(name string) *BeanDefinition {
	d.cond.OnMissingProperty(name)
	return d
}

// ConditionOnPropertyValue 为 Bean 设置一个 PropertyValueCondition
func (d *BeanDefinition) ConditionOnPropertyValue(name string, havingValue interface{}) *BeanDefinition {
	d.cond.OnPropertyValue(name, havingValue)
	return d
}

// ConditionOnBean 为 Bean 设置一个 BeanCondition
func (d *BeanDefinition) ConditionOnBean(selector BeanSelector) *BeanDefinition {
	d.cond.OnBean(selector)
	return d
}

// ConditionOnMissingBean 为 Bean 设置一个 MissingBeanCondition
func (d *BeanDefinition) ConditionOnMissingBean(selector BeanSelector) *BeanDefinition {
	d.cond.OnMissingBean(selector)
	return d
}

// ConditionOnExpression 为 Bean 设置一个 ExpressionCondition
func (d *BeanDefinition) ConditionOnExpression(expression string) *BeanDefinition {
	d.cond.OnExpression(expression)
	return d
}

// ConditionOnMatches 为 Bean 设置一个 FunctionCondition
func (d *BeanDefinition) ConditionOnMatches(fn ConditionFunc) *BeanDefinition {
	d.cond.OnMatches(fn)
	return d
}

// ConditionOnProfile 为 Bean 设置一个 ProfileCondition
func (d *BeanDefinition) ConditionOnProfile(profile string) *BeanDefinition {
	d.cond.OnProfile(profile)
	return d
}

// checkCondition 检查 Condition 的执行结果，成功返回 true，失败返回 false
func (d *BeanDefinition) checkCondition(ctx SpringContext) bool {
	return d.cond.Matches(ctx)
}

// Options 设置 Option 模式函数的参数绑定
func (d *BeanDefinition) Options(options ...*optionArg) *BeanDefinition {
	arg := &fnOptionBindingArg{options}
	switch bean := d.bean.(type) {
	case *constructorBean:
		bean.optionArg = arg
	case *methodBean:
		bean.optionArg = arg
	default:
		panic(errors.New("error call"))
	}
	return d
}

// DependsOn 设置 Bean 的间接依赖
func (d *BeanDefinition) DependsOn(selectors ...BeanSelector) *BeanDefinition {
	for _, s := range selectors {
		d.dependsOn = append(d.dependsOn, BeanSelectorToString(s))
	}
	return d
}

// Primary 设置 Bean 的优先级
func (d *BeanDefinition) Primary(primary bool) *BeanDefinition {
	d.primary = primary
	return d
}

// validLifeCycleFunc 判断是否是合法的用于 Bean 生命周期控制的回调函数，生命周期函数的要求：
// 有至少一个参数，且第一个参数的类型必须是 Bean 的类型，没有返回值或者只能返回 error 类型值。
func validLifeCycleFunc(fn interface{}, beanType reflect.Type) (reflect.Type, bool) {
	fnType := reflect.TypeOf(fn)

	// 必须是函数
	if fnType.Kind() != reflect.Func {
		return nil, false
	}

	// 有至少一个输入参数
	if fnType.NumIn() < 1 {
		return nil, false
	}

	// 第一个入参的类型必须是 Bean 的类型
	if fnType.In(0) != beanType {
		return nil, false
	}

	// 无返回值，或者只返回 error
	if numOut := fnType.NumOut(); numOut > 1 {
		return nil, false
	} else if numOut == 1 {
		if out := fnType.Out(0); out != errorType {
			return nil, false
		}
	}

	return fnType, true
}

// Init 设置 Bean 初始化时的回调
func (d *BeanDefinition) Init(fn interface{}, tags ...string) *BeanDefinition {

	fnType, ok := validLifeCycleFunc(fn, d.Type())
	if !ok {
		panic(errors.New("init should be func(bean) or func(bean)error"))
	}

	d.init = &runnable{
		fn:           fn,
		withReceiver: true,
		receiver:     d.Value(),
		stringArg:    newFnStringBindingArg(fnType, true, tags),
	}

	return d
}

// Destroy 设置 Bean 销毁时的回调
func (d *BeanDefinition) Destroy(fn interface{}, tags ...string) *BeanDefinition {

	fnType, ok := validLifeCycleFunc(fn, d.Type())
	if !ok {
		panic(errors.New("destroy should be func(bean) or func(bean)error"))
	}

	d.destroy = &runnable{
		fn:           fn,
		withReceiver: true,
		receiver:     d.Value(),
		stringArg:    newFnStringBindingArg(fnType, true, tags),
	}

	return d
}

// Export 显示指定 Bean 的导出接口
func (d *BeanDefinition) Export(exports ...TypeOrPtr) *BeanDefinition {
	return d.export(exports)
}

func (d *BeanDefinition) export(exports []TypeOrPtr) *BeanDefinition {
	for _, o := range exports { // 使用 map 进行排重

		var typ reflect.Type
		if t, ok := o.(reflect.Type); ok {
			typ = t
		} else {
			typ = SpringUtils.Indirect(reflect.TypeOf(o))
		}

		if typ.Kind() == reflect.Interface {
			d.exports[typ] = struct{}{}
		} else {
			panic(errors.New("must export interface type"))
		}
	}
	return d
}

// ToBeanDefinition 将 Bean 源转换为 BeanDefinition 对象
func ToBeanDefinition(name string, i interface{}) *BeanDefinition {
	return ValueToBeanDefinition(name, reflect.ValueOf(i))
}

// ValueToBeanDefinition 将 Value 转换为 BeanDefinition 对象
func ValueToBeanDefinition(name string, v reflect.Value) *BeanDefinition {
	if !v.IsValid() || SpringUtils.IsNil(v) {
		panic(errors.New("bean can't be nil"))
	}
	return newBeanDefinition(name, newObjectBean(v))
}

// FnToBeanDefinition 将构造函数转换为 BeanDefinition 对象
func FnToBeanDefinition(name string, fn interface{}, tags ...string) *BeanDefinition {

	// 具名函数取函数名作为默认名称，匿名函数还是使用 Bean 的类型名称
	// 具名函数形如: .../go-spring/spring-core_test.NewManager
	// 匿名函数形如: .../go-spring/spring-core_test.TestNestValueField.func1.1
	if name == "" {
		fnPtr := reflect.ValueOf(fn).Pointer()
		fnInfo := runtime.FuncForPC(fnPtr)
		s := strings.Split(fnInfo.Name(), "/")
		ss := strings.Split(s[len(s)-1], ".")
		if len(ss) == 2 { // 判断是具名函数
			name = "@" + ss[1]
		}
	}

	return newBeanDefinition(name, newConstructorBean(fn, tags))
}

// MethodToBeanDefinition 将成员方法转换为 BeanDefinition 对象
func MethodToBeanDefinition(name string, parent string, methodName string, tags ...string) *BeanDefinition {
	if name == "" { // 取函数名作为默认名称
		name = "@" + methodName
	}
	return newBeanDefinition(name, newFakeMethodBean(parent, methodName, tags))
}

// ValidBeanValue 返回是否是合法的 Bean 值及其类型
func ValidBeanValue(v reflect.Value) (reflect.Type, bool) {
	if v.IsValid() {
		if beanType := v.Type(); IsRefType(beanType.Kind()) {
			return beanType, true
		}
	}
	return nil, false
}

// fieldBeanDefinition 具有字段名的 BeanDefinition 实现
type fieldBeanDefinition struct {
	*BeanDefinition
	field string // 字段名称
}

// Description 返回 Bean 的详细描述 TODO 需要完善
func (d *fieldBeanDefinition) Description() string {
	return fmt.Sprintf("%s field: %s %s", d.bean.beanClass(), d.field, d.FileLine())
}

// delegateBeanDefinition 代理功能的 BeanDefinition 实现 TODO 不是很明确
type delegateBeanDefinition struct {
	*BeanDefinition
	delegate beanDefinition // 代理项
}

// Description 返回 Bean 的详细描述
func (d *delegateBeanDefinition) Description() string {
	return fmt.Sprintf("%s value %s", d.delegate.springBean().beanClass(), d.delegate.FileLine())
}
