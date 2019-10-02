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

import (
	"encoding/json"
	"github.com/didi/go-spring/spring-trace"
)

//
// RPC 错误值
//
type RpcError struct {
	Code int32  `json:"code"` // 错误码
	Msg  string `json:"msg"`  // 错误信息
}

func (e *RpcError) Panic(err error) *RpcPanic {
	return &RpcPanic{
		Result: &RpcResult{
			RpcError: e,
			Err:      err.Error(),
		},
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

	Err  string      `json:"err"`  // 错误源
	Data interface{} `json:"data"` // 返回值数据
}

//
// 工厂函数
//
func NewRpcResult(e *RpcError, data interface{}) *RpcResult {
	return &RpcResult{
		RpcError: e,
		Data:     data,
	}
}

func (r *RpcResult) String() string {
	b, _ := json.Marshal(r)
	return string(b)
}

//
// RPC 上下文
//
type RpcContext interface {
	SpringTrace.TraceContext

	Bind(i interface{}) error
}

type Handler func(RpcContext) interface{}

type RpcContainer interface {
	Stop()

	// 启动 RPC 服务器
	Start(address string) error
	StartTLS(address string, certFile, keyFile string) error

	// 注册 RPC 方法（服务名+方法名）
	Register(service string, method string, fn Handler)
}

type RpcBeanInitialization interface {
	InitRpcBean(c RpcContainer)
}
