package rpc

import (
	"math"
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

// NewRpcResult RpcResult 的构造函数
func NewRpcResult(data interface{}) RpcResult {
	return RpcResult{ErrorCode: DEFAULT, Data: data}
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
	return &RpcResult{ErrorCode: ErrorCode(r), Err: err.Error()}
}
