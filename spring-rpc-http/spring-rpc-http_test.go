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

package SpringHttpRpc_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-spring/go-spring/spring-gin"
	"github.com/go-spring/go-spring/spring-rpc"
	"github.com/go-spring/go-spring/spring-rpc-http"
	"github.com/go-spring/go-spring/spring-utils"
)

func TestContainer(t *testing.T) {

	c := SpringHttpRpc.NewContainer(SpringGin.NewContainer())
	//c := SpringHttpRpc.NewContainer(SpringEcho.NewContainer())

	store := make(map[string]string)

	type GetReq struct {
		Key string `form:"key" json:"key"`
	}

	c.Register("store", "get", func(ctx SpringRpc.RpcContext) interface{} {

		var param GetReq
		ctx.Bind(&param)
		fmt.Println("/get", "key=", param.Key)

		val := store[param.Key]
		fmt.Println("/get", "val=", val)

		return val
	})

	type SetReq map[string]string

	c.Register("store", "set", func(ctx SpringRpc.RpcContext) interface{} {

		var param SetReq
		ctx.Bind(&param)
		fmt.Println("/set", "param="+SpringUtils.ToJson(param))

		for k, v := range param {
			store[k] = v
		}
		return "ok"
	})

	c.Register("store", "panic", func(ctx SpringRpc.RpcContext) interface{} {

		err := errors.New("this is a panic")
		SpringRpc.ERROR.Panic(err).When(err != nil)

		return "success"
	})

	go c.Start(":8080")

	time.Sleep(time.Millisecond * 100)

	var respGet string
	reqGet := &GetReq{Key: "a"}
	err := SpringHttpRpc.CallService("store", "get", reqGet, &respGet)
	fmt.Println("err:", SpringUtils.String(err), "||", "resp:", respGet)

	var respSet string
	reqSet := &SetReq{"a": "1"}
	err = SpringHttpRpc.CallService("store", "set", reqSet, &respSet)
	fmt.Println("err:", SpringUtils.String(err), "||", "resp:", respSet)

	err = SpringHttpRpc.CallService("store", "get", reqGet, &respGet)
	fmt.Println("err:", SpringUtils.String(err), "||", "resp:", respGet)

	var respPanic string
	err = SpringHttpRpc.CallService("store", "panic", nil, &respPanic)
	fmt.Println("err:", SpringUtils.String(err), "||", "resp:", respPanic)
}
