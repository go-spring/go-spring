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
	VAL_TYPE = 1 // 值类型
	REF_TYPE = 2 // 引用类型
)

var KindType = []uint8{
	0,        // Invalid
	VAL_TYPE, // Bool
	VAL_TYPE, // Int
	VAL_TYPE, // Int8
	VAL_TYPE, // Int16
	VAL_TYPE, // Int32
	VAL_TYPE, // Int64
	VAL_TYPE, // Uint
	VAL_TYPE, // Uint8
	VAL_TYPE, // Uint16
	VAL_TYPE, // Uint32
	VAL_TYPE, // Uint64
	0,        // Uintptr
	VAL_TYPE, // Float32
	VAL_TYPE, // Float64
	VAL_TYPE, // Complex64
	VAL_TYPE, // Complex128
	REF_TYPE, // Array
	REF_TYPE, // Chan
	REF_TYPE, // Func
	REF_TYPE, // Interface
	REF_TYPE, // Map
	REF_TYPE, // Ptr
	REF_TYPE, // Slice
	VAL_TYPE, // String
	VAL_TYPE, // Struct
	0,        // UnsafePointer
}

//
// IsRefType 是否是引用类型
//
func IsRefType(k reflect.Kind) bool {
	return KindType[k] == REF_TYPE
}

//
// IsValueType 是否是值类型
//
func IsValueType(k reflect.Kind) bool {
	return KindType[k] == VAL_TYPE
}

//
// error 的类型
//
var ERROR_TYPE = reflect.TypeOf((*error)(nil)).Elem()

//
// 定义 SpringBean 接口
//
type SpringBean interface {
	Bean() interface{}
	Type() reflect.Type
	Value() reflect.Value
	TypeName() string
}

//
// originalBean 保存原始对象的 SpringBean
//
type originalBean struct {
	bean     interface{}
	rType    reflect.Type  // 类型
	typeName string        // 原始类型的全限定名
	rValue   reflect.Value // 值
}

//
// newOriginalBean 工厂函数
//
func newOriginalBean(bean interface{}) *originalBean {

	if bean == nil {
		panic("nil isn't valid bean")
	}

	t := reflect.TypeOf(bean)

	if !IsRefType(t.Kind()) {
		panic("bean must be ref type")
	}

	return &originalBean{
		bean:     bean,
		rType:    t,
		typeName: TypeName(t),
		rValue:   reflect.ValueOf(bean),
	}
}

//
// Bean
//
func (b *originalBean) Bean() interface{} {
	return b.bean
}

//
// Type
//
func (b *originalBean) Type() reflect.Type {
	return b.rType
}

//
// Value
//
func (b *originalBean) Value() reflect.Value {
	return b.rValue
}

//
// TypeName
//
func (b *originalBean) TypeName() string {
	return b.typeName
}

//
// constructorArg
//
type constructorArg interface {
	Get(ctx SpringContext, fnType reflect.Type) []reflect.Value
}

//
// stringConstructorArg 基于字符串 tag 的构造函数参数
//
type stringConstructorArg struct {
	tags []string
}

//
// Get
//
func (ca *stringConstructorArg) Get(ctx SpringContext, fnType reflect.Type) []reflect.Value {
	args := make([]reflect.Value, fnType.NumIn())
	ctx0 := ctx.(*defaultSpringContext)

	for i, tag := range ca.tags {
		it := fnType.In(i)
		iv := reflect.New(it).Elem()

		if strings.HasPrefix(tag, "$") {
			bindStructField(ctx, it, iv, "", "", tag)
		} else {
			ctx0.getBeanByName(tag, emptyValue, iv, "")
		}

		args[i] = iv
	}
	return args
}

//
// optionConstructorArg 基于 Option 模式的构造函数参数
//
type optionConstructorArg struct {
	options []*optionArg
}

//
// Get
//
func (ca *optionConstructorArg) Get(ctx SpringContext, _ reflect.Type) []reflect.Value {
	ctx0 := ctx.(*defaultSpringContext)
	args := make([]reflect.Value, 0)

	for _, arg := range ca.options {

		// 判断 Option 条件是否成立
		if arg.cond != nil && !arg.cond.Matches(ctx) {
			continue
		}

		optValue := reflect.ValueOf(arg.fn)
		optType := optValue.Type()

		fnTags := make([]string, optType.NumIn())

		if len(arg.tags) > 0 {
			indexed := false // 是否包含序号

			if tag := arg.tags[0]; tag != "" {
				if i := strings.Index(tag, ":"); i > 0 {
					_, err := strconv.Atoi(tag[:i])
					indexed = err == nil
				}
			}

			if indexed { // 有序号
				for _, tag := range arg.tags {
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
				for i, tag := range arg.tags {
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

		optIn := make([]reflect.Value, optType.NumIn())

		for i, tag := range fnTags {

			it := optType.In(i)
			iv := reflect.New(it).Elem()

			if strings.HasPrefix(tag, "$") {
				bindStructField(ctx, it, iv, "", "", tag)
			} else {
				ctx0.getBeanByName(tag, emptyValue, iv, "")
			}

			optIn[i] = iv
		}

		optOut := optValue.Call(optIn)
		args = append(args, optOut[0])
	}
	return args
}

//
// constructorBean 保存构造函数的 SpringBean
//
type constructorBean struct {
	originalBean

	fn  interface{}
	arg constructorArg
}

//
// newConstructorBean 工厂函数，所有 tag 都必须同时有或者同时没有序号。
//
func newConstructorBean(fn interface{}, tags ...string) *constructorBean {

	fnType := reflect.TypeOf(fn)

	if fnType.Kind() != reflect.Func {
		panic("constructor must be \"func(...) bean\" or \"func(...) (bean, error)\"")
	}

	if fnType.NumOut() < 1 || fnType.NumOut() > 2 {
		panic("constructor must be \"func(...) bean\" or \"func(...) (bean, error)\"")
	}

	if fnType.NumOut() == 2 { // 第二个返回值必须是 error 类型
		if !fnType.Out(1).Implements(ERROR_TYPE) {
			panic("constructor must be \"func(...) bean\" or \"func(...) (bean, error)\"")
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
		arg: &stringConstructorArg{fnTags},
	}
}

//
// methodBean 保存成员方法的 SpringBean
//
type methodBean struct {
	originalBean

	method string
	arg    constructorArg
	parent *BeanDefinition
}

//
// newMethodBean 工厂函数，所有 tag 都必须同时有或者同时没有序号。
//
func newMethodBean(parent *BeanDefinition, method string, tags ...string) *methodBean {

	fnValue := parent.Value().MethodByName(method)
	if !fnValue.IsValid() {
		panic("找不到目标方法")
	}

	fnType := fnValue.Type()

	if fnType.NumOut() < 1 || fnType.NumOut() > 2 {
		panic("method must be \"func(...) bean\" or \"func(...) (bean, error)\"")
	}

	if fnType.NumOut() == 2 { // 第二个返回值必须是 error 类型
		if !fnType.Out(1).Implements(ERROR_TYPE) {
			panic("method must be \"func(...) bean\" or \"func(...) (bean, error)\"")
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
		arg:    &stringConstructorArg{fnTags},
	}
}

//
// 定义 Bean 的状态值
//
type BeanStatus int

const (
	BeanStatus_Default  = BeanStatus(0) // 默认状态
	BeanStatus_Resolved = BeanStatus(1) // 已决议状态
	BeanStatus_Wiring   = BeanStatus(2) // 正在绑定状态
	BeanStatus_Wired    = BeanStatus(3) // 绑定完成状态
	BeanStatus_Deleted  = BeanStatus(4) // 已删除状态
)

//
// BeanDefinition 定义 BeanDefinition 类型
//
type BeanDefinition struct {
	SpringBean

	name   string     // 名称
	status BeanStatus // 状态

	Constriction

	primary  bool        // 主版本
	initFunc interface{} // 绑定结束的回调

	file string // 注册点信息
	line int    // 注册点信息
}

//
// newBeanDefinition 工厂函数
//
func newBeanDefinition(bean SpringBean, name string) *BeanDefinition {

	// 生成默认名称
	if name == "" {
		name = bean.Type().String()
	}

	return &BeanDefinition{
		SpringBean: bean,
		name:       name,
		status:     BeanStatus_Default,
	}
}

//
// Name 获取 Name
//
func (d *BeanDefinition) Name() string {
	return d.name
}

//
// BeanId 获取 BeanId
//
func (d *BeanDefinition) BeanId() string {
	return d.TypeName() + ":" + d.name
}

//
// Match 测试类型全限定名和 Bean 名称是否都能匹配。
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
// ConditionOn 设置一个 Condition
//
func (d *BeanDefinition) ConditionOn(cond Condition) *BeanDefinition {
	d.Constriction.ConditionOn(cond)
	return d
}

//
// ConditionOnProperty 设置一个 PropertyCondition
//
func (d *BeanDefinition) ConditionOnProperty(name string) *BeanDefinition {
	d.Constriction.ConditionOnProperty(name)
	return d
}

//
// ConditionOnMissingProperty 设置一个 MissingPropertyCondition
//
func (d *BeanDefinition) ConditionOnMissingProperty(name string) *BeanDefinition {
	d.Constriction.ConditionOnMissingProperty(name)
	return d
}

//
// ConditionOnPropertyValue 设置一个 PropertyValueCondition
//
func (d *BeanDefinition) ConditionOnPropertyValue(name string, havingValue interface{}) *BeanDefinition {
	d.Constriction.ConditionOnPropertyValue(name, havingValue)
	return d
}

//
// ConditionOnBean 设置一个 BeanCondition
//
func (d *BeanDefinition) ConditionOnBean(beanId string) *BeanDefinition {
	d.Constriction.ConditionOnBean(beanId)
	return d
}

//
// ConditionOnMissingBean 设置一个 MissingBeanCondition
//
func (d *BeanDefinition) ConditionOnMissingBean(beanId string) *BeanDefinition {
	d.Constriction.ConditionOnMissingBean(beanId)
	return d
}

//
// ConditionOnExpression 设置一个 ExpressionCondition
//
func (d *BeanDefinition) ConditionOnExpression(expression string) *BeanDefinition {
	d.Constriction.ConditionOnExpression(expression)
	return d
}

//
// ConditionOnMatches 设置一个 FunctionCondition
//
func (d *BeanDefinition) ConditionOnMatches(fn ConditionFunc) *BeanDefinition {
	d.Constriction.ConditionOnMatches(fn)
	return d
}

//
// Options 设置 Option 模式构造函数的参数绑定
//
func (d *BeanDefinition) Options(options ...*optionArg) *BeanDefinition {
	cBean, ok := d.SpringBean.(*constructorBean)
	if !ok {
		panic("只有构造函数 Bean 才能调用此方法")
	}
	cBean.arg = &optionConstructorArg{options}
	return d
}

//
// Profile 设置 bean 的运行环境
//
func (d *BeanDefinition) Profile(profile string) *BeanDefinition {
	d.Constriction.Profile(profile)
	return d
}

//
// DependsOn 设置 bean 的非直接依赖
//
func (d *BeanDefinition) DependsOn(beanId ...string) *BeanDefinition {
	d.Constriction.DependsOn(beanId...)
	return d
}

//
// Primary 设置 bean 的优先级
//
func (d *BeanDefinition) Primary(primary bool) *BeanDefinition {
	d.primary = primary
	return d
}

//
// InitFunc 设置 bean 绑定结束的回调
//
func (d *BeanDefinition) InitFunc(fn interface{}) *BeanDefinition {

	fnType := reflect.TypeOf(fn)
	fnValue := reflect.ValueOf(fn)

	if fnValue.Kind() != reflect.Func || fnType.NumOut() > 0 || fnType.NumIn() != 1 {
		panic("initFunc should be func(bean)")
	}

	if fnType.In(0) != d.Type() {
		panic("initFunc should be func(bean)")
	}

	d.initFunc = fn
	return d
}

//
// Apply 设置 Bean 应用自定义限制
//
func (d *BeanDefinition) Apply(c *Constriction) *BeanDefinition {
	d.Constriction.Apply(c)
	return d
}
