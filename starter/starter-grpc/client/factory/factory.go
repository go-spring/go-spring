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

package GrpcClientFactory

import (
	"github.com/go-spring/starter-grpc"
	"google.golang.org/grpc"
)

// NewClientConnInterface 根据配置创建 grpc.ClientConnInterface 对象
func NewClientConnInterface(config StarterGrpc.EndpointConfig) (grpc.ClientConnInterface, error) {
	return grpc.Dial(config.Address, grpc.WithInsecure())
}
