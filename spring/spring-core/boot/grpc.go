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

package boot

import (
	"github.com/go-spring/spring-core/app"
	"github.com/go-spring/spring-core/core"
)

// RegisterGRpcServer 注册 gRPC 服务提供者，fn 是 gRPC 自动生成的服务注册函数，serviceName 是服务名称，
// 必须对应 *_grpc.pg.go 文件里面 grpc.ServiceDesc 的 ServiceName 字段，server 是服务具体提供者对象。
func RegisterGRpcServer(fn interface{}, serviceName string, server interface{}) *app.GRpcServer {
	s := app.NewGRpcServer(fn, serviceName, server)
	gApp.RegisterGRpcServer(s)
	return s
}

// RegisterGRpcClient 注册 gRPC 服务客户端，fn 是 gRPC 自动生成的客户端构造函数
func RegisterGRpcClient(fn interface{}, endpoint string) *core.BeanDefinition {
	c := app.NewGRpcClient(fn, endpoint)
	gApp.RegisterGRpcClient(c)
	return c
}
