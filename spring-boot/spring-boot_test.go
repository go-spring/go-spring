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
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/elliotchance/redismock"
	"github.com/go-redis/redis"
	"github.com/go-spring/go-spring-parent/spring-utils"
	"github.com/go-spring/go-spring-web/spring-web"
	"github.com/go-spring/go-spring/spring-boot"
	_ "github.com/go-spring/go-spring/starter-echo"
	_ "github.com/go-spring/go-spring/starter-go-redis"
	_ "github.com/go-spring/go-spring/starter-go-redis-mock"
	_ "github.com/go-spring/go-spring/starter-mysql-gorm"
	_ "github.com/go-spring/go-spring/starter-mysql-mock"
	"github.com/jinzhu/gorm"
)

func init() {
	SpringBoot.RegisterBean(new(MyController)).Init(
		func(c *MyController) {
			SpringBoot.GetMapping("/ok", c.OK).
				ConditionOnProfile("test").
				ConditionOnMissingProperty("ok_enable")
		})

	SpringBoot.RegisterBean(new(MyRunner)).AsInterface((*SpringBoot.CommandLineRunner)(nil))
	SpringBoot.RegisterBeanFn(NewMyModule, "${message}").AsInterface((*SpringBoot.ApplicationEvent)(nil))

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
	os.Setenv(SpringBoot.SpringProfile, "test")
	SpringBoot.SetProperty("db.url", "root:root@/information_schema?charset=utf8&parseTime=True&loc=Local")
	SpringBoot.RunApplication("testdata/config/", "k8s:testdata/config/config-map.yaml")
}

////////////////// MyController ///////////////////

type MyController struct {
	RedisClient redis.Cmdable `autowire:""`
	DB          *gorm.DB      `autowire:""`
}

func (c *MyController) OK(ctx SpringWeb.WebContext) {

	err := c.RedisClient.Set("key", "ok", time.Second*10).Err()
	SpringUtils.Panic(err).When(err != nil)

	val, err := c.RedisClient.Get("key").Result()
	SpringUtils.Panic(err).When(err != nil)

	rows, err := c.DB.Table("ENGINES").Select("ENGINE").Rows()
	SpringUtils.Panic(err).When(err != nil)

	count := 0

	defer rows.Close()
	for rows.Next() {
		count++

		var engine string
		_ = rows.Scan(&engine)
		fmt.Println(engine)

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
		fmt.Println("get all properties:")
		for k, v := range ctx.GetAllProperties() {
			fmt.Println(k + "=" + fmt.Sprint(v))
		}
	})
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
	fmt.Println("MyModule start")

	var e *MyModule
	ctx.GetBean(&e)
	fmt.Printf("event: %+v\n", e)

	ctx.SafeGoroutine(Process)
}

func (m *MyModule) OnStopApplication(ctx SpringBoot.ApplicationContext) {
	fmt.Println("MyModule stop")
}

func Process() {
	defer SpringBoot.Exit()

	defer fmt.Println("go stop")
	fmt.Println("go start")

	var m *MyModule
	SpringBoot.GetBean(&m)
	fmt.Printf("process: %+v\n", m)

	time.Sleep(200 * time.Millisecond)

	if resp, err := http.Get("http://localhost:8080/ok"); err != nil {
		panic(err)
	} else {
		if body, e := ioutil.ReadAll(resp.Body); e != nil {
			panic(e)
		} else {
			fmt.Printf("resp code=%d body=%s\n", resp.StatusCode, string(body))
			if string(body) != "ok" {
				panic(errors.New("error"))
			}
		}
	}
}
