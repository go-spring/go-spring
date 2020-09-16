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

package testcases

import (
	"container/list"
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-spring/spring-utils"
	"github.com/go-spring/spring-web"
)

///////////////////// filter ////////////////////////

type LogFilter struct{}

func (f *LogFilter) Invoke(ctx SpringWeb.WebContext, chain SpringWeb.FilterChain) {

	if strings.Index(ctx.Path(), "*") > 0 {
		fmt.Println(ctx.Path(), "->", ctx.Request().URL)
	} else {
		fmt.Println(ctx.Path())
	}

	chain.Next(ctx)
}

type InterruptFilter struct{}

func (f *InterruptFilter) Invoke(ctx SpringWeb.WebContext, chain SpringWeb.FilterChain) {
	ctx.LogInfo("interrupt")
}

type GlobalInterruptFilter struct{}

func (f *GlobalInterruptFilter) Invoke(ctx SpringWeb.WebContext, chain SpringWeb.FilterChain) {
	if ctx.Path() == "/global_interrupt" {
		ctx.LogInfo("global interrupt")
	} else {
		chain.Next(ctx)
	}
}

type NumberFilter struct {
	l *list.List
	n int
}

func NewNumberFilter(n int, l *list.List) *NumberFilter {
	return &NumberFilter{
		l: l,
		n: n,
	}
}

func (f *NumberFilter) Invoke(ctx SpringWeb.WebContext, chain SpringWeb.FilterChain) {

	defer func() {
		ctx.LogInfo("after ", f.n)
		f.l.PushBack(f.n)
	}()

	ctx.LogInfo("before ", f.n)
	f.l.PushBack(f.n)

	chain.Next(ctx)
}

type StringFilter struct {
	s string
}

func NewStringFilter(s string) *StringFilter {
	return &StringFilter{s: s}
}

func (f *StringFilter) Invoke(ctx SpringWeb.WebContext, chain SpringWeb.FilterChain) {

	defer ctx.LogInfo("after ", f.s)
	ctx.LogInfo("before ", f.s)

	chain.Next(ctx)
}

///////////////////// service ////////////////////////

type Service struct {
	store map[string]string
}

func NewService() *Service {
	return &Service{
		store: make(map[string]string),
	}
}

func (s *Service) Get(ctx SpringWeb.WebContext) {

	key := ctx.QueryParam("key")
	ctx.LogInfo("/get ", "key=", key)

	val := s.store[key]
	ctx.LogInfo("/get ", "val=", val)

	ctx.String(http.StatusOK, val)
}

func (s *Service) Set(ctx SpringWeb.WebContext) {

	var param struct {
		Name string `form:"name" json:"name"`
		Age  string `form:"age" json:"age"`
	}

	if err := ctx.Bind(&param); err != nil {
		panic(err)
	}

	ctx.LogInfo("/set ", "param="+SpringUtils.ToJson(param))

	s.store["name"] = param.Name
	s.store["age"] = param.Age

	ctx.NoContent(http.StatusOK)
}

func (s *Service) Panic(ctx SpringWeb.WebContext) {
	panic("this is a panic")
}

type EmptyRequest struct{}

type EmptyResponse struct{}

// Empty 验证 echo 和 gin 的 bind 功能，echo 的 bind 不允许空 body，spring-web 做了统一
func (s *Service) Empty(ctx context.Context, request *EmptyRequest) *EmptyResponse {
	return &EmptyResponse{}
}

///////////////////// rpc service ////////////////////////

type RpcService struct{}

type EchoRequest struct {
	Str string `query:"str" form:"str" validate:"required,len=4"`
}

type EchoResponse struct {
	Echo string `json:"echo"`
}

// Echo BIND 的结构体参数形式
func (s *RpcService) Echo(ctx context.Context, request *EchoRequest) *EchoResponse {
	return &EchoResponse{"echo " + request.Str}
}
