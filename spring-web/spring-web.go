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

package SpringWeb

import (
	"net/http"
	"encoding/json"
	"runtime/debug"
	Logger "github.com/didichuxing/go-spring/spring-logger"
)

//
// 错误值
//
type Error struct {
	Code int32  `json:"code"`
	Msg  string `json:"msg"`
}

var (
	ERROR   = Error{-1, "ERROR"}
	SUCCESS = Error{200, "SUCCESS"}
)

//
// 返回值类型
//
type RpcResult struct {
	Error

	Data interface{} `json:"data,omitempty"`
}

type Handler func(*SpringWebContext) interface{}

//
// 封装的路由器
//
type WebRouter struct {
	root string
	wc   WebContainer
}

func NewWebRouter(wc WebContainer, root string) *WebRouter {
	return &WebRouter{
		wc:   wc,
		root: root,
	}
}

func (r *WebRouter) Router(path string) *WebRouter {
	return &WebRouter{
		wc:   r.wc,
		root: r.root + path,
	}
}

func (r *WebRouter) GET(path string, fn Handler, tags ...string) {
	r.wc.GET(r.root+path, fn, tags...)
}

func (r *WebRouter) POST(path string, fn Handler, tags ...string) {
	r.wc.POST(r.root+path, fn, tags...)
}

//
// 容器接口
//
type WebContainer interface {
	Stop()

	Start(address string) error
	StartTLS(address string, certFile, keyFile string) error

	Router(path string) *WebRouter

	GET(path string, fn Handler, tags ...string)
	POST(path string, fn Handler, tags ...string)
}

//
// 容器上下文
//
type SpringWebContext struct {
	R *http.Request
	W http.ResponseWriter
}

func (context *SpringWebContext) Bind(i interface{}) error {
	bytes := make([]byte, context.R.ContentLength)
	context.R.Body.Read(bytes)
	return json.Unmarshal(bytes, i)
}

func (context *SpringWebContext) Json(i interface{}) error {
	bytes, err := json.MarshalIndent(i, "", "  ")
	_, err = context.W.Write(bytes)
	return err
}

func (context *SpringWebContext) Write(b []byte) error {
	_, err := context.W.Write(b)
	return err
}

//
// 直接回复响应
//
const DirectResponse = "direct response"

//
// 包装处理器
//
type HandlerWrapper struct {
	Fn Handler
}

func (handler *HandlerWrapper) Handler(w http.ResponseWriter, r *http.Request) {

	ctx := &SpringWebContext{W: w, R: r}

	defer func() {
		if err := recover(); err != nil {
			Logger.Errorln(string(debug.Stack()))
			ctx.Json(err)
		}
	}()

	data := handler.Fn(ctx)

	if str, ok := data.(string); ok {
		if str != DirectResponse {
			ctx.Write([]byte(str))
		}
	} else {
		ctx.Json(&RpcResult{SUCCESS, data})
	}
}

//
// 初始化 Spring Controller
//
type SpringControllerInitialization interface {
	InitController(c WebContainer)
}
