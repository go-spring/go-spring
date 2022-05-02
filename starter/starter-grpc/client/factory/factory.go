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

package factory

import (
	"github.com/go-spring/spring-core/grpc"
	g "google.golang.org/grpc"
)

// NewClient 根据配置创建 grpc.ClientConnInterface 对象
func NewClient(config grpc.EndpointConfig) (g.ClientConnInterface, error) {
	return g.Dial(config.Address, g.WithInsecure())
}
