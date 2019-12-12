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
)

//
// 设定 Bean 的各种元数据
//
type Annotation struct {
	bean *BeanDefinition
}

//
// 构造函数
//
func NewAnnotation(bean *BeanDefinition) *Annotation {
	return &Annotation{
		bean: bean,
	}
}

//
// 设置一个 Condition
//
func (annotation *Annotation) ConditionOn(cond Condition) *Annotation {
	annotation.bean.cond = cond
	return annotation
}

//
// 设置一个 PropertyCondition
//
func (annotation *Annotation) ConditionOnProperty(name string, havingValue string) *Annotation {
	annotation.bean.cond = NewPropertyCondition(name, havingValue)
	return annotation
}

//
// 设置一个 BeanCondition
//
func (annotation *Annotation) ConditionOnBean(beanId string) *Annotation {
	annotation.bean.cond = NewBeanCondition(beanId)
	return annotation
}

//
// 设置一个 MissingBeanCondition
//
func (annotation *Annotation) ConditionOnMissingBean(beanId string) *Annotation {
	annotation.bean.cond = NewMissingBeanCondition(beanId)
	return annotation
}

//
// 设置一个 ExpressionCondition
//
func (annotation *Annotation) ConditionOnExpression(expression string) *Annotation {
	annotation.bean.cond = NewExpressionCondition(expression)
	return annotation
}

//
// 设置一个 FunctionCondition
//
func (annotation *Annotation) ConditionOnMatches(fn ConditionFunc) *Annotation {
	annotation.bean.cond = NewFunctionCondition(fn)
	return annotation
}

//
// 设置 bean 的运行环境
//
func (annotation *Annotation) Profile(profile string) *Annotation {
	annotation.bean.profile = profile
	return annotation
}

//
// 设置 bean 的非直接依赖
//
func (annotation *Annotation) DependsOn(beanId ...string) *Annotation {
	annotation.bean.dependsOn = beanId
	return annotation
}

//
// 设置 bean 的优先级
//
func (annotation *Annotation) Primary(primary bool) *Annotation {
	annotation.bean.primary = primary
	return annotation
}

//
// 设置 bean 绑定结束的回调
//
func (annotation *Annotation) InitFunc(fn interface{}) *Annotation {

	fnType := reflect.TypeOf(fn)
	fnValue := reflect.ValueOf(fn)

	if fnValue.Kind() != reflect.Func || fnType.NumOut() > 0 || fnType.NumIn() != 1 {
		panic("initFunc should be func(bean)")
	}

	if fnType.In(0) != annotation.bean.Type() {
		panic("initFunc should be func(bean)")
	}

	annotation.bean.initFunc = fn
	return annotation
}
