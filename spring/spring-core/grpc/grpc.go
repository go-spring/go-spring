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

package grpc

import (
	"github.com/go-spring/spring-core/cond"
	"github.com/go-spring/spring-core/core"
)

///////////////////// gRPC Server //////////////////////

type Server struct {
	fn          interface{}
	server      interface{}    // 服务对象
	serviceName string         // 服务名称
	cond        cond.Condition // 判断条件
}

// NewServer Server 的构造函数
func NewServer(fn interface{}, serviceName string, server interface{}) *Server {
	return &Server{fn: fn, server: server, serviceName: serviceName}
}

// Handler
func (s *Server) Handler() interface{} {
	return s.fn
}

// ServiceName 返回服务名称
func (s *Server) ServiceName() string {
	return s.serviceName
}

// Server 返回服务对象
func (s *Server) Server() interface{} {
	return s.server
}

// WithCondition 设置一个 Condition
func (s *Server) WithCondition(cond cond.Condition) *Server {
	s.cond = cond
	return s
}

// CheckCondition 成功返回 true，失败返回 false
func (s *Server) CheckCondition(ctx core.ApplicationContext) bool {
	if s.cond == nil {
		return true
	}
	return s.cond.Matches(ctx)
}

///////////////////// gRPC Client //////////////////////

type Client = core.BeanDefinition

func NewClient(fn interface{}, endpoint string) *Client {
	return core.CtorBean(fn, endpoint)
}
