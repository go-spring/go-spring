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
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"go-spring.org/spring/gs"

	_ "go-spring.org/starter-echo"
)

func init() {
	gs.Provide(&Controller{})
	gs.Provide(func(c *Controller) *echo.Echo {
		e := echo.New()
		e.HideBanner = true
		e.GET("/echo", c.Echo)
		return e
	})
}

type Controller struct{}

func (c *Controller) Echo(ctx echo.Context) error {
	return ctx.String(http.StatusOK, "Hello, echo!")
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
	resp, err := http.Get("http://localhost:9090/echo")
	if err != nil {
		fmt.Fprintln(os.Stderr, "request failed:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	fmt.Println("Response from server:", string(b))
	if string(b) != "Hello, echo!" {
		fmt.Fprintln(os.Stderr, "unexpected response:", string(b))
		os.Exit(1)
	}
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}
