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
	"strings"
)

//
// 定义 SpringBean 类型
//
type SpringBean interface{}

//
// 检查是否为合法的 SpringBean 对象
//
func IsValidBean(bean SpringBean) (reflect.Type, bool) {
	// 指针、Map、数组都是合法的 Bean，函数指针也应该可以视为一种 Bean
	if bean != nil {
		t := reflect.TypeOf(bean)
		if k := t.Kind(); k == reflect.Ptr || k == reflect.Map || k == reflect.Slice {
			return t, true
		}
	}
	return nil, false
}

//
// 定义 SpringBean 初始化接口
//
type BeanInitialization interface {
	InitBean(ctx SpringContext)
}

//
// 定义 SpringBean 的状态值
//
type BeanStatus int

const (
	BeanStatus_Default  BeanStatus = 0 // 默认状态
	BeanStatus_Resolved BeanStatus = 1 // 已决议状态
	BeanStatus_Wiring   BeanStatus = 2 // 正在绑定状态
	BeanStatus_Wired    BeanStatus = 3 // 绑定完成状态
)

//
// 定义 BeanDefinition 类型
//
type BeanDefinition struct {
	Bean     SpringBean    // 对象指针
	Name     string        // 名称
	Status   BeanStatus    // 状态
	Type     reflect.Type  // 类型
	TypeName string        // 原始类型的全限定名
	Value    reflect.Value // 值
	cond     *Conditional  // 注册条件
}

//
// 获取原始类型的全限定名，golang 允许不同的路径下存在相同的包，故此有全限定名的需求。形如
// "github.com/go-spring/go-spring/spring-core/SpringCore.DefaultSpringContext"
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
// 测试类型全限定名和 Bean 名称是否都能匹配。
//
func (bean *BeanDefinition) Match(typeName string, beanName string) bool {

	typeIsSame := false
	if typeName == "" || bean.TypeName == typeName {
		typeIsSame = true
	}

	nameIsSame := false
	if beanName == "" || bean.Name == beanName {
		nameIsSame = true
	}

	return typeIsSame && nameIsSame
}

//
// 将 SpringBean 转换为 BeanDefinition 对象
//
func ToBeanDefinition(name string, bean SpringBean) *BeanDefinition {

	var (
		ok bool
		t  reflect.Type
	)

	if t, ok = IsValidBean(bean); !ok {
		panic("bean must be pointer or slice or map")
	}

	// 生成默认名称
	if name == "" {
		name = t.String()
	}

	return &BeanDefinition{
		Status:   BeanStatus_Default,
		Name:     name,
		Bean:     bean,
		Type:     t,
		TypeName: TypeName(t),
		Value:    reflect.ValueOf(bean),
	}
}

//
// 解析 BeanId 的内容，"TypeName:BeanName?" 或者 "[]?"
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
		panic("collection mode shouldn't have type")
	}
	return
}
