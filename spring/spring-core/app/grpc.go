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

package app

import (
	"github.com/go-spring/spring-core/cond"
	"github.com/go-spring/spring-core/core"
)

///////////////////// gRPC Server //////////////////////

type GRpcServer struct {
	fn          interface{}
	server      interface{}    // 服务对象
	serviceName string         // 服务名称
	cond        cond.Condition // 判断条件
}

// NewGRpcServer GRpcServer 的构造函数
func NewGRpcServer(fn interface{}, serviceName string, server interface{}) *GRpcServer {
	return &GRpcServer{fn: fn, server: server, serviceName: serviceName}
}

// ServiceName 返回服务名称
func (s *GRpcServer) ServiceName() string {
	return s.serviceName
}

// Server 返回服务对象
func (s *GRpcServer) Server() interface{} {
	return s.server
}

// WithCondition 设置一个 Condition
func (s *GRpcServer) WithCondition(cond cond.Condition) *GRpcServer {
	s.cond = cond
	return s
}

// CheckCondition 成功返回 true，失败返回 false
func (s *GRpcServer) CheckCondition(ctx core.ApplicationContext) bool {
	if s.cond == nil {
		return true
	}
	return s.cond.Matches(ctx)
}

///////////////////// gRPC Client //////////////////////

type GRpcClient = core.BeanDefinition

func NewGRpcClient(fn interface{}, endpoint string) *GRpcClient {
	return core.CtorBean(fn, endpoint)
}
