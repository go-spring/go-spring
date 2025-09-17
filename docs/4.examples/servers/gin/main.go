/*
 * Copyright 2025 The Go-Spring Authors.
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

package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-spring/spring-core/gs"
)

func init() {
	gin.SetMode(gin.ReleaseMode)
	gs.EnableSimpleHttpServer(false)

	gs.Object(&Controller{})
	gs.Provide(func(c *Controller) *gin.Engine {
		e := gin.Default()
		e.GET("/echo", c.Echo)
		return e
	})
}

type Controller struct{}

func (c *Controller) Echo(ctx *gin.Context) {
	ctx.String(http.StatusOK, "Hello, gin!")
}

func main() {
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")
	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest()
	}()
	gs.Run()
}

func runTest() {
	resp, _ := http.Get("http://localhost:9090/echo")
	b, _ := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	fmt.Println("Response from server:", string(b))
	gs.ShutDown()
}
