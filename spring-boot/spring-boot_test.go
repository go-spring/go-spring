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

package SpringBoot_test

import (
	"container/list"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/elliotchance/redismock"
	"github.com/go-redis/redis"
	"github.com/go-spring/go-spring-parent/spring-logger"
	"github.com/go-spring/go-spring-parent/spring-utils"
	"github.com/go-spring/go-spring-web/spring-echo"
	"github.com/go-spring/go-spring-web/spring-web"
	"github.com/go-spring/go-spring/spring-boot"
	"github.com/go-spring/go-spring/spring-core"
	_ "github.com/go-spring/go-spring/starter-go-redis"
	_ "github.com/go-spring/go-spring/starter-go-redis-mock"
	_ "github.com/go-spring/go-spring/starter-mysql-gorm"
	_ "github.com/go-spring/go-spring/starter-mysql-mock"
	"github.com/go-spring/go-spring/starter-web"
	"github.com/jinzhu/gorm"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

func init() {
	// SpringLogger.SetLogger(&SpringLogger.Console{})

	l := list.New()

	SpringBoot.RegisterNameBean("f2", NewNumberFilter(2, l))
	SpringBoot.RegisterNameBean("f5", NewNumberFilter(5, l))
	SpringBoot.RegisterNameBean("f7", NewNumberFilter(7, l))

	// 测试 Config 系列函数的功能
	{
		SpringBoot.ConfigWithName("config_f2", func(filter *NumberFilter) {
			fmt.Println("NumberFilter:", filter.n)
		}, "f2").ConditionOnPropertyValue("f2.enable", true)

		SpringBoot.ConfigWithName("config_f5", func(filter *NumberFilter, appName string) {
			fmt.Println("NumberFilter:", filter.n, "appName:", appName)
		}, "f5", "${spring.application.name}").After("config_f2")

		SpringBoot.Config(func(filter *NumberFilter) {
			fmt.Println("NumberFilter:", filter.n)
		}, "f7").Before("config_f2").ConditionOnPropertyValue("f7.enable", true)
	}

	// 全局函数设置整个应用的信息
	SpringWeb.Swagger().WithDescription("spring boot test")

	// 注册过滤器
	{
		SpringBoot.RegisterBean(new(SingleBeanFilter))
		SpringBoot.RegisterNameBean("server", NewStringFilter("server"))
		SpringBoot.RegisterNameBean("container", NewStringFilter("container"))
		SpringBoot.RegisterNameBean("router", NewStringFilter("router"))
		SpringBoot.RegisterNameBean("router//ok", NewStringFilter("router//ok"))
		SpringBoot.RegisterNameBean("router//echo", NewStringFilter("router//echo"))
	}

	// 使用 "router" 名称的过滤器，需要使用 SpringBoot.FilterBean 封装，为了编译器能够进行类型检查
	r := SpringBoot.Route("/api", SpringBoot.FilterBean("router", (*SingleBeanFilter)(nil)))

	// 接受简单函数，可以使用 SpringBoot.Filter 封装，进而增加可用条件
	r.GetMapping("/func", func(ctx SpringWeb.WebContext) {
		ctx.String(http.StatusOK, "func() return ok")
	}, SpringBoot.Filter(SpringEcho.Filter(middleware.KeyAuth(
		func(key string, context echo.Context) (bool, error) {
			return key == "key_auth", nil
		}))).ConditionOnPropertyValue("key_auth", true),
	)

	// 接受类型方法，也可以不使用 SpringBoot.Filter 封装
	r.GET("/method", (*MyController).Method, SpringEcho.Filter(
		func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(echoCtx echo.Context) error {
				webCtx := SpringEcho.WebContext(echoCtx)
				webCtx.LogInfo("call method")
				return nil
			}
		}))

	SpringBoot.RegisterBean(new(MyController)).Init(func(c *MyController) {

		// 接受对象方法
		r.GetMapping("/ok", c.OK, SpringBoot.FilterBean("router//ok")).
			ConditionOnProfile("test").
			Swagger(). // 设置接口的信息
			WithDescription("ok")

		// 该接口不会注册，因为没有匹配的端口
		r.GetMapping("/nil", c.OK).OnPorts(9999)

		// 注意这个接口不和任何 Router 绑定
		SpringBoot.GetBinding("/echo", c.Echo, SpringBoot.FilterBean("router//echo")).
			Swagger(). // 设置接口的信息
			WithDescription("echo")
	})

	SpringBoot.RegisterBean(new(MyRunner))
	SpringBoot.RegisterBeanFn(NewMyModule, "${message}")

	SpringBoot.RegisterBean(func(mock sqlmock.Sqlmock) {
		mock.ExpectQuery("SELECT ENGINE FROM `ENGINES`").WillReturnRows(
			mock.NewRows([]string{"ENGINE"}).AddRow("sql-mock"),
		)
	})

	SpringBoot.RegisterBean(func(mock *redismock.ClientMock) {
		mock.On("Set", "key", "ok", time.Second*10).Return(redis.NewStatusResult("", nil))
		mock.On("Get", "key").Return(redis.NewStringResult("ok", nil))
	})

	SpringBoot.RegisterBeanFn(func(config WebStarter.WebServerConfig) SpringWeb.WebContainer {
		cfg := SpringWeb.ContainerConfig{Port: config.Port}
		c := SpringEcho.NewContainer(cfg)
		c.AddFilter(SpringBoot.FilterBean("container"))
		return c
	})

	SpringBoot.RegisterBeanFn(func() *SpringWeb.WebServer {
		return SpringWeb.NewWebServer().AddFilter(
			SpringBoot.FilterBean("server"),
		)
	})
}

func TestRunApplication(t *testing.T) {

	// 配置文件里面也指定了 spring.profile 的值
	// _ = os.Setenv(SpringBoot.SpringProfile, "test")

	// 过滤系统环境变量
	SpringBoot.ExpectSysProperties("GOPATH")

	// 设置过滤器是否启用
	SpringBoot.SetProperty("key_auth", false)

	SpringBoot.SetProperty("db.url", "root:root@/information_schema?charset=utf8&parseTime=True&loc=Local")

	configLocations := []string{
		"testdata/config/", "k8s:testdata/config/config-map.yaml",
	}

	// 等效 SpringBoot.RunApplication(configLocations...)
	SpringBoot.NewApplication().Run(configLocations...)
}

///////////////////// filter ////////////////////////

type SingleBeanFilter struct {
	_ SpringWeb.Filter `export:""`
}

func (f *SingleBeanFilter) Invoke(ctx SpringWeb.WebContext, chain SpringWeb.FilterChain) {
	ctx.LogInfo("::SingleBeanFilter")
	chain.Next(ctx)
}

type NumberFilter struct {
	_ SpringWeb.Filter `export:""`

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
		ctx.LogInfo("::after", f.n)
		f.l.PushBack(f.n)
	}()

	ctx.LogInfo("::before", f.n)
	f.l.PushBack(f.n)

	chain.Next(ctx)
}

type StringFilter struct {
	_ SpringWeb.Filter `export:""`

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

////////////////// MyController ///////////////////

type MyController struct {
	RedisClient redis.Cmdable                 `autowire:""`
	DB          *gorm.DB                      `autowire:""`
	AppCtx      SpringBoot.ApplicationContext `autowire:""`
}

type EchoRequest struct {
	Str string `query:"str"`
}

type EchoResponse struct {
	Echo string `json:"echo"`
}

func (c *MyController) Echo(request EchoRequest) *EchoResponse {
	return &EchoResponse{"echo " + request.Str}
}

func (c *MyController) Method(ctx SpringWeb.WebContext) {
	ctx.String(http.StatusOK, "method() return ok")
}

func (c *MyController) OK(ctx SpringWeb.WebContext) {

	err := c.RedisClient.Set("key", "ok", time.Second*10).Err()
	SpringUtils.Panic(err).When(err != nil)

	val, err := c.RedisClient.Get("key").Result()
	SpringUtils.Panic(err).When(err != nil)

	rows, err := c.DB.Table("ENGINES").Select("ENGINE").Rows()
	SpringUtils.Panic(err).When(err != nil)
	defer func() { _ = rows.Close() }()

	count := 0

	for rows.Next() {
		count++

		var engine string
		_ = rows.Scan(&engine)
		SpringLogger.Info(engine)

		if engine != "sql-mock" {
			panic(errors.New("error"))
		}
	}

	if count != 1 {
		panic(errors.New("error"))
	}

	ctx.JSONBlob(200, []byte(val))
}

////////////////// MyRunner ///////////////////

type MyRunner struct {
	_ SpringBoot.CommandLineRunner `export:""`
}

func (_ *MyRunner) Run(ctx SpringBoot.ApplicationContext) {

	ctx.SafeGoroutine(func() {
		SpringLogger.Trace("get all properties:")
		for k, v := range ctx.GetProperties() {
			SpringLogger.Tracef("%v=%v", k, v)
		}
		SpringLogger.Info("exit right now in MyRunner::Run")
	})

	fn := func(ctx SpringBoot.ApplicationContext, version string) {
		if version != "v0.0.1" {
			panic(errors.New("error"))
		}
	}
	_ = ctx.Run(fn, "1:${version:=v0.0.1}").On(SpringCore.ConditionOnProfile("test"))

	ctx.SafeGoroutine(func() {
		defer SpringLogger.Info("exit after waiting in MyRunner::Run")

		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Context().Done():
				return
			case <-ticker.C:
				SpringLogger.Info("MyRunner::Run")
			}
		}
	})
}

////////////////// MyModule ///////////////////

type MyModule struct {
	_ SpringBoot.ApplicationEvent `export:""`

	msg string
}

func NewMyModule(msg string) *MyModule {
	return &MyModule{
		msg: msg,
	}
}

func (m *MyModule) OnStartApplication(ctx SpringBoot.ApplicationContext) {
	SpringLogger.Info("MyModule start")

	var e *MyModule
	ctx.GetBean(&e)
	SpringLogger.Infof("event: %+v", e)

	ctx.SafeGoroutine(Process)
}

func (m *MyModule) OnStopApplication(ctx SpringBoot.ApplicationContext) {
	SpringLogger.Info("MyModule stop")
}

func Process() {
	defer SpringBoot.Exit()

	defer SpringLogger.Info("go stop")
	SpringLogger.Info("go start")

	var m *MyModule
	SpringBoot.GetBean(&m)
	SpringLogger.Infof("process: %+v", m)

	time.Sleep(200 * time.Millisecond)

	if resp, err := http.Get("http://localhost:8080/api/ok"); err != nil {
		panic(err)
	} else {
		if body, e := ioutil.ReadAll(resp.Body); e != nil {
			panic(e)
		} else {
			SpringLogger.Infof("resp code=%d body=%s", resp.StatusCode, string(body))
			if string(body) != "ok" {
				panic(errors.New("error"))
			}
		}
	}

	if resp, err := http.Get("http://127.0.0.1:8080/swagger/doc.json"); err != nil {
		panic(err)
	} else {
		if body, e := ioutil.ReadAll(resp.Body); e != nil {
			panic(e)
		} else {
			SpringLogger.Infof("resp code=%d body=%s", resp.StatusCode, string(body))
		}
	}

	if resp, err := http.Get("http://localhost:8080/echo?str=echo"); err != nil {
		panic(err)
	} else {
		if body, e := ioutil.ReadAll(resp.Body); e != nil {
			panic(e)
		} else {
			SpringLogger.Infof("resp code=%d body=%s(echo add a \\n on text end)", resp.StatusCode, string(body))
			if string(body) != "{\"Code\":0,\"Msg\":\"SUCCESS\",\"Err\":\"\",\"Data\":{\"echo\":\"echo echo\"}}\n" {
				panic(errors.New("error"))
			}
		}
	}

	if req, err := http.NewRequest("GET", "http://localhost:8080/api/func", nil); err != nil {
		panic(err)
	} else {
		auth := middleware.DefaultKeyAuthConfig.AuthScheme + " " + "key_auth"
		req.Header.Set(echo.HeaderAuthorization, auth)
		if resp, e := http.DefaultClient.Do(req); e != nil {
			panic(e)
		} else {
			if body, e0 := ioutil.ReadAll(resp.Body); e0 != nil {
				panic(e0)
			} else {
				SpringLogger.Infof("resp code=%d body=%s", resp.StatusCode, string(body))
				if string(body) != "ok" {
					panic(errors.New("error"))
				}
			}
		}
	}
}
