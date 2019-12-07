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
// 设置 bean 注册需要满足的条件
//
func (annotation *Annotation) Conditional(cond *Conditional) *Annotation {
	annotation.bean.cond.Conditional(cond)
	return annotation
}

//
// 指定的属性值匹配时注册当前 bean
//
func (annotation *Annotation) ConditionOnProperty(name string, havingValue string) *Annotation {
	annotation.bean.cond.ConditionOnProperty(name, havingValue)
	return annotation
}

//
// 指定的 bean 存在时注册当前 bean
//
func (annotation *Annotation) ConditionalOnBean(beanId string) *Annotation {
	annotation.bean.cond.ConditionalOnBean(beanId)
	return annotation
}

//
// 指定的 bean 不存在时注册当前 bean
//
func (annotation *Annotation) ConditionalOnMissingBean(beanId string) *Annotation {
	annotation.bean.cond.ConditionalOnMissingBean(beanId)
	return annotation
}

//
// 设置 bean 注册需要满足的表达式条件
//
func (annotation *Annotation) ConditionalOnExpression(expression string) *Annotation {
	annotation.bean.cond.ConditionalOnExpression(expression)
	return annotation
}

//
// 设置 bean 注册需要满足的函数条件
//
func (annotation *Annotation) ConditionOnMatches(fn ConditionFunc) *Annotation {
	annotation.bean.cond.ConditionOnMatches(fn)
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
// 设置 bean 为主版本
//
func (annotation *Annotation) Primary(primary bool) *Annotation {
	annotation.bean.primary = primary
	return annotation
}
