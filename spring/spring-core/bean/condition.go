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

package bean

import (
	"github.com/go-spring/spring-core/properties"
)

type ConditionContext interface {

	// GetProfile 返回运行环境
	GetProfile() string

	//Properties
	Properties() properties.Properties

	// FindBean 查询单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
	// 它和 GetBean 的区别是它在调用后不能保证返回的 Bean 已经完成了注入和绑定过程。
	FindBean(selector BeanSelector) (*BeanDefinition, bool)
}

// Condition 定义一个判断条件
type Condition interface {
	// Matches 成功返回 true，失败返回 false
	Matches(ctx ConditionContext) bool
}
