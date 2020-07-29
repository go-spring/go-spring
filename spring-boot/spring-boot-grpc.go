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

package SpringBoot

import (
	"fmt"
	"reflect"

	"github.com/go-spring/go-spring-parent/spring-utils"
	"github.com/go-spring/go-spring/spring-core"
)

///////////////////// gRPC Server //////////////////////

// GRpcServerMap gRPC 服务列表
var GRpcServerMap = make(map[reflect.Value]*GRpcServer)

// RegisterGRpcServer 注册 gRPC 服务，fn 是 gRPC 自动生成的服务注册函数
func RegisterGRpcServer(fn interface{}, server interface{}) *GRpcServer {
	v := reflect.ValueOf(fn)
	if _, ok := GRpcServerMap[v]; ok {
		_, _, fnName := SpringUtils.FileLine(fn)
		panic(fmt.Errorf("duplicate registration, gRpcServer: %s", fnName))
	}
	s := newGRpcServer(server)
	GRpcServerMap[v] = s
	return s
}

type GRpcServer struct {
	server interface{}             // 服务对象
	cond   *SpringCore.Conditional // 判断条件
}

// newGRpcServer GRpcServer 的构造函数
func newGRpcServer(server interface{}) *GRpcServer {
	return &GRpcServer{
		server: server,
		cond:   SpringCore.NewConditional(),
	}
}

// Server 返回服务对象
func (s *GRpcServer) Server() interface{} {
	return s.server
}

// Or c=a||b
func (s *GRpcServer) Or() *GRpcServer {
	s.cond.Or()
	return s
}

// And c=a&&b
func (s *GRpcServer) And() *GRpcServer {
	s.cond.And()
	return s
}

// ConditionOn 设置一个 Condition
func (s *GRpcServer) ConditionOn(cond SpringCore.Condition) *GRpcServer {
	s.cond.OnCondition(cond)
	return s
}

// ConditionNot 设置一个取反的 Condition
func (s *GRpcServer) ConditionNot(cond SpringCore.Condition) *GRpcServer {
	s.cond.OnConditionNot(cond)
	return s
}

// ConditionOnProperty 设置一个 PropertyCondition
func (s *GRpcServer) ConditionOnProperty(name string) *GRpcServer {
	s.cond.OnProperty(name)
	return s
}

// ConditionOnMissingProperty 设置一个 MissingPropertyCondition
func (s *GRpcServer) ConditionOnMissingProperty(name string) *GRpcServer {
	s.cond.OnMissingProperty(name)
	return s
}

// ConditionOnPropertyValue 设置一个 PropertyValueCondition
func (s *GRpcServer) ConditionOnPropertyValue(name string, havingValue interface{},
	options ...SpringCore.PropertyValueConditionOption) *GRpcServer {
	s.cond.OnPropertyValue(name, havingValue, options...)
	return s
}

// ConditionOnOptionalPropertyValue 设置一个 PropertyValueCondition，当属性值不存在时默认条件成立
func (s *GRpcServer) ConditionOnOptionalPropertyValue(name string, havingValue interface{}) *GRpcServer {
	s.cond.OnOptionalPropertyValue(name, havingValue)
	return s
}

// ConditionOnBean 设置一个 BeanCondition
func (s *GRpcServer) ConditionOnBean(selector SpringCore.BeanSelector) *GRpcServer {
	s.cond.OnBean(selector)
	return s
}

// ConditionOnMissingBean 设置一个 MissingBeanCondition
func (s *GRpcServer) ConditionOnMissingBean(selector SpringCore.BeanSelector) *GRpcServer {
	s.cond.OnMissingBean(selector)
	return s
}

// ConditionOnExpression 设置一个 ExpressionCondition
func (s *GRpcServer) ConditionOnExpression(expression string) *GRpcServer {
	s.cond.OnExpression(expression)
	return s
}

// ConditionOnMatches 设置一个 FunctionCondition
func (s *GRpcServer) ConditionOnMatches(fn SpringCore.ConditionFunc) *GRpcServer {
	s.cond.OnMatches(fn)
	return s
}

// ConditionOnProfile 设置一个 ProfileCondition
func (s *GRpcServer) ConditionOnProfile(profile string) *GRpcServer {
	s.cond.OnProfile(profile)
	return s
}

// CheckCondition 成功返回 true，失败返回 false
func (s *GRpcServer) CheckCondition(ctx SpringCore.SpringContext) bool {
	return s.cond.Matches(ctx)
}

///////////////////// gRPC Client //////////////////////

// RegisterGRpcClient 注册 gRPC 服务客户端，fn 是 gRPC 自动生成的客户端构造函数
func RegisterGRpcClient(fn interface{}, endpoint string) *SpringCore.BeanDefinition {
	return RegisterBeanFn(fn, endpoint)
}
