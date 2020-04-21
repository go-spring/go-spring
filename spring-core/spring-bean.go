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
)

const (
	valType = 1 // 值类型
	refType = 2 // 引用类型
)

var kindType = []uint8{
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
	return kindType[k] == refType
}

// IsValueType 返回是否是值类型
func IsValueType(k reflect.Kind) bool {
	return kindType[k] == valType
}

// TypeName 返回原始类型的全限定名，golang 允许不同的路径下存在相同的包，故此有全限定名的需求。
// 形如 "github.com/go-spring/go-spring/spring-core/SpringCore.DefaultSpringContext"
func TypeName(t reflect.Type) string {

	if t == nil {
		panic(errors.New("type shouldn't be nil"))
	}

	// Map 的全限定名太复杂，不予处理，而且 Map 作为注入对象要三思而后行！
	for {
		if k := t.Kind(); k != reflect.Ptr && k != reflect.Slice {
			break
		} else {
			t = t.Elem()
		}
	}

	if pkgPath := t.PkgPath(); pkgPath != "" { // 内置类型的路径为空
		return pkgPath + "/" + t.String()
	} else {
		return t.String()
	}
}

// errorType error 的类型
var errorType = reflect.TypeOf((*error)(nil)).Elem()

// SpringBean Bean 源接口
type SpringBean interface {
	Bean() interface{}    // 源
	Type() reflect.Type   // 类型
	Value() reflect.Value // 值
	TypeName() string     // 原始类型的全限定名
	beanClass() string    // SpringBean 的实现类型
}

// objectBean 原始 Bean 源
type objectBean struct {
	rType    reflect.Type  // 类型
	rValue   reflect.Value // 值
	typeName string        // 原始类型的全限定名
}

// newObjectBean objectBean 的构造函数
func newObjectBean(v reflect.Value) *objectBean {
	if t := v.Type(); !IsRefType(t.Kind()) {
		panic(errors.New("bean must be ref type"))
	} else {
		return &objectBean{
			rType:    t,
			typeName: TypeName(t),
			rValue:   v,
		}
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

// TypeName 返回 Bean 的原始类型的全限定名
func (b *objectBean) TypeName() string {
	return b.typeName
}

// beanClass 返回 SpringBean 的实现类型
func (b *objectBean) beanClass() string {
	return "object bean"
}

// functionBean 函数定义的 Bean 源
type functionBean struct {
	objectBean

	stringArg *fnStringBindingArg // 普通参数绑定
	optionArg *fnOptionBindingArg // Option 绑定
}

// IsFuncBeanType 返回是否是合法的函数 Bean 类型
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
	if fnType.NumOut() == 2 {
		if !fnType.Out(1).Implements(errorType) {
			return false
		}
	}

	return true
}

// newFunctionBean functionBean 的构造函数
func newFunctionBean(fnType reflect.Type, withReceiver bool, tags []string) functionBean {

	// 检查是否是合法的函数 Bean 类型
	if ok := IsFuncBeanType(fnType); !ok {
		t1 := "func(...)bean"
		t2 := "func(...)(bean, error)"
		panic(fmt.Errorf("func bean must be %s or %s", t1, t2))
	}

	t := fnType.Out(0)
	v := reflect.New(t)

	// 引用类型需要解一层指针
	if IsRefType(t.Kind()) {
		v = v.Elem()
	}

	// 然后需要重新确定类型
	t = v.Type()

	return functionBean{
		objectBean: objectBean{
			rType:    t,
			typeName: TypeName(t),
			rValue:   v,
		},
		stringArg: newFnStringBindingArg(fnType, withReceiver, tags),
	}
}

// constructorBean 构造函数定义的 Bean 源
type constructorBean struct {
	functionBean

	fn interface{} // 构造函数
}

// newConstructorBean constructorBean 的构造函数，所有 tag 都必须同时有或者同时没有序号。
func newConstructorBean(fn interface{}, tags ...string) *constructorBean {
	return &constructorBean{
		functionBean: newFunctionBean(reflect.TypeOf(fn), false, tags),
		fn:           fn,
	}
}

// beanClass 返回 SpringBean 的实现类型
func (b *constructorBean) beanClass() string {
	return "constructor bean"
}

// methodBean 成员方法定义的 Bean 源
type methodBean struct {
	functionBean

	parent *BeanDefinition // 父对象的定义
	method string          // 成员方法名称
}

// newMethodBean methodBean 的构造函数，所有 tag 都必须同时有或者同时没有序号。
func newMethodBean(parent *BeanDefinition, method string, tags ...string) *methodBean {
	var fnType reflect.Type

	t := parent.Type()
	if m, ok := t.MethodByName(method); ok {
		fnType = m.Type
	} else {
		panic(fmt.Errorf("can't find method: %s", method))
	}

	withReceiver := false
	if t.Kind() != reflect.Interface {
		withReceiver = true
	}

	return &methodBean{
		functionBean: newFunctionBean(fnType, withReceiver, tags),
		parent:       parent,
		method:       method,
	}
}

// beanClass 返回 SpringBean 的实现类型
func (b *methodBean) beanClass() string {
	return "method bean"
}

// fakeMethodBean 延迟创建的 Method Bean
type fakeMethodBean struct {
	// parent 选择器:
	// *BeanDefinition 表示直接使用 parent 对象;
	// string 类型值表示根据 BeanId 查询 parent 对象;
	// (Type)(nil) 类型值表示根据类型查询 parent 对象。
	selector interface{}

	// 成员方法名称
	method string

	// 成员方法标签
	tags []string
}

// newFakeMethodBean fakeMethodBean 的构造函数
func newFakeMethodBean(selector interface{}, method string, tags ...string) *fakeMethodBean {
	return &fakeMethodBean{
		selector: selector,
		method:   method,
		tags:     tags,
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

// IBeanDefinition 对 BeanDefinition 的抽象接口
type IBeanDefinition interface {
	Bean() interface{}    // 源
	Type() reflect.Type   // 类型
	Value() reflect.Value // 值
	TypeName() string     // 原始类型的全限定名

	Name() string        // 返回 Bean 的名称
	BeanId() string      // 返回 Bean 的 BeanId
	Caller() string      // 返回 Bean 的注册点
	Description() string // 返回 Bean 的详细描述

	springBean() SpringBean      // 返回 SpringBean 对象
	getStatus() beanStatus       // 返回 Bean 的状态值
	setStatus(status beanStatus) // 设置 Bean 的状态值
	getDependsOn() []interface{} // 返回 Bean 的非直接依赖项
	getInit() *runnable          // 返回 Bean 的初始化函数
	getDestroy() *runnable       // 返回 Bean 的销毁函数
	getFile() string             // 返回 Bean 注册点所在文件的名称
	getLine() int                // 返回 Bean 注册点所在文件的行数
}

// BeanDefinition Bean 的详细定义
type BeanDefinition struct {
	bean   SpringBean // 源
	name   string     // 名称
	status beanStatus // 状态

	file string // 注册点所在文件
	line int    // 注册点所在行数

	cond      *Conditional  // 判断条件
	primary   bool          // 主版本
	dependsOn []interface{} // 非直接依赖

	init    *runnable // 初始化的回调
	destroy *runnable // 销毁时的回调

	exports    []reflect.Type // 导出接口类型
	autoExport bool           // 自动导出接口
}

// newBeanDefinition BeanDefinition 的构造函数
func newBeanDefinition(name string, bean SpringBean) *BeanDefinition {

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

	if _, ok := bean.(*fakeMethodBean); !ok {
		if name == "" { // 生成默认名称
			name = bean.Type().String()
		}
	}

	return &BeanDefinition{
		bean:       bean,
		name:       name,
		status:     beanStatus_Default,
		file:       file,
		line:       line,
		cond:       NewConditional(),
		autoExport: true,
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

// Caller 返回 Bean 的注册点
func (d *BeanDefinition) Caller() string {
	return fmt.Sprintf("%s:%d", d.file, d.line)
}

// springBean 返回 SpringBean 对象
func (d *BeanDefinition) springBean() SpringBean {
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

// getDependsOn 返回 Bean 的非直接依赖项
func (d *BeanDefinition) getDependsOn() []interface{} {
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
	return fmt.Sprintf("%s \"%s\" %s", d.bean.beanClass(), d.name, d.Caller())
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
func (d *BeanDefinition) ConditionOnBean(selector interface{}) *BeanDefinition {
	d.cond.OnBean(selector)
	return d
}

// ConditionOnMissingBean 为 Bean 设置一个 MissingBeanCondition
func (d *BeanDefinition) ConditionOnMissingBean(selector interface{}) *BeanDefinition {
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

// Matches 成功返回 true，失败返回 false
func (d *BeanDefinition) Matches(ctx SpringContext) bool {
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
		panic(errors.New("只有 func Bean 才能调用此方法"))
	}
	return d
}

// DependsOn 设置 Bean 的非直接依赖
func (d *BeanDefinition) DependsOn(selector ...interface{}) *BeanDefinition {
	d.dependsOn = append(d.dependsOn, selector...)
	return d
}

// Primary 设置 Bean 的优先级
func (d *BeanDefinition) Primary(primary bool) *BeanDefinition {
	d.primary = primary
	return d
}

// validLifeCycleFunc 判断是否是合法的用于 Bean 生命周期控制的回调函数
func validLifeCycleFunc(fn interface{}, beanType reflect.Type) (reflect.Type, bool) {
	fnType := reflect.TypeOf(fn)

	// 必须是函数
	if fnType.Kind() != reflect.Func {
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

	// 有至少一个输入参数
	if fnType.NumIn() < 1 {
		return nil, false
	}

	// 第一个入参必须是 Bean 的类型 TODO 兼容类型也可以？
	if fnType.In(0) != beanType {
		return nil, false
	}

	return fnType, true
}

// Init 设置 Bean 初始化的回调
func (d *BeanDefinition) Init(fn interface{}, tags ...string) *BeanDefinition {

	fnType, ok := validLifeCycleFunc(fn, d.Type())
	if !ok {
		panic(errors.New("init should be func(bean) or func(bean)error"))
	}

	d.init = &runnable{
		fn:           fn,
		stringArg:    newFnStringBindingArg(fnType, true, tags),
		withReceiver: true,
		receiver:     d.Value(),
	}

	return d
}

// Destroy 设置 Bean 销毁时的回调
func (d *BeanDefinition) Destroy(fn interface{}, tags ...string) *BeanDefinition {

	fnType, ok := validLifeCycleFunc(fn, d.Type())
	if !ok {
		panic(errors.New("destroy should be func(bean) or func(bean)error")) //TODO
	}

	d.destroy = &runnable{
		fn:           fn,
		stringArg:    newFnStringBindingArg(fnType, true, tags),
		withReceiver: true,
		receiver:     d.Value(),
	}

	return d
}

// AutoExport 设置是否自动导出接口
func (d *BeanDefinition) AutoExport(enable bool) *BeanDefinition {
	d.autoExport = enable
	return d
}

// Export 指定 Bean 的导出接口
func (d *BeanDefinition) Export(exports ...interface{}) *BeanDefinition {
	return d.export(exports...)
}

// Deprecated: Use "Export" instead.
func (d *BeanDefinition) AsInterface(exports ...interface{}) *BeanDefinition {
	return d.export(exports...)
}

func (d *BeanDefinition) export(exports ...interface{}) *BeanDefinition {
	for _, o := range exports {
		t := reflect.TypeOf(o)
		if t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Interface {
			d.exports = append(d.exports, t.Elem())
		} else {
			panic(errors.New("must export interface type"))
		}
	}
	return d
}

// IsNil 返回 reflect.Value 的值是否为 nil，比原生方法更安全
func IsNil(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice, reflect.UnsafePointer:
		return v.IsNil()
	}
	return false
}

// ToBeanDefinition 将 Bean 转换为 BeanDefinition 对象
func ToBeanDefinition(name string, i interface{}) *BeanDefinition {
	return ValueToBeanDefinition(name, reflect.ValueOf(i))
}

// ValueToBeanDefinition 将 Value 转换为 BeanDefinition 对象
func ValueToBeanDefinition(name string, v reflect.Value) *BeanDefinition {
	if !v.IsValid() || IsNil(v) {
		panic(errors.New("bean can't be nil"))
	}
	bean := newObjectBean(v)
	return newBeanDefinition(name, bean)
}

// FnToBeanDefinition 将构造函数转换为 BeanDefinition 对象
func FnToBeanDefinition(name string, fn interface{}, tags ...string) *BeanDefinition {

	// 生成默认名称，取函数名，只针对具名函数，匿名函数还是使用 Bean 的类型名称
	// 具名函数: github.com/go-spring/go-spring/spring-core_test.NewManager
	// 匿名函数: github.com/go-spring/go-spring/spring-core_test.TestDefaultSpringContext_NestValueField.func1.1
	if name == "" {
		fnPtr := reflect.ValueOf(fn).Pointer()
		fnInfo := runtime.FuncForPC(fnPtr)
		s := strings.Split(fnInfo.Name(), "/")
		ss := strings.Split(s[len(s)-1], ".")
		if len(ss) == 2 {
			name = "@" + ss[1]
		}
	}

	bean := newConstructorBean(fn, tags...)
	return newBeanDefinition(name, bean)
}

// MethodToBeanDefinition 将成员方法转换为 BeanDefinition 对象
func MethodToBeanDefinition(name string, selector interface{}, method string, tags ...string) *BeanDefinition {

	if name == "" { // 生成默认名称，取函数名
		name = "@" + method
	}

	bean := newFakeMethodBean(selector, method, tags...)
	return newBeanDefinition(name, bean)
}

// ParseBeanId 解析 BeanId 的内容，"TypeName:BeanName?" 或者 "[]?"
func ParseBeanId(beanId string) (typeName string, beanName string, nullable bool) {

	if ss := strings.Split(beanId, ":"); len(ss) > 1 {
		typeName = ss[0]
		beanName = ss[1]
	} else {
		beanName = ss[0]
	}

	if strings.HasSuffix(beanName, "?") {
		beanName = beanName[:len(beanName)-1]
		nullable = true
	}

	if beanName == "[]" && typeName != "" {
		panic(errors.New("collection mode shouldn't have type"))
	}
	return
}

// ValidBeanValue 返回是否是合法的 Bean 值
func ValidBeanValue(v reflect.Value) (reflect.Type, bool) {
	if v.IsValid() {
		if beanType := v.Type(); IsRefType(beanType.Kind()) {
			return beanType, true
		}
	}
	return nil, false
}
