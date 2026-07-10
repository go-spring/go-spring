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
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"go-spring.org/spring/gs"

	_ "go-spring.org/starter-hertz"
)

func init() {
	gs.Provide(&Controller{})
	gs.Provide(func(c *Controller) *server.Hertz {
		h := server.Default(server.WithHostPorts("127.0.0.1:9090"))
		h.GET("/echo", c.Echo)
		return h
	})
}

type Controller struct{}

func (c *Controller) Echo(ctx context.Context, r *app.RequestContext) {
	r.String(consts.StatusOK, "Hello, hertz!")
}

func main() {
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest()
	}()

	gs.Configure(func(app gs.App) {
		app.Property("spring.http.server.enabled", "false")
	}).Run()
}

func runTest() {
	resp, err := http.Get("http://127.0.0.1:9090/echo")
	if err != nil {
		fmt.Fprintln(os.Stderr, "request failed:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	fmt.Println("Response from server:", string(b))
	if string(b) != "Hello, hertz!" {
		fmt.Fprintln(os.Stderr, "unexpected response:", string(b))
		os.Exit(1)
	}
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}
