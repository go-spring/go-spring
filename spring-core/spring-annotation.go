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

func (annotation *Annotation) Conditional(cond *Conditional) *Annotation {
	annotation.bean.cond.Conditional(cond)
	return annotation
}

func (annotation *Annotation) ConditionOnProperty(name string, havingValue string) *Annotation {
	annotation.bean.cond.ConditionOnProperty(name, havingValue)
	return annotation
}

func (annotation *Annotation) ConditionalOnBean(beanId string) *Annotation {
	annotation.bean.cond.ConditionalOnBean(beanId)
	return annotation
}

func (annotation *Annotation) ConditionalOnMissingBean(beanId string) *Annotation {
	annotation.bean.cond.ConditionalOnMissingBean(beanId)
	return annotation
}

func (annotation *Annotation) ConditionalOnExpression(expression string) *Annotation {
	annotation.bean.cond.ConditionalOnExpression(expression)
	return annotation
}

func (annotation *Annotation) ConditionOnMatches(fn ConditionFunc) *Annotation {
	annotation.bean.cond.ConditionOnMatches(fn)
	return annotation
}

func (annotation *Annotation) Profile(profile string) *Annotation {
	annotation.bean.profile = profile
	return annotation
}

func (annotation *Annotation) DependsOn(beanId ...string) *Annotation {
	annotation.bean.dependsOn = beanId
	return annotation
}
