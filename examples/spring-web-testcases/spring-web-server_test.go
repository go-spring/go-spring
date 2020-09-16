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

package testcases_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/go-spring/examples/testcases"
	"github.com/go-spring/spring-echo"
	"github.com/go-spring/spring-gin"
	"github.com/go-spring/spring-utils"
	"github.com/go-spring/spring-web"
)

func TestWebServer(t *testing.T) {

	server := SpringWeb.NewWebServer()
	server.AddFilter(testcases.NewStringFilter("web_server"))

	s := testcases.NewService()

	// 可用于全局的路由分组
	r := SpringWeb.NewRouter("/v1", testcases.NewStringFilter("@router"))
	r.GetMapping("/router", func(ctx SpringWeb.WebContext) {
		err := ctx.String(http.StatusOK, "router:ok")
		SpringUtils.Panic(err).When(err != nil)
	}, testcases.NewStringFilter("@router:/router"))

	// 添加第一个 Web 容器
	{
		cfg := SpringWeb.ContainerConfig{Port: 8080}
		g := SpringGin.NewContainer(cfg)
		server.AddContainer(g)
		g.AddRouter(r)

		g.AddFilter(testcases.NewStringFilter("gin"))
		g.GetMapping("/get", s.Get, testcases.NewStringFilter("gin:/get"))
	}

	// 添加第二个 Web 容器
	{
		cfg := SpringWeb.ContainerConfig{Port: 9090}
		e := SpringEcho.NewContainer(cfg)
		server.AddContainer(e)
		e.AddRouter(r)

		e.AddFilter(testcases.NewStringFilter("echo"))
		r0 := e.Route("", testcases.NewStringFilter("echo:route"))
		{
			r0.PostMapping("/set", s.Set, testcases.NewStringFilter("echo:route:/set"))
			r0.GetMapping("/panic", s.Panic, testcases.NewStringFilter("echo:route:/panic"))
		}
	}

	// 启动 web 服务器
	server.Start()

	time.Sleep(time.Millisecond * 100)
	fmt.Println()

	resp, _ := http.Get("http://127.0.0.1:8080/get?key=a")
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))
	fmt.Println()

	_, _ = http.PostForm("http://127.0.0.1:9090/set", url.Values{
		"a": []string{"1"},
	})

	fmt.Println()

	resp, _ = http.Get("http://127.0.0.1:8080/get?key=a")
	body, _ = ioutil.ReadAll(resp.Body)
	fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))
	fmt.Println()

	resp, _ = http.Get("http://127.0.0.1:9090/panic")
	body, _ = ioutil.ReadAll(resp.Body)
	fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))
	fmt.Println()

	resp, _ = http.Get("http://127.0.0.1:8080/v1/router")
	body, _ = ioutil.ReadAll(resp.Body)
	fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))
	fmt.Println()

	resp, _ = http.Get("http://127.0.0.1:9090/v1/router")
	body, _ = ioutil.ReadAll(resp.Body)
	fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))
	fmt.Println()

	server.Stop(context.TODO())

	time.Sleep(time.Millisecond * 50)
}
