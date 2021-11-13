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

// Package SpringEcho 封装 github.com/labstack/echo 实现的 Web 框架
package SpringEcho

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-core/web"
	"github.com/labstack/echo"
)

func init() {
	echo.NotFoundHandler = func(c echo.Context) error {
		panic(echo.ErrNotFound)
	}
	echo.MethodNotAllowedHandler = func(c echo.Context) error {
		panic(echo.ErrMethodNotAllowed)
	}
}

type route struct {
	fn           web.Handler // Web 处理函数
	wildCardName string      // 通配符的名称
}

// Container echo 实现的 Web 容器
type Container struct {
	*web.AbstractContainer
	echoServer *echo.Echo
	routes     map[string]route // 记录所有通过 spring-echo 注册的路由
}

// NewContainer 创建 echo 实现的 Web 容器
func NewContainer(config web.ContainerConfig) *Container {
	c := &Container{}
	c.echoServer = echo.New()
	c.echoServer.HideBanner = true
	c.routes = make(map[string]route)
	c.AbstractContainer = web.NewAbstractContainer(config)
	return c
}

// Start 启动 Web 容器
func (c *Container) Start() error {

	if err := c.AbstractContainer.Start(); err != nil {
		return err
	}

	loggerFilter := c.GetLoggerFilter()
	recoveryFilter := new(recoveryFilter)

	// 添加容器级别的过滤器，这样在路由不存在时也会调用这些过滤器
	c.echoServer.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(echoCtx echo.Context) error {
			var webCtx web.Context

			// 如果 method+path 是 spring-echo 注册过的，那么可以保证 Context
			// 的 Handler 是准确的，否则是不准确的，请优先使用 spring-echo 注册路由。
			key := echoCtx.Request().Method + echoCtx.Path()
			if r, ok := c.routes[key]; ok {
				webCtx = NewContext(r.fn, r.wildCardName, echoCtx)
			} else {
				webCtx = NewContext(nil, echoCtx.Path(), echoCtx)
			}

			web.StartRecord(webCtx)
			defer func() {
				web.StopRecord(webCtx)
			}()

			web.StartReplay(webCtx)
			defer func() {
				web.StopReplay(webCtx)
			}()

			chain := web.NewDefaultFilterChain([]web.Filter{
				loggerFilter,
				recoveryFilter,
				web.HandlerFilter(Handler(next)),
			})
			chain.Next(WebContext(echoCtx))
			return nil
		}
	})

	urlPatterns, err := web.URLPatterns(c.GetFilters())
	if err != nil {
		return err
	}

	// 映射 Web 处理函数
	for _, mapper := range c.Mappers() {
		path, wildCardName := web.ToPathStyle(mapper.Path(), web.EchoPathStyle)
		fn := HandlerWrapper(mapper.Handler(), wildCardName, urlPatterns.Get(mapper.Path()))
		for _, method := range web.GetMethod(mapper.Method()) {
			c.echoServer.Add(method, path, fn)
			c.routes[method+path] = route{fn: mapper.Handler(), wildCardName: wildCardName}
		}
	}

	if cfg := c.Config(); cfg.EnableSSL {
		err = c.echoServer.StartTLS(c.Address(), cfg.CertFile, cfg.KeyFile)
	} else {
		err = c.echoServer.Start(c.Address())
	}

	log.Infof("exit echo server on %s return %s", c.Address(), cast.ToString(err))
	return err
}

// Stop 停止 Web 容器
func (c *Container) Stop(ctx context.Context) error {
	err := c.echoServer.Shutdown(ctx)
	log.Infof("shutdown echo server on %s return %s", c.Address(), cast.ToString(err))
	return err
}

// HandlerWrapper Web 处理函数包装器
func HandlerWrapper(fn web.Handler, wildCardName string, filters []web.Filter) echo.HandlerFunc {
	return func(echoCtx echo.Context) error {
		ctx := WebContext(echoCtx)
		if ctx == nil {
			ctx = NewContext(fn, wildCardName, echoCtx)
		}
		web.InvokeHandler(ctx, fn, filters)
		return nil
	}
}

/////////////////// handler //////////////////////

// echoHandler 封装 Echo 处理函数
type echoHandler echo.HandlerFunc

func (e echoHandler) Invoke(ctx web.Context) {
	if err := e(EchoContext(ctx)); err != nil {
		panic(err)
	}
}

func (e echoHandler) FileLine() (file string, line int, fnName string) {
	return util.FileLine(e)
}

// Handler 适配 echo 形式的处理函数
func Handler(fn echo.HandlerFunc) web.Handler { return echoHandler(fn) }

/////////////////// filter //////////////////////

// echoFilter 封装 Echo 中间件
type echoFilter echo.MiddlewareFunc

func (filter echoFilter) Invoke(ctx web.Context, chain web.FilterChain) {

	h := filter(func(echoCtx echo.Context) error {
		chain.Next(ctx)
		return nil
	})

	if err := h(EchoContext(ctx)); err != nil {
		panic(err)
	}
}

// Filter 适配 echo 形式的中间件函数
func Filter(fn echo.MiddlewareFunc) web.Filter { return echoFilter(fn) }

// recoveryFilter 适配 echo 的恢复过滤器
type recoveryFilter struct{}

func (f *recoveryFilter) Invoke(ctx web.Context, chain web.FilterChain) {

	defer func() {
		if err := recover(); err != nil {

			ctxLogger := log.Ctx(ctx.Context())
			ctxLogger.Error(err, "\n", string(debug.Stack()))

			httpE := web.HttpError{Code: http.StatusInternalServerError}
			switch e := err.(type) {
			case *echo.HTTPError:
				httpE.Code = e.Code
				if e.Code == http.StatusNotFound {
					httpE.Message = "404 page not found"
				} else if e.Code == http.StatusMethodNotAllowed {
					httpE.Message = "405 method not allowed"
				} else {
					httpE.Message = fmt.Sprintf("%v", e.Message)
				}
				httpE.Internal = e.Internal
			case *web.HttpError:
				httpE = *e
			case web.HttpError:
				httpE = e
			case error:
				httpE.Message = e.Error()
			default:
				httpE.Message = http.StatusText(httpE.Code)
				httpE.Internal = err
			}

			echoCtx := EchoContext(ctx)
			if !echoCtx.Response().Committed {
				if echoCtx.Request().Method == http.MethodHead { // Issue #608
					if err = echoCtx.NoContent(httpE.Code); err != nil {
						ctxLogger.Error(err)
					}
				} else {
					web.ErrorHandler(ctx, &httpE)
				}
			}
		}
	}()

	chain.Next(ctx)
}
