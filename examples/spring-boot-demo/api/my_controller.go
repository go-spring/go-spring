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

package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis"
	"github.com/go-spring/examples/spring-boot-demo/filter"
	"github.com/go-spring/spring-boot"
	"github.com/go-spring/spring-echo"
	"github.com/go-spring/spring-logger"
	"github.com/go-spring/spring-utils"
	"github.com/go-spring/spring-web"
	"github.com/jinzhu/gorm"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"go.mongodb.org/mongo-driver/mongo"
)

func init() {

	// 使用 "router" 名称的过滤器，需要使用 SpringBoot.FilterBean 封装，为了编译器能够进行类型检查
	r := SpringBoot.Route("/api", SpringBoot.FilterBean(
		"router", (*filter.SingleBeanFilter)(nil)),
	)

	// 接受简单函数，可以使用 SpringBoot.Filter 封装，进而增加可用条件
	r.GetMapping("/func", func(ctx SpringWeb.WebContext) {
		ctx.String("func() return ok")
	}, SpringBoot.Filter(SpringEcho.Filter(middleware.KeyAuth(
		func(key string, context echo.Context) (bool, error) {
			return key == "key_auth", nil
		}))).ConditionOnPropertyValue("key_auth", true),
	)

	SpringBoot.RegisterBean(new(MyController)).Init(func(c *MyController) {

		// 接受对象方法
		r.GetMapping("/ok", c.OK, SpringBoot.FilterBean("router//ok")).
			ConditionOnProfile("test").
			Swagger(). // 设置接口的信息
			WithDescription("ok")

		// 注意这个接口不和任何 Router 绑定
		SpringBoot.GetBinding("/echo", c.Echo, SpringBoot.FilterBean("router//echo")).
			Swagger(). // 设置接口的信息
			WithDescription("echo")

		SpringBoot.Go(func(ctx context.Context) {
			defer func() { SpringLogger.Info("exit after waiting in ::Go") }()

			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					SpringLogger.Info("::Go")
				}
			}
		})
	})
}

type MyController struct {
	RedisClient redis.Cmdable                 `autowire:""`
	MongoClient *mongo.Client                 `autowire:"?"`
	DB          *gorm.DB                      `autowire:""`
	AppCtx      SpringBoot.ApplicationContext `autowire:""`
}

type EchoRequest struct {
	Str string `query:"str"`
}

type EchoResponse struct {
	Echo string `json:"echo"`
}

func (c *MyController) Echo(ctx context.Context, request *EchoRequest) *SpringWeb.RpcResult {
	if c.MongoClient != nil {
		fmt.Println(c.MongoClient.Database("db0").Name())
	}
	return SpringWeb.SUCCESS.Data(&EchoResponse{"echo " + request.Str})
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

	ctx.JSONBlob([]byte(val))
}
