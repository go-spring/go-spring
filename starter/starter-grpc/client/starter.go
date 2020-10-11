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
	"fmt"

	"github.com/go-spring/spring-boot"
	"github.com/go-spring/spring-core"
	"github.com/go-spring/starter-grpc/client/factory"
)

// NOTE: 为了避免该变量被导出，所以使用了小写模式。
const grpc_endpoint_prefix = "grpc.endpoint"

func init() {
	SpringBoot.AfterPrepare(func(ctx SpringCore.SpringContext) {
		for endpoint := range ctx.GetGroupedProperties(grpc_endpoint_prefix) {
			tag := fmt.Sprintf("${%s.%s}", grpc_endpoint_prefix, endpoint)
			ctx.RegisterNameBeanFn(endpoint, GrpcClientFactory.NewClientConnInterface, tag)
		}
	})
}
