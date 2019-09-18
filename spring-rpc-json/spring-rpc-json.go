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

package SpringRpcJson

import (
	"encoding/json"
	"net/http"
	"context"
	"fmt"
	"net/url"
	"strings"
	"encoding/xml"
	"errors"
	"github.com/didi/go-spring/spring-rpc"
)

//
// 简单的错误信息
//
type ErrorCode struct {
	Code int32  `json:"code"`
	Msg  string `json:"msg"`
}

//
// 详细的错误信息
//
type Error struct {
	ErrorCode

	// 详细错误
	Error string `json:"err"`
}

var (
	ERROR   = ErrorCode{-1, "ERROR"}
	SUCCESS = ErrorCode{200, "SUCCESS"}
)

//
// 返回值类型
//
type RpcResult struct {
	Error

	Data interface{} `json:"data,omitempty"`
}

func NewRpcResult(code ErrorCode, err string, data interface{}) *RpcResult {
	return &RpcResult{
		Error: Error{code, err},
		Data:  data,
	}
}

func NewSuccessRpcResult(data interface{}) *RpcResult {
	return &RpcResult{
		Error: Error{SUCCESS, ""},
		Data:  data,
	}
}

func NewErrorRpcResult(err string) *RpcResult {
	return &RpcResult{
		Error: Error{ERROR, err},
	}
}

func ReadRpcResult(b []byte, i interface{}) error {
	var r RpcResult
	r.Data = i
	return json.Unmarshal(b, &r)
}

type SpringRpcJsonContext struct {
	R *http.Request
	W http.ResponseWriter
}

func (ctx *SpringRpcJsonContext) Context() context.Context {
	return context.Background()
}

func (ctx *SpringRpcJsonContext) Bind(i interface{}) error {
	req := ctx.R
	ctype := req.Header.Get("Content-Type")
	switch {
	case strings.HasPrefix(ctype, "application/json"):
		if err := json.NewDecoder(req.Body).Decode(i); err != nil {
			return err
		}
	case strings.HasPrefix(ctype, "application/xml"):
		if err := xml.NewDecoder(req.Body).Decode(i); err != nil {
			return err
		}
	case strings.HasPrefix(ctype, "application/x-www-form-urlencoded"):
		params, err := FormParams(req)
		if err != nil {
			return err
		}
		if err = bindData(i, params, "form"); err != nil {
			return err
		}
	default:
		return errors.New(fmt.Sprint(http.StatusUnsupportedMediaType))
	}
	return nil
}

func (ctx *SpringRpcJsonContext) Json(i interface{}) error {
	ctx.W.Header().Set("Content-Type", "application/json")
	bytes, err := json.MarshalIndent(i, "", "  ")
	_, err = ctx.W.Write(bytes)
	return err
}

type SpringRpcJsonContainer struct {
	HttpServer *http.Server
}

func (c *SpringRpcJsonContainer) Stop() {
	c.HttpServer.Shutdown(context.TODO())
}

func (c *SpringRpcJsonContainer) Start(address string) error {
	c.HttpServer = &http.Server{Addr: address, Handler: nil}
	return c.HttpServer.ListenAndServe()
}

func (c *SpringRpcJsonContainer) StartTLS(address string, certFile, keyFile string) error {
	c.HttpServer = &http.Server{Addr: address, Handler: nil}
	return c.HttpServer.ListenAndServeTLS(certFile, keyFile)
}

func (c *SpringRpcJsonContainer) Register(service string, method string, fn SpringRpc.Handler) {

	var path string

	if strings.HasPrefix(service, "/") {
		path = service
	} else {
		path = "/" + service + "_" + method
	}

	http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {

		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		ctx := &SpringRpcJsonContext{W: w, R: r}

		defer func() {
			if err := recover(); err != nil {
				//Logger.Errorln(string(debug.Stack()))

				var errString string
				if e, ok := err.(error); ok {
					errString = e.Error()
				} else {
					errString = fmt.Sprint(err)
				}

				ctx.Json(NewErrorRpcResult(errString))
			}
		}()

		data := fn(ctx)

		if data != SpringRpc.DirectResponse {
			ctx.Json(NewSuccessRpcResult(data))
		}
	})
}

func CallService(service string, method string, reqData interface{}, respData interface{}) error {

	b, _ := json.MarshalIndent(reqData, "", "  ")
	fmt.Println(string(b))

	data := url.Values{}
	data["v"] = []string{string(b)}

	resp, err := http.PostForm("http://127.0.0.1:8080"+"/"+service+"_"+method, data)
	if err != nil {
		return err
	}

	body := make([]byte, resp.ContentLength)
	resp.Body.Read(body)

	return ReadRpcResult(body, respData)
}
