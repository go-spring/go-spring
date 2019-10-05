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

import "github.com/didi/go-spring/spring-trace"

//
// RPC 错误值
//
type RpcError struct {
	Code int32  // 错误码
	Msg  string // 错误信息
}

func (e *RpcError) Panic(err error) *RpcPanic {
	return &RpcPanic{
		Result: &RpcResult{
			RpcError: e,
			Err:      err.Error(),
		},
	}
}

func (e *RpcError) Data(data interface{}) *RpcResult {
	return &RpcResult{
		RpcError: e,
		Data:     data,
	}
}

//
// 封装触发 panic 条件
//
type RpcPanic struct {
	Result *RpcResult
}

func (p *RpcPanic) When(isPanic bool) {
	if isPanic {
		panic(p.Result)
	}
}

var (
	ERROR   = &RpcError{-1, "ERROR"}
	SUCCESS = &RpcError{200, "SUCCESS"}
)

//
// RPC 返回值
//
type RpcResult struct {
	*RpcError

	Err  string      // 错误源
	Data interface{} // 返回值数据
}

//
// RPC 上下文，务必保持是 WebContext 的子集！
//
type RpcContext interface {
	SpringTrace.TraceContext

	Bind(i interface{}) error

	// Get retrieves data from the context.
	Get(key string) interface{}

	// Set saves data in the context.
	Set(key string, val interface{})
}

type Handler func(RpcContext) interface{}

//
// RPC 服务器
//
type RpcContainer interface {
	Stop()

	Start(address string) error
	StartTLS(address string, certFile, keyFile string) error

	// 注册 RPC 方法（服务名+方法名）
	Register(service string, method string, fn Handler)
}

//
// RPC Bean 初始化
//
type RpcBeanInitialization interface {
	InitRpcBean(c RpcContainer)
}
