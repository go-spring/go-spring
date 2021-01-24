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

	"github.com/go-spring/spring-core"
	"github.com/go-spring/spring-utils"
)

///////////////////// gRPC Server //////////////////////

// GRpcServerMap gRPC 服务列表
var GRpcServerMap = make(map[reflect.Value]*GRpcServer)

// RegisterGRpcServer 注册 gRPC 服务提供者，fn 是 gRPC 自动生成的服务注册函数，serviceName 是服务名称，
// 必须对应 *_grpc.pg.go 文件里面 grpc.ServiceDesc 的 ServiceName 字段，server 是服务具体提供者对象。
func RegisterGRpcServer(fn interface{}, serviceName string, server interface{}) *GRpcServer {
	v := reflect.ValueOf(fn)
	if _, ok := GRpcServerMap[v]; ok {
		_, _, fnName := SpringUtils.FileLine(fn)
		panic(fmt.Errorf("duplicate registration, gRpcServer: %s", fnName))
	}
	s := newGRpcServer(serviceName, server)
	GRpcServerMap[v] = s
	return s
}

type GRpcServer struct {
	server      interface{}          // 服务对象
	serviceName string               // 服务名称
	cond        SpringCore.Condition // 判断条件
}

// newGRpcServer GRpcServer 的构造函数
func newGRpcServer(serviceName string, server interface{}) *GRpcServer {
	return &GRpcServer{server: server, serviceName: serviceName}
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
func (s *GRpcServer) WithCondition(cond SpringCore.Condition) *GRpcServer {
	s.cond = cond
	return s
}

// CheckCondition 成功返回 true，失败返回 false
func (s *GRpcServer) CheckCondition(ctx SpringCore.ApplicationContext) bool {
	if s.cond == nil {
		return true
	}
	return s.cond.Matches(ctx)
}

///////////////////// gRPC Client //////////////////////

// RegisterGRpcClient 注册 gRPC 服务客户端，fn 是 gRPC 自动生成的客户端构造函数
func RegisterGRpcClient(fn interface{}, endpoint string) *SpringCore.BeanDefinition {
	return RegisterBeanFn(fn, endpoint)
}
