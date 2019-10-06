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

package SpringHttpRpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/didi/go-spring/spring-rpc"
	"github.com/didi/go-spring/spring-web"
)

type Container struct {
	WebContainer SpringWeb.WebContainer
}

func NewContainer(c SpringWeb.WebContainer) *Container {
	return &Container{
		WebContainer: c,
	}
}

func (c *Container) Stop() {
	c.WebContainer.Stop()
}

func (c *Container) Start(address string) error {
	return c.WebContainer.Start(address)
}

func (c *Container) StartTLS(address string, certFile, keyFile string) error {
	return c.WebContainer.StartTLS(address, certFile, keyFile)
}

//
// 组装服务地址
//
func makeServicePath(service string, method string) string {
	if strings.HasPrefix(service, "/") {
		return service
	} else {
		return "/" + service + "_" + method
	}
}

func (c *Container) Register(service string, method string, fn SpringRpc.Handler) {

	path := makeServicePath(service, method)

	// HTTP RPC 只能使用 POST 方法传输数据
	c.WebContainer.POST(path, func(ctx SpringWeb.WebContext) {

		defer func() {
			if r := recover(); r != nil {
				rpcResult, ok := r.(*SpringRpc.RpcResult)
				if !ok {
					rpcResult = SpringRpc.ERROR.Data(nil)
				}
				ctx.JSON(http.StatusOK, rpcResult)
			}
		}()

		// HTTP RPC 只能返回 json 格式的数据
		ctx.Header("Content-Type", "application/json")

		rpcResult := SpringRpc.SUCCESS.Data(fn(ctx))
		err := ctx.JSON(http.StatusOK, rpcResult)
		if err != nil {
			ctx.Logger("__rpc_out").Error(err)
		}
	})
}

//
// 读取 RPC 结果
//
func ReadRpcResult(b []byte, i interface{}) error {
	r := SpringRpc.RpcResult{Data: i}

	err := json.Unmarshal(b, &r)
	if err != nil {
		return err
	}

	if r.Code != SpringRpc.SUCCESS.Code {
		return errors.New(r.Err)
	}

	return nil
}

//
// 请求本地服务
//
func CallService(service string, method string, data interface{}, respData interface{}) error {

	var body io.Reader

	if data != nil {
		b, _ := json.Marshal(data)
		body = bytes.NewReader(b)
		fmt.Println("req:", string(b))
	}

	url := "http://127.0.0.1:8080" + makeServicePath(service, method)
	resp, err := http.Post(url, "application/json", body)
	if err != nil {
		return err
	}

	respBody, _ := ioutil.ReadAll(resp.Body)
	return ReadRpcResult(respBody, respData)
}
