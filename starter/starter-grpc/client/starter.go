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

package StarterGrpcClient

import (
	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/gs/arg"
	"github.com/go-spring/starter-core"
	"github.com/go-spring/starter-grpc/client/factory"
)

func init() {
	gs.OnProperty("grpc.endpoint", func(endpoints map[string]StarterCore.GrpcEndpointConfig) {
		for endpoint, config := range endpoints {
			gs.Provide(factory.NewClient, arg.Value(config)).Name(endpoint)
		}
	})
}
