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
	"reflect"
	"strconv"
	"strings"
)

// 哪些类型的数据可以成为 Bean？一般来讲引用类型的数据都可以成为 Bean。
// 当使用对象注册时，无论是否转成 interface 都能获取到对象的真实类型，
// 当使用构造函数注册时，如果返回的是非引用类型会强制转成对应的引用类型，
// 如果返回的是 interface 那么这种情况下会使用 interface 的类型注册。

// 哪些类型可以成为 Bean 的接收者？除了使用 Bean 的真实类型去接收，还可
// 以使用 Bean 实现的 interface 去接收，而且推荐用 interface 去接收。

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

//
// IsRefType 返回是否是引用类型
//
func IsRefType(k reflect.Kind) bool {
	return kindType[k] == refType
}

//
// IsValueType 返回是否是值类型
//
func IsValueType(k reflect.Kind) bool {
	return kindType[k] == valType
}

//
// TypeName 获取原始类型的全限定名，golang 允许不同的路径下存在相同的包，故此有全限定名的需求。
// 形如 "github.com/go-spring/go-spring/spring-core/SpringCore.DefaultSpringContext"
//
func TypeName(t reflect.Type) string {

	if t == nil {
		panic("type shouldn't be nil")
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

//
// ErrorType 定义 error 的类型
//
var ErrorType = reflect.TypeOf((*error)(nil)).Elem()

//
// SpringBean 定义 Bean 源的接口
//
type SpringBean interface {
	Bean() interface{}    // 源
	Type() reflect.Type   // 类型
	Value() reflect.Value // 值
	TypeName() string     // 原始类型的全限定名
}

//
// originalBean 只保存 Bean 源
//
type originalBean struct {
	bean     interface{}   // 源
	rType    reflect.Type  // 类型
	rValue   reflect.Value // 值
	typeName string        // 原始类型的全限定名
}

//
// newOriginalBean originalBean 的构造函数
//
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

//
// Bean 返回 Bean 的源
//
func (b *originalBean) Bean() interface{} {
	return b.bean
}

//
// Type 返回 Bean 的类型
//
func (b *originalBean) Type() reflect.Type {
	return b.rType
}

//
// Value 返回 Bean 的值
//
func (b *originalBean) Value() reflect.Value {
	return b.rValue
}

//
// TypeName 返回 Bean 的原始类型全限定名
//
func (b *originalBean) TypeName() string {
	return b.typeName
}

//
// constructorBean 保存构造函数定义的 Bean 源
//
type constructorBean struct {
	originalBean

	fn  interface{}  // 构造函数
	arg fnBindingArg // 构造函数的参数绑定
}

//
// newConstructorBean constructorBean 的构造函数，所有 tag 都必须同时有或者同时没有序号。
//
func newConstructorBean(fn interface{}, tags ...string) *constructorBean {

	fnType := reflect.TypeOf(fn)

	if fnType.Kind() != reflect.Func || fnType.NumOut() < 1 || fnType.NumOut() > 2 {
		panic(errors.New("constructor must be \"func(...) bean\" or \"func(...) (bean, error)\""))
	}

	if fnType.NumOut() == 2 { // 第二个返回值必须是 error 类型
		if !fnType.Out(1).Implements(ErrorType) {
			panic(errors.New("constructor must be \"func(...) bean\" or \"func(...) (bean, error)\""))
		}
	}

	fnTags := make([]string, fnType.NumIn())

	if len(tags) > 0 {
		indexed := false // 是否包含序号

		if tag := tags[0]; tag != "" {
			if i := strings.Index(tag, ":"); i > 0 {
				_, err := strconv.Atoi(tag[:i])
				indexed = err == nil
			}
		}

		if indexed { // 有序号
			for _, tag := range tags {
				index := strings.Index(tag, ":")
				if index <= 0 {
					panic("tag \"" + tag + "\" should have index")
				}
				i, err := strconv.Atoi(tag[:index])
				if err != nil {
					panic("tag \"" + tag + "\" should have index")
				}
				fnTags[i] = tag[index+1:]
			}

		} else { // 无序号
			for i, tag := range tags {
				if index := strings.Index(tag, ":"); index > 0 {
					_, err := strconv.Atoi(tag[:index])
					if err == nil {
						panic("tag \"" + tag + "\" should no index")
					}
				}
				fnTags[i] = tag
			}
		}
	}

	t := fnType.Out(0)

	// 创建指针类型
	v := reflect.New(t)

	if IsRefType(t.Kind()) {
		v = v.Elem()
	}

	// 重新确定类型
	t = v.Type()

	return &constructorBean{
		originalBean: originalBean{
			bean:     v.Interface(),
			rType:    t,
			typeName: TypeName(t),
			rValue:   v,
		},
		fn:  fn,
		arg: &fnStringBindingArg{fnTags},
	}
}

//
// methodBean 保存成员方法定义的 Bean 源
//
type methodBean struct {
	originalBean

	parent *BeanDefinition // 父对象
	method string          // 成员方法名称
	arg    fnBindingArg    // 成员方法的参数绑定
}

//
// newMethodBean methodBean 的构造函数，所有 tag 都必须同时有或者同时没有序号。
//
func newMethodBean(parent *BeanDefinition, method string, tags ...string) *methodBean {

	fnValue := parent.Value().MethodByName(method)

	if ok := fnValue.IsValid(); !ok {
		panic(errors.New("can't find method"))
	}

	fnType := fnValue.Type()

	if fnType.NumOut() < 1 || fnType.NumOut() > 2 {
		panic(errors.New("method must be \"func(...) bean\" or \"func(...) (bean, error)\""))
	}

	if fnType.NumOut() == 2 { // 第二个返回值必须是 error 类型
		if !fnType.Out(1).Implements(ErrorType) {
			panic(errors.New("method must be \"func(...) bean\" or \"func(...) (bean, error)\""))
		}
	}

	fnTags := make([]string, fnType.NumIn())

	if len(tags) > 0 {
		indexed := false // 是否包含序号

		if tag := tags[0]; tag != "" {
			if i := strings.Index(tag, ":"); i > 0 {
				_, err := strconv.Atoi(tag[:i])
				indexed = err == nil
			}
		}

		if indexed { // 有序号
			for _, tag := range tags {
				index := strings.Index(tag, ":")
				if index <= 0 {
					panic("tag \"" + tag + "\" should have index")
				}
				i, err := strconv.Atoi(tag[:index])
				if err != nil {
					panic("tag \"" + tag + "\" should have index")
				}
				fnTags[i] = tag[index+1:]
			}

		} else { // 无序号
			for i, tag := range tags {
				if index := strings.Index(tag, ":"); index > 0 {
					_, err := strconv.Atoi(tag[:index])
					if err == nil {
						panic("tag \"" + tag + "\" should no index")
					}
				}
				fnTags[i] = tag
			}
		}
	}

	t := fnType.Out(0)

	// 创建指针类型
	v := reflect.New(t)

	if IsRefType(t.Kind()) {
		v = v.Elem()
	}

	// 重新确定类型
	t = v.Type()

	return &methodBean{
		originalBean: originalBean{
			bean:     v.Interface(),
			rType:    t,
			typeName: TypeName(t),
			rValue:   v,
		},
		parent: parent,
		method: method,
		arg:    &fnStringBindingArg{fnTags},
	}
}

//
// 定义 Bean 的状态值
//
type beanStatus int

const (
	beanStatus_Default  = beanStatus(0) // 默认状态
	beanStatus_Resolved = beanStatus(1) // 已决议状态
	beanStatus_Wiring   = beanStatus(2) // 正在绑定状态
	beanStatus_Wired    = beanStatus(3) // 绑定完成状态
	beanStatus_Deleted  = beanStatus(4) // 已删除状态
)

//
// BeanDefinition 存储 Bean 的详细定义
//
type BeanDefinition struct {
	SpringBean

	name   string     // 名称
	status beanStatus // 状态

	Constriction

	primary   bool        // 主版本
	dependsOn []string    // 非直接依赖
	initFunc  interface{} // 绑定结束的回调

	file string // 注册点信息
	line int    // 注册点信息
}

//
// newBeanDefinition BeanDefinition 的构造函数
//
func newBeanDefinition(bean SpringBean, name string) *BeanDefinition {

	if name == "" { // 生成默认名称
		name = bean.Type().String()
	}

	return &BeanDefinition{
		SpringBean: bean,
		name:       name,
		status:     beanStatus_Default,
	}
}

//
// Name 返回 Bean 的名称
//
func (d *BeanDefinition) Name() string {
	return d.name
}

//
// BeanId 返回 Bean 的 BeanId
//
func (d *BeanDefinition) BeanId() string {
	return d.TypeName() + ":" + d.name
}

//
// Match 测试 Bean 的类型全限定名和 Bean 的名称是否都能匹配。
//
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

//
// ToBeanDefinition 将 Bean 转换为 BeanDefinition 对象
//
func ToBeanDefinition(name string, i interface{}) *BeanDefinition {
	bean := newOriginalBean(i)
	return newBeanDefinition(bean, name)
}

//
// FnToBeanDefinition 将构造函数转换为 BeanDefinition 对象
//
func FnToBeanDefinition(name string, fn interface{}, tags ...string) *BeanDefinition {
	bean := newConstructorBean(fn, tags...)
	return newBeanDefinition(bean, name)
}

//
// MethodToBeanDefinition 将成员方法转换为 BeanDefinition 对象
//
func MethodToBeanDefinition(name string, parent *BeanDefinition, method string, tags ...string) *BeanDefinition {
	bean := newMethodBean(parent, method, tags...)
	return newBeanDefinition(bean, name)
}

//
// ConditionOn 为 Bean 设置一个 Condition
//
func (d *BeanDefinition) ConditionOn(cond Condition) *BeanDefinition {
	d.Constriction.ConditionOn(cond)
	return d
}

//
// ConditionOnProperty 为 Bean 设置一个 PropertyCondition
//
func (d *BeanDefinition) ConditionOnProperty(name string) *BeanDefinition {
	d.Constriction.ConditionOnProperty(name)
	return d
}

//
// ConditionOnMissingProperty 为 Bean 设置一个 MissingPropertyCondition
//
func (d *BeanDefinition) ConditionOnMissingProperty(name string) *BeanDefinition {
	d.Constriction.ConditionOnMissingProperty(name)
	return d
}

//
// ConditionOnPropertyValue 为 Bean 设置一个 PropertyValueCondition
//
func (d *BeanDefinition) ConditionOnPropertyValue(name string, havingValue interface{}) *BeanDefinition {
	d.Constriction.ConditionOnPropertyValue(name, havingValue)
	return d
}

//
// ConditionOnBean 为 Bean 设置一个 BeanCondition
//
func (d *BeanDefinition) ConditionOnBean(beanId string) *BeanDefinition {
	d.Constriction.ConditionOnBean(beanId)
	return d
}

//
// ConditionOnMissingBean 为 Bean 设置一个 MissingBeanCondition
//
func (d *BeanDefinition) ConditionOnMissingBean(beanId string) *BeanDefinition {
	d.Constriction.ConditionOnMissingBean(beanId)
	return d
}

//
// ConditionOnExpression 为 Bean 设置一个 ExpressionCondition
//
func (d *BeanDefinition) ConditionOnExpression(expression string) *BeanDefinition {
	d.Constriction.ConditionOnExpression(expression)
	return d
}

//
// ConditionOnMatches 为 Bean 设置一个 FunctionCondition
//
func (d *BeanDefinition) ConditionOnMatches(fn ConditionFunc) *BeanDefinition {
	d.Constriction.ConditionOnMatches(fn)
	return d
}

//
// Options 设置 Option 模式函数的参数绑定
//
func (d *BeanDefinition) Options(options ...*optionArg) *BeanDefinition {
	switch b := d.SpringBean.(type) {
	case *constructorBean:
		b.arg = &fnOptionBindingArg{options}
	case *methodBean:
		b.arg = &fnOptionBindingArg{options}
	default:
		panic(errors.New("只有通过 func 定义的 Bean 才能调用此方法"))
	}
	return d
}

//
// Profile 设置 Bean 的运行环境
//
func (d *BeanDefinition) Profile(profile string) *BeanDefinition {
	d.Constriction.Profile(profile)
	return d
}

//
// DependsOn 设置 Bean 的非直接依赖
//
func (d *BeanDefinition) DependsOn(beanIds ...string) *BeanDefinition {
	if len(d.dependsOn) > 0 {
		panic(errors.New("dependsOn already set"))
	}
	d.dependsOn = beanIds
	return d
}

//
// Primary 设置 Bean 的优先级
//
func (d *BeanDefinition) Primary(primary bool) *BeanDefinition {
	d.primary = primary
	return d
}

//
// InitFunc 设置 Bean 绑定结束的回调
//
func (d *BeanDefinition) InitFunc(fn interface{}) *BeanDefinition {

	fnType := reflect.TypeOf(fn)
	fnValue := reflect.ValueOf(fn)

	if fnValue.Kind() != reflect.Func || fnType.NumOut() > 0 || fnType.NumIn() != 1 || fnType.In(0) != d.Type() {
		panic(errors.New("initFunc should be func(bean)"))
	}

	d.initFunc = fn
	return d
}

//
// Apply 为 Bean 应用自定义限制
//
func (d *BeanDefinition) Apply(c *Constriction) *BeanDefinition {
	d.Constriction.Apply(c)
	return d
}

//
// ParseBeanId 解析 BeanId 的内容，"TypeName:BeanName?" 或者 "[]?"
//
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
