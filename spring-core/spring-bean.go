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

//
// 哪些类型的数据可以成为 Bean？一般来讲引用类型的数据都可以成为 Bean。
// 当使用对象注册时，无论是否转成 interface 都能获取到对象的真实类型，
// 当使用构造函数注册时，如果返回的是非引用类型会强制转成对应的引用类型，
// 如果返回的是 interface 那么这种情况下会使用 interface 的类型注册。
//
var _VALID_BEAN_KINDS = []reflect.Kind{
	reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map, reflect.Func,
}

//
// 哪些类型可以成为 Bean 的接收者？除了使用 Bean 的真实类型去接收，还可
// 以使用 Bean 实现的 interface 去接收，而且推荐用 interface 去接收。
//
var _VALID_RECEIVER_KINDS = []reflect.Kind{
	reflect.Interface, reflect.Ptr, reflect.Slice, reflect.Map, reflect.Func,
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
// 保存原始对象的 SpringBean
//
type OriginalBean struct {
	bean     interface{}
	rType    reflect.Type  // 类型
	typeName string        // 原始类型的全限定名
	rValue   reflect.Value // 值
}

//
// 工厂函数
//
func NewOriginalBean(bean interface{}) *OriginalBean {

	if bean == nil {
		panic("nil isn't valid bean")
	}

	t := reflect.TypeOf(bean)

	if !IsValidBean(t.Kind()) {
		panic("bean must be ptr or slice or map or func")
	}

	return &OriginalBean{
		bean:     bean,
		rType:    t,
		typeName: TypeName(t),
		rValue:   reflect.ValueOf(bean),
	}
}

func (b *OriginalBean) Bean() interface{} {
	return b.bean
}

func (b *OriginalBean) Type() reflect.Type {
	return b.rType
}

func (b *OriginalBean) Value() reflect.Value {
	return b.rValue
}

func (b *OriginalBean) TypeName() string {
	return b.typeName
}

type ConstructorArg interface {
	Get(ctx SpringContext, fnType reflect.Type) []reflect.Value
}

//
// 基于字符串 tag 的构造函数参数
//
type StringConstructorArg struct {
	tags []string
}

func (ca *StringConstructorArg) Get(ctx SpringContext, fnType reflect.Type) []reflect.Value {
	args := make([]reflect.Value, fnType.NumIn())
	ctx0 := ctx.(*DefaultSpringContext)

	for i, tag := range ca.tags {
		it := fnType.In(i)
		iv := reflect.New(it).Elem()

		if strings.HasPrefix(tag, "$") {
			bindStructField(ctx, it, iv, "", "", tag)
		} else {
			ctx0.getBeanByName(tag, _EMPTY_VALUE, iv, "")
		}

		args[i] = iv
	}
	return args
}

//
// 基于 Option 模式的构造函数参数
//
type OptionConstructorArg struct {
	options []map[string]interface{}
}

func (ca *OptionConstructorArg) Get(ctx SpringContext, fnType reflect.Type) []reflect.Value {
	ctx0 := ctx.(*DefaultSpringContext)
	args := make([]reflect.Value, 0)

	for _, optMap := range ca.options {
		for tag, opt := range optMap {

			optValue := reflect.ValueOf(opt)
			optType := optValue.Type()

			it := optType.In(0)
			iv := reflect.New(it).Elem()

			if strings.HasPrefix(tag, "$") {
				bindStructField(ctx, it, iv, "", "", tag)
			} else {
				ctx0.getBeanByName(tag, _EMPTY_VALUE, iv, "")
			}

			optOut := optValue.Call([]reflect.Value{iv})
			args = append(args, optOut[0])
		}
	}
	return args
}

//
// 保存构造函数的 SpringBean
//
type ConstructorBean struct {
	OriginalBean

	fn  interface{}
	arg ConstructorArg
}

//
// 工厂函数，所有 tag 都必须同时有或者同时没有序号。
//
func NewConstructorBean(fn interface{}, tags ...string) *ConstructorBean {

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

	if IsValidBean(t.Kind()) {
		v = v.Elem()
	}

	// 重新确定类型
	t = v.Type()

	return &ConstructorBean{
		OriginalBean: OriginalBean{
			bean:     v.Interface(),
			rType:    t,
			typeName: TypeName(t),
			rValue:   v,
		},
		fn:  fn,
		arg: &StringConstructorArg{fnTags},
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
)

//
// 定义 BeanDefinition 类型
//
type BeanDefinition struct {
	SpringBean

	Name      string      // 名称
	status    BeanStatus  // 状态
	cond      Condition   // 注册条件
	profile   string      // 运行环境
	dependsOn []string    // 非直接依赖
	primary   bool        // 主版本
	initFunc  interface{} // 绑定结束的回调
}

//
// 工厂函数
//
func NewBeanDefinition(bean SpringBean, name string) *BeanDefinition {

	// 生成默认名称
	if name == "" {
		name = bean.Type().String()
	}

	return &BeanDefinition{
		SpringBean: bean,
		Name:       name,
		status:     BeanStatus_Default,
	}
}

//
// 测试类型全限定名和 Bean 名称是否都能匹配。
//
func (d *BeanDefinition) Match(typeName string, beanName string) bool {

	typeIsSame := false
	if typeName == "" || d.TypeName() == typeName {
		typeIsSame = true
	}

	nameIsSame := false
	if beanName == "" || d.Name == beanName {
		nameIsSame = true
	}

	return typeIsSame && nameIsSame
}

//
// 将 Bean 转换为 BeanDefinition 对象
//
func ToBeanDefinition(name string, i interface{}) *BeanDefinition {
	bean := NewOriginalBean(i)
	return NewBeanDefinition(bean, name)
}

//
// 将构造函数转换为 BeanDefinition 对象
//
func FnToBeanDefinition(name string, fn interface{}, tags ...string) *BeanDefinition {
	bean := NewConstructorBean(fn, tags...)
	return NewBeanDefinition(bean, name)
}
