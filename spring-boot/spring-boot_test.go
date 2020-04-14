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
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/elliotchance/redismock"
	"github.com/go-redis/redis"
	"github.com/go-spring/go-spring-parent/spring-logger"
	"github.com/go-spring/go-spring-parent/spring-utils"
	"github.com/go-spring/go-spring-web/spring-web"
	"github.com/go-spring/go-spring/spring-boot"
	"github.com/go-spring/go-spring/spring-core"
	_ "github.com/go-spring/go-spring/starter-echo"
	_ "github.com/go-spring/go-spring/starter-go-redis"
	_ "github.com/go-spring/go-spring/starter-go-redis-mock"
	_ "github.com/go-spring/go-spring/starter-mysql-gorm"
	_ "github.com/go-spring/go-spring/starter-mysql-mock"
	"github.com/jinzhu/gorm"
)

func init() {
	// SpringLogger.SetLogger(&SpringLogger.Console{})

	l := list.New()

	SpringBoot.RegisterNameBean("f2", NewNumberFilter(2, l)).Export((*SpringWeb.Filter)(nil))
	SpringBoot.RegisterNameBean("f5", NewNumberFilter(5, l)).Export((*SpringWeb.Filter)(nil))
	SpringBoot.RegisterNameBean("f7", NewNumberFilter(7, l)).Export((*SpringWeb.Filter)(nil))

	// 全局函数设置整个应用的信息
	SpringWeb.Swagger().WithDescription("spring boot test")

	SpringBoot.RegisterBean(new(MyController)).Init(func(c *MyController) {

		r := SpringBoot.Route("/api")
		{
			r.GET("/ok", c.OK).
				ConditionOnProfile("test").
				SetFilterNames("f2").
				Swagger(). // 设置接口的信息
				WithDescription("ok")
		}

		SpringBoot.GetMapping("/echo", SpringWeb.BIND(c.Echo)).
			SetFilterNames("f5").
			Swagger(). // 设置接口的信息
			WithDescription("echo")
	})

	SpringBoot.RegisterBean(new(MyRunner)).Export((*SpringBoot.CommandLineRunner)(nil))
	SpringBoot.RegisterBeanFn(NewMyModule, "${message}").Export((*SpringBoot.ApplicationEvent)(nil))

	SpringBoot.RegisterBean(func(mock sqlmock.Sqlmock) {
		mock.ExpectQuery("SELECT ENGINE FROM `ENGINES`").WillReturnRows(
			mock.NewRows([]string{"ENGINE"}).AddRow("sql-mock"),
		)
	})

	SpringBoot.RegisterBean(func(mock *redismock.ClientMock) {
		mock.On("Set", "key", "ok", time.Second*10).Return(redis.NewStatusResult("", nil))
		mock.On("Get", "key").Return(redis.NewStringResult("ok", nil))
	})
}

func TestRunApplication(t *testing.T) {

	// 配置文件里面也指定了 spring.profile 的值
	// _ = os.Setenv(SpringBoot.SpringProfile, "test")

	SpringBoot.SetProperty("db.url", "root:root@/information_schema?charset=utf8&parseTime=True&loc=Local")
	SpringBoot.RunApplication("testdata/config/", "k8s:testdata/config/config-map.yaml")
}

///////////////////// filter ////////////////////////

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

func (f *NumberFilter) Invoke(ctx SpringWeb.WebContext, chain *SpringWeb.FilterChain) {

	defer func() {
		ctx.LogInfo("::after", f.n)
		f.l.PushBack(f.n)
	}()

	ctx.LogInfo("::before", f.n)
	f.l.PushBack(f.n)

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

func (s *MyController) Echo(request EchoRequest) *EchoResponse {
	return &EchoResponse{"echo " + request.Str}
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
}

func (_ *MyRunner) Run(ctx SpringBoot.ApplicationContext) {
	ctx.SafeGoroutine(func() {
		SpringLogger.Trace("get all properties:")
		for k, v := range ctx.GetProperties() {
			SpringLogger.Tracef("%v=%v", k, v)
		}
	})

	fn := func(ctx SpringBoot.ApplicationContext, version string) {
		if version != "v0.0.1" {
			panic(errors.New("error"))
		}
	}
	ctx.Run(fn, "1:${version:=v0.0.1}").On(SpringCore.OnProfile("test"))
}

////////////////// MyModule ///////////////////

type MyModule struct {
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
}
