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
	"container/list"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-openapi/spec"
	"github.com/go-spring/spring-echo"
	"github.com/go-spring/spring-gin"
	"github.com/go-spring/spring-web"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/magiconair/properties/assert"
)

func TestWebContainer(t *testing.T) {
	cfg := SpringWeb.ContainerConfig{Port: 8080}

	testRun := func(c SpringWeb.WebContainer) {

		c.Swagger().
			WithDescription("web container test").
			AddDefinition("Set", new(spec.Schema).
				Typed("object", "").
				AddRequired("name", "age").
				SetProperty("name", *spec.StringProperty()).
				SetProperty("age", *spec.Int32Property()))

		// 添加容器过滤器，这些过滤器在路由未注册时也仍会执行
		c.AddFilter(&LogFilter{}, &GlobalInterruptFilter{})

		// 启动 web 服务器
		c.Start()

		time.Sleep(time.Millisecond * 100)
		fmt.Println()

		resp, _ := http.Get("http://127.0.0.1:8080/get?key=a")
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))
		fmt.Println()

		resp, _ = http.PostForm("http://127.0.0.1:8080/v1/set", url.Values{
			"a": []string{"1"},
		})
		fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))
		fmt.Println()

		resp, _ = http.Get("http://127.0.0.1:8080/get?key=a")
		body, _ = ioutil.ReadAll(resp.Body)
		fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))
		fmt.Println()

		resp, _ = http.Get("http://127.0.0.1:8080/v1/panic")
		body, _ = ioutil.ReadAll(resp.Body)
		fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))
		fmt.Println()

		resp, _ = http.PostForm("http://127.0.0.1:8080/v1/panic", nil)
		body, _ = ioutil.ReadAll(resp.Body)
		fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))
		fmt.Println()

		resp, _ = http.Get("http://127.0.0.1:8080/interrupt")
		body, _ = ioutil.ReadAll(resp.Body)
		fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))
		fmt.Println()

		resp, _ = http.Get("http://127.0.0.1:8080/global_interrupt")
		body, _ = ioutil.ReadAll(resp.Body)
		fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))
		fmt.Println()

		resp, _ = http.Get("http://127.0.0.1:8080/native")
		body, _ = ioutil.ReadAll(resp.Body)
		fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))
		fmt.Println()

		resp, _ = http.Get("http://127.0.0.1:8080/swagger/doc.json")
		body, _ = ioutil.ReadAll(resp.Body)
		fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))
		fmt.Println()

		resp, _ = http.Get("http://127.0.0.1:8080/wild_1/anything")
		body, _ = ioutil.ReadAll(resp.Body)
		fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))
		fmt.Println()

		resp, _ = http.Get("http://127.0.0.1:8080/wild_2/anything")
		body, _ = ioutil.ReadAll(resp.Body)
		fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))
		fmt.Println()

		resp, _ = http.Get("http://127.0.0.1:8080/wild_3/anything")
		body, _ = ioutil.ReadAll(resp.Body)
		fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))
		fmt.Println()

		resp, _ = http.Get("http://127.0.0.1:8080/v1/namespaces/default/pods/joke")
		body, _ = ioutil.ReadAll(resp.Body)
		fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))
		fmt.Println()

		resp, _ = http.Post("http://127.0.0.1:8080/empty", "", nil)
		body, _ = ioutil.ReadAll(resp.Body)
		fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))
		fmt.Println()

		resp, _ = http.Get("http://127.0.0.1:8080/empty")
		body, _ = ioutil.ReadAll(resp.Body)
		fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))
		fmt.Println()

		c.Stop(context.TODO())

		time.Sleep(time.Millisecond * 50)
	}

	prepare := func(c SpringWeb.WebContainer) SpringWeb.WebContainer {

		l := list.New()
		f2 := NewNumberFilter(2, l)
		f5 := NewNumberFilter(5, l)
		f7 := NewNumberFilter(7, l)

		s := NewService()

		c.GetMapping("/get", s.Get, f5).Swagger("").
			WithDescription("get").
			AddParam(spec.QueryParam("key")).
			WithConsumes(SpringWeb.MIMEApplicationForm).
			WithProduces(SpringWeb.MIMEApplicationJSON).
			RespondsWith(http.StatusOK, spec.NewResponse().
				WithSchema(spec.StringProperty()).
				AddExample(SpringWeb.MIMEApplicationJSON, 2))

		c.GetMapping("/global_interrupt", s.Get)
		c.GetMapping("/interrupt", s.Get, f5, &InterruptFilter{})

		// 障眼法
		r := c.Route("/v1", f2, f7)
		{
			r.PostMapping("/set", s.Set).Swagger("").
				WithDescription("set").
				//WithConsumes(SpringWeb.MIMEApplicationForm).
				WithConsumes(SpringWeb.MIMEApplicationJSON).
				//AddParam(spec.QueryParam("name")).
				//AddParam(spec.QueryParam("age")).
				AddParam(&spec.Parameter{
					ParamProps: spec.ParamProps{
						Name:   "body",
						In:     "body",
						Schema: spec.RefSchema("#/definitions/Set"),
					},
				}).
				RespondsWith(http.StatusOK, nil)

			r.Request(SpringWeb.MethodGetPost, "/panic", SpringWeb.FUNC(s.Panic))

			r.GetMapping("/namespaces/:namespace/pods/:pod", func(webCtx SpringWeb.WebContext) {
				assert.Equal(t, "default", webCtx.PathParam("namespace"))
				assert.Equal(t, "joke", webCtx.PathParam("pod"))
			})
		}

		c.GetMapping("/wild_1/*", func(webCtx SpringWeb.WebContext) {
			assert.Equal(t, "anything", webCtx.PathParam("*"))
			assert.Equal(t, []string{"*"}, webCtx.PathParamNames())
			assert.Equal(t, []string{"anything"}, webCtx.PathParamValues())
			webCtx.JSON(http.StatusOK, webCtx.PathParam("*"))
		})

		c.GetMapping("/wild_2/*none", func(webCtx SpringWeb.WebContext) {
			assert.Equal(t, "anything", webCtx.PathParam("*"))
			assert.Equal(t, "anything", webCtx.PathParam("none"))
			assert.Equal(t, []string{"*"}, webCtx.PathParamNames())
			assert.Equal(t, []string{"anything"}, webCtx.PathParamValues())
			webCtx.JSON(http.StatusOK, webCtx.PathParam("*"))
		})

		c.GetMapping("/wild_3/{*}", func(webCtx SpringWeb.WebContext) {
			assert.Equal(t, "anything", webCtx.PathParam("*"))
			assert.Equal(t, []string{"*"}, webCtx.PathParamNames())
			assert.Equal(t, []string{"anything"}, webCtx.PathParamValues())
			webCtx.JSON(http.StatusOK, webCtx.PathParam("*"))
		})

		c.Request(SpringWeb.MethodGetPost, "/empty", SpringWeb.BIND(s.Empty))

		return c
	}

	// 创建 gin 容器
	ginServer := func() SpringWeb.WebContainer {
		c := SpringGin.NewContainer(cfg)

		// 使用 gin 原生的中间件
		fLogger := SpringGin.Filter(gin.Logger())
		c.SetLoggerFilter(fLogger)

		// 使用 gin 原生的中间件
		fRecover := SpringGin.Filter(gin.Recovery())
		c.SetRecoveryFilter(fRecover)

		return c
	}

	t.Run("gin no route", func(t *testing.T) {
		testRun(ginServer())
	})

	t.Run("gin wild route", func(t *testing.T) {
		c := ginServer()

		c.HandleGet("/native", SpringGin.Gin(func(ctx *gin.Context) {
			ctx.String(http.StatusOK, "gin")
		}))

		testRun(prepare(c))
	})

	// 创建 echo 容器
	echoServer := func() SpringWeb.WebContainer {
		c := SpringEcho.NewContainer(cfg)

		// 使用 echo 原生的中间件
		fLogger := SpringEcho.Filter(middleware.Logger())
		c.SetLoggerFilter(fLogger)

		// 使用 echo 原生的中间件
		fRecover := SpringEcho.Filter(middleware.Recover())
		c.SetRecoveryFilter(fRecover)

		return c
	}

	t.Run("echo no route", func(t *testing.T) {
		testRun(echoServer())
	})

	t.Run("echo wild route", func(t *testing.T) {
		c := echoServer()

		c.HandleGet("/native", SpringEcho.Echo(func(ctx echo.Context) error {
			return ctx.String(http.StatusOK, "echo")
		}))

		testRun(prepare(c))
	})
}

func TestEchoServer(t *testing.T) {

	// 创建 Echo 服务器
	server := func() *echo.Echo {

		e := echo.New()
		e.HideBanner = true

		// echo 的全局中间件在路由未注册时仍然可用
		e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				fmt.Println("echo::use::0")
				defer fmt.Println("echo::use::1")
				return next(c)
			}
		})

		return e
	}

	testRun := func(e *echo.Echo) {

		go func() {
			address := ":8080"
			err := e.Start(address)
			fmt.Println("exit http server on", address, "return", err)
		}()

		time.Sleep(100 * time.Millisecond)

		resp, _ := http.Get("http://127.0.0.1:8080/echo")
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))

		_ = e.Shutdown(context.Background())
		time.Sleep(100 * time.Millisecond)
	}

	t.Run("no route", func(t *testing.T) {
		testRun(server())
	})

	t.Run("wild route", func(t *testing.T) {
		e := server()

		// 配合 echo 框架使用
		e.GET("/*", SpringEcho.HandlerWrapper(
			SpringWeb.FUNC(func(webCtx SpringWeb.WebContext) {
				assert.Equal(t, "echo", webCtx.PathParam("*"))
				assert.Equal(t, []string{"*"}, webCtx.PathParamNames())
				assert.Equal(t, []string{"echo"}, webCtx.PathParamValues())
				webCtx.JSON(http.StatusOK, map[string]string{
					"a": "1",
				})
			}), "", nil))

		testRun(e)
	})
}

func TestGinServer(t *testing.T) {

	// 创建 gin 服务器
	server := func() *gin.Engine {
		g := gin.New()

		// gin 的全局中间件在路由未注册时仍然可用
		g.Use(func(c *gin.Context) {
			fmt.Println("gin::use::0")
			defer fmt.Println("gin::use::1")
			c.Next()
		})

		return g
	}

	testRun := func(g *gin.Engine) {

		httpServer := &http.Server{
			Addr:    ":8080",
			Handler: g,
		}

		go func() {
			err := httpServer.ListenAndServe()
			fmt.Println("exit http server on", httpServer.Addr, "return", err)
		}()

		time.Sleep(100 * time.Millisecond)

		resp, _ := http.Get("http://127.0.0.1:8080/gin")
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))

		_ = httpServer.Shutdown(context.Background())
		time.Sleep(100 * time.Millisecond)
	}

	t.Run("no route", func(t *testing.T) {
		testRun(server())
	})

	t.Run("wild route", func(t *testing.T) {
		g := server()

		// 配合 gin 框架使用
		g.GET("/*"+SpringWeb.DefaultWildCardName, SpringGin.HandlerWrapper(
			SpringWeb.FUNC(func(webCtx SpringWeb.WebContext) {
				assert.Equal(t, "gin", webCtx.PathParam("*"))
				assert.Equal(t, []string{"*"}, webCtx.PathParamNames())
				assert.Equal(t, []string{"gin"}, webCtx.PathParamValues())
				webCtx.JSON(http.StatusOK, map[string]string{
					"a": "1",
				})
			}), SpringWeb.DefaultWildCardName, nil)...)

		testRun(g)
	})
}

func TestHttpServer(t *testing.T) {
	var server *http.Server

	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		// 暂未实现
	})

	go func() {
		address := ":8080"
		server = &http.Server{Addr: address}
		err := server.ListenAndServe()
		fmt.Println("exit http server on", address, "return", err)
	}()

	time.Sleep(100 * time.Millisecond)

	resp, _ := http.Get("http://127.0.0.1:8080/")
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))

	_ = server.Shutdown(context.Background())
	time.Sleep(100 * time.Millisecond)
}
