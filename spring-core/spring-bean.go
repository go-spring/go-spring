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
	"strconv"
	"strings"
)

// 哪些类型的数据可以成为 Bean？一般来讲引用类型的数据都可以成为 Bean。
// 当使用对象注册时，无论是否转成 Interface 都能获取到对象的真实类型，
// 当使用构造函数注册时，如果返回的是非引用类型会强制转成对应的引用类型，
// 如果返回的是 Interface 那么这种情况下会使用 Interface 的类型注册。

// 哪些类型可以成为 Bean 的接收者？除了使用 Bean 的真实类型去接收，还可
// 以使用 Bean 实现的 Interface 去接收，而且推荐用 Interface 去接收。

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
	refType, // Array
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

	if pkgPath := t.PkgPath(); pkgPath != "" {
		return pkgPath + "/" + t.String()
	} else {
		return t.String()
	}
}

// ErrorType error 的类型
var ErrorType = reflect.TypeOf((*error)(nil)).Elem()

// SpringBean Bean 源接口
type SpringBean interface {
	Bean() interface{}    // 源
	Type() reflect.Type   // 类型
	Value() reflect.Value // 值
	TypeName() string     // 原始类型的全限定名
}

// originalBean 原始 Bean 源
type originalBean struct {
	bean     interface{}   // 源
	rType    reflect.Type  // 类型
	rValue   reflect.Value // 值
	typeName string        // 原始类型的全限定名
}

// newOriginalBean originalBean 的构造函数
func newOriginalBean(bean interface{}) *originalBean {

	if bean == nil {
		panic(errors.New("nil isn't valid bean"))
	}

	t := reflect.TypeOf(bean)

	if ok := IsRefType(t.Kind()); !ok {
		panic(errors.New("bean must be ref type"))
	}

	return &originalBean{
		bean:     bean,
		rType:    t,
		typeName: TypeName(t),
		rValue:   reflect.ValueOf(bean),
	}
}

// Bean 返回 Bean 的源
func (b *originalBean) Bean() interface{} {
	return b.bean
}

// Type 返回 Bean 的类型
func (b *originalBean) Type() reflect.Type {
	return b.rType
}

// Value 返回 Bean 的值
func (b *originalBean) Value() reflect.Value {
	return b.rValue
}

// TypeName 返回 Bean 的原始类型的全限定名
func (b *originalBean) TypeName() string {
	return b.typeName
}

// functionBean 函数定义的 Bean 源
type functionBean struct {
	originalBean

	fnValue reflect.Value // 函数的值
	arg     fnBindingArg  // 参数绑定
}

// newFunctionBean functionBean 的构造函数
func newFunctionBean(fnValue reflect.Value, tags []string) functionBean {
	fnType := fnValue.Type()

	// 检查是否是合法的 Bean 函数定义
	validFuncType := func() bool {

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
			if !fnType.Out(1).Implements(ErrorType) {
				return false
			}
		}

		return true
	}

	if ok := validFuncType(); !ok {
		t1 := "func(...) bean"
		t2 := "func(...) (bean, error)"
		panic(errors.New(fmt.Sprintf("func bean must be \"%s\" or \"%s\"", t1, t2)))
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
		originalBean: originalBean{
			bean:     v.Interface(),
			rType:    t,
			typeName: TypeName(t),
			rValue:   v,
		},
		fnValue: fnValue,
		arg:     newFnStringBindingArg(fnType, tags),
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
		functionBean: newFunctionBean(reflect.ValueOf(fn), tags),
		fn:           fn,
	}
}

// methodBean 成员方法定义的 Bean 源
type methodBean struct {
	functionBean

	parent *BeanDefinition // 父对象的定义
	method string          // 成员方法名称
}

// newMethodBean methodBean 的构造函数，所有 tag 都必须同时有或者同时没有序号。
func newMethodBean(parent *BeanDefinition, method string, tags ...string) *methodBean {

	fnValue := parent.Value().MethodByName(method)

	if ok := fnValue.IsValid(); !ok {
		panic(errors.New("can't find method: " + method))
	}

	return &methodBean{
		functionBean: newFunctionBean(fnValue, tags),
		parent:       parent,
		method:       method,
	}
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

// BeanDefinition Bean 的详细定义
type BeanDefinition struct {
	bean   SpringBean // 源
	name   string     // 名称
	status beanStatus // 状态

	file string // 注册点所在文件
	line int    // 注册点所在行数

	cond      *Conditional // 判断条件
	primary   bool         // 主版本
	dependsOn []string     // 非直接依赖

	initFunc interface{} // 绑定结束的回调
}

// newBeanDefinition BeanDefinition 的构造函数
func newBeanDefinition(bean SpringBean, name string) *BeanDefinition {

	var (
		file string
		line int
	)

	// 获取注册点信息
	for i := 2; i < 10; i++ {
		_, file0, line0, _ := runtime.Caller(i)
		if !strings.Contains(file0, "/go-spring/go-spring/spring-") || strings.HasSuffix(file0, "_test.go") {
			file = file0
			line = line0
			break
		}
	}

	if name == "" { // 生成默认名称
		name = bean.Type().String()
	}

	return &BeanDefinition{
		bean:   bean,
		name:   name,
		status: beanStatus_Default,
		file:   file,
		line:   line,
		cond:   NewConditional(),
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
	return d.TypeName() + ":" + d.name
}

// Caller 返回 Bean 的调用点
func (d *BeanDefinition) Caller() string {
	return d.file + ":" + strconv.Itoa(d.line)
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
func (d *BeanDefinition) ConditionOnBean(beanId string) *BeanDefinition {
	d.cond.OnBean(beanId)
	return d
}

// ConditionOnMissingBean 为 Bean 设置一个 MissingBeanCondition
func (d *BeanDefinition) ConditionOnMissingBean(beanId string) *BeanDefinition {
	d.cond.OnMissingBean(beanId)
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
		bean.arg = arg
	case *methodBean:
		bean.arg = arg
	default:
		panic(errors.New("只有 func Bean 才能调用此方法"))
	}
	return d
}

// DependsOn 设置 Bean 的非直接依赖
func (d *BeanDefinition) DependsOn(beanIds ...string) *BeanDefinition {
	if len(d.dependsOn) > 0 {
		panic(errors.New("dependsOn already set"))
	}
	d.dependsOn = beanIds
	return d
}

// Primary 设置 Bean 的优先级
func (d *BeanDefinition) Primary(primary bool) *BeanDefinition {
	d.primary = primary
	return d
}

// InitFunc 设置 Bean 的绑定结束回调
func (d *BeanDefinition) InitFunc(fn interface{}) *BeanDefinition {
	fnType := reflect.TypeOf(fn)

	// 判断是否是合法的回调函数
	validInitFunc := func() bool {

		// 必须是函数
		if fnType.Kind() != reflect.Func {
			return false
		}

		// 不能有返回值
		if fnType.NumOut() > 0 {
			return false
		}

		// 必须有一个输入参数
		if fnType.NumIn() != 1 {
			return false
		}

		// 输入参数必须是 Bean 的类型
		if fnType.In(0) != d.Type() {
			return false
		}

		return true
	}

	if ok := validInitFunc(); !ok {
		panic(errors.New("initFunc should be func(bean)"))
	}

	d.initFunc = fn
	return d
}

// ToBeanDefinition 将 Bean 转换为 BeanDefinition 对象
func ToBeanDefinition(name string, i interface{}) *BeanDefinition {
	bean := newOriginalBean(i)
	return newBeanDefinition(bean, name)
}

// FnToBeanDefinition 将构造函数转换为 BeanDefinition 对象
func FnToBeanDefinition(name string, fn interface{}, tags ...string) *BeanDefinition {
	bean := newConstructorBean(fn, tags...)
	return newBeanDefinition(bean, name)
}

// MethodToBeanDefinition 将成员方法转换为 BeanDefinition 对象
func MethodToBeanDefinition(name string, parent *BeanDefinition, method string, tags ...string) *BeanDefinition {
	bean := newMethodBean(parent, method, tags...)
	return newBeanDefinition(bean, name)
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
