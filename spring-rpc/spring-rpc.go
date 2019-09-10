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

package SpringRpc

import "context"

type SpringRpcContext interface {
	Bind(i interface{}) error
	Context() context.Context
}

type Handler func(SpringRpcContext) interface{}

type RpcContainer interface {
	Stop()

	Start(address string) error
	StartTLS(address string, certFile, keyFile string) error

	//
	// {"code":200,"msg":"success","data":null}
	//
	Register(service string, method string, fn Handler)
}

type RpcBeanInitialization interface {
	InitRpcBean(c RpcContainer)
}

//
// 兼容老代码，直接回复响应
//
const DirectResponse = "direct response"
