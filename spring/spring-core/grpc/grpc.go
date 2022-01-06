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

import "github.com/go-spring/spring-core/internal"

// ServerConfig gRPC 服务器配置。
type ServerConfig = internal.GrpcServerConfig

// EndpointConfig gRPC 客户端配置。
type EndpointConfig = internal.GrpcEndpointConfig

type Server struct {
	Register interface{} // 服务注册函数
	Service  interface{} // 服务提供者
}
