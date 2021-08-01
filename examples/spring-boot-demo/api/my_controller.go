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
	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/log"
	"github.com/go-spring/spring-core/web"
	"github.com/go-spring/spring-stl/util"
	"github.com/jinzhu/gorm"
	"go.mongodb.org/mongo-driver/mongo"
)

func init() {

	gs.GetMapping("/api/func", func(ctx web.Context) {
		ctx.String("func() return ok")
	})

	gs.Object(new(MyController)).Init(func(c *MyController) {

		gs.GetMapping("/api/ok", c.OK)
		gs.GetBinding("/api/echo", c.Echo)

		gs.Go(func(ctx context.Context) {
			defer func() { log.Info("exit after waiting in ::Go") }()

			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					log.Info("::Go")
				}
			}
		})
	})
}

type MyController struct {
	RedisClient redis.Cmdable `autowire:""`
	MongoClient *mongo.Client `autowire:"?"`
	DB          *gorm.DB      `autowire:""`
}

type EchoRequest struct {
	Str string `query:"str"`
}

type EchoResponse struct {
	Echo string `json:"echo"`
}

func (c *MyController) Echo(ctx context.Context, request *EchoRequest) *web.RpcResult {
	if c.MongoClient != nil {
		fmt.Println(c.MongoClient.Database("db0").Name())
	}
	return web.SUCCESS.Data(&EchoResponse{"echo " + request.Str})
}

func (c *MyController) OK(ctx web.Context) {

	err := c.RedisClient.Set("key", "ok", time.Second*10).Err()
	util.Panic(err).When(err != nil)

	val, err := c.RedisClient.Get("key").Result()
	util.Panic(err).When(err != nil)

	rows, err := c.DB.Table("ENGINES").Select("ENGINE").Rows()
	util.Panic(err).When(err != nil)
	defer func() { _ = rows.Close() }()

	count := 0

	for rows.Next() {
		count++

		var engine string
		_ = rows.Scan(&engine)
		log.Info(engine)

		if engine != "sql-mock" {
			panic(errors.New("error"))
		}
	}

	if count != 1 {
		panic(errors.New("error"))
	}

	ctx.JSONBlob([]byte(val))
}
