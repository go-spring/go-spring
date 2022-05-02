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

package internal

// GrpcServerConfig gRPC 服务器配置，通常配合服务器名称前缀一起使用。
type GrpcServerConfig struct {
	Port int `value:"${port:=9090}"`
}

// GrpcEndpointConfig gRPC 服务端点配置，通常配合端点名称前缀一起使用。
type GrpcEndpointConfig struct {
	Address string `value:"${address:=127.0.0.1:9090}"`
}
