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

package SpringGin_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/go-spring/go-spring/spring-gin"
	"github.com/go-spring/go-spring/spring-utils"
	"github.com/go-spring/go-spring/spring-web"
)

type NumberFilter struct {
	n int
}

func NewNumberFilter(n int) *NumberFilter {
	return &NumberFilter{
		n: n,
	}
}

func (f *NumberFilter) Invoke(ctx SpringWeb.WebContext, chain *SpringWeb.FilterChain) {
	defer fmt.Println("::after", f.n)
	fmt.Println("::before", f.n)
	chain.Next(ctx)
}

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
	ctx.LogInfo("/get", "key=", key)

	val := s.store[key]
	ctx.LogInfo("/get", "val=", val)

	ctx.String(http.StatusOK, val)
}

func (s *Service) Set(ctx SpringWeb.WebContext) {

	var param struct {
		A string `form:"a" json:"a"`
	}

	ctx.Bind(&param)

	ctx.LogInfo("/set", "param="+SpringUtils.ToJson(param))

	s.store["a"] = param.A
}

func (s *Service) Panic(ctx SpringWeb.WebContext) {
	panic("this is a panic")
}

func TestContainer(t *testing.T) {
	c := SpringGin.NewContainer()

	s := NewService()

	f2 := NewNumberFilter(2)
	f5 := NewNumberFilter(5)
	f7 := NewNumberFilter(7)

	c.GET("/get", s.Get, f2, f5)

	c.Group("", func(r *SpringWeb.Route) {
		r.GET("/panic", s.Panic)
		r.POST("/set", s.Set)
	}, f2, f7)

	go c.Start(":8080")

	time.Sleep(time.Millisecond * 100)
	fmt.Println()

	resp, _ := http.Get("http://127.0.0.1:8080/get?key=a")
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))
	fmt.Println()

	http.PostForm("http://127.0.0.1:8080/set", url.Values{
		"a": []string{"1"},
	})

	fmt.Println()

	resp, _ = http.Get("http://127.0.0.1:8080/get?key=a")
	body, _ = ioutil.ReadAll(resp.Body)
	fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))
	fmt.Println()

	resp, _ = http.Get("http://127.0.0.1:8080/panic")
	body, _ = ioutil.ReadAll(resp.Body)
	fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))
}
