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

package web

import (
	"fmt"
	"math"

	"github.com/go-spring/spring-core/errors"
	"github.com/go-spring/spring-core/util"
)

var (
	ERROR   = NewRpcError(-1, "ERROR")
	SUCCESS = NewRpcSuccess(200, "SUCCESS")
	DEFAULT = NewErrorCode(math.MaxInt32, "DEFAULT")
)

// ErrorCode 错误码
type ErrorCode struct {
	Code int32  `json:"code"` // 错误码
	Msg  string `json:"msg"`  // 错误信息
}

// NewErrorCode ErrorCode 的构造函数
func NewErrorCode(code int32, msg string) ErrorCode {
	return ErrorCode{Code: code, Msg: msg}
}

// RpcResult 定义 RPC 返回值
type RpcResult struct {
	ErrorCode

	Err  string      `json:"err,omitempty"`  // 错误源
	Data interface{} `json:"data,omitempty"` // 返回值
}

// RpcSuccess 定义一个 RPC 成功值
type RpcSuccess ErrorCode

// NewRpcSuccess RpcSuccess 的构造函数
func NewRpcSuccess(code int32, msg string) RpcSuccess {
	return RpcSuccess(NewErrorCode(code, msg))
}

// Data 绑定一个值
func (r RpcSuccess) Data(data interface{}) *RpcResult {
	return &RpcResult{ErrorCode: ErrorCode(r), Data: data}
}

// RpcError 定义一个 RPC 异常值
type RpcError ErrorCode

// NewRpcError RpcError 的构造函数
func NewRpcError(code int32, msg string) RpcError {
	return RpcError(NewErrorCode(code, msg))
}

// Error 绑定一个错误
func (r RpcError) Error(err error) *RpcResult {
	return r.error(1, err, nil)
}

// ErrorWithData 绑定一个错误和一个值
func (r RpcError) ErrorWithData(err error, data interface{}) *RpcResult {
	return r.error(1, err, data)
}

// error skip 是相对于当前函数的调用深度
func (r RpcError) error(skip int, err error, data interface{}) *RpcResult {
	str := errors.WithFileLine(err, skip+1).Error()
	return &RpcResult{ErrorCode: ErrorCode(r), Err: str, Data: data}
}

// Panic 抛出一个异常值
func (r RpcError) Panic(err error) *util.PanicCond {
	return util.NewPanicCond(func() interface{} {
		return r.error(2, err, nil)
	})
}

// Panicf 抛出一段需要格式化的错误字符串
func (r RpcError) Panicf(format string, a ...interface{}) *util.PanicCond {
	return util.NewPanicCond(func() interface{} {
		return r.error(2, fmt.Errorf(format, a...), nil)
	})
}

// PanicImmediately 立即抛出一个异常值
func (r RpcError) PanicImmediately(err error) {
	panic(r.error(1, err, nil))
}
