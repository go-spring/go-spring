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

// 封装 github.com/labstack/echo 实现的 Web 框架
package SpringEcho

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/go-spring/spring-logger"
	"github.com/go-spring/spring-utils"
	"github.com/go-spring/spring-web"
	"github.com/labstack/echo"
)

type route struct {
	fn           SpringWeb.Handler // Web 处理函数
	wildCardName string            // 通配符的名称
}

// Container echo 实现的 Web 容器
type Container struct {
	*SpringWeb.AbstractContainer
	echoServer *echo.Echo
	routes     map[string]route // 记录所有通过 spring-echo 注册的路由
}

// NewContainer 创建 echo 实现的 Web 容器
func NewContainer(config SpringWeb.ContainerConfig) *Container {
	c := &Container{}
	c.echoServer = echo.New()
	c.echoServer.HideBanner = true
	c.routes = make(map[string]route)
	c.AbstractContainer = SpringWeb.NewAbstractContainer(config)
	return c
}

// Start 启动 Web 容器
func (c *Container) Start() error {

	if err := c.AbstractContainer.Start(); err != nil {
		return err
	}

	var cFilters []SpringWeb.Filter
	{
		if loggerFilter := c.GetLoggerFilter(); loggerFilter != nil {
			cFilters = append(cFilters, loggerFilter)
		} else {
			cFilters = append(cFilters, SpringWeb.LoggerFilter)
		}

		cFilters = append(cFilters, &recoveryFilter{})
		cFilters = append(cFilters, c.GetFilters()...)
	}

	// 添加容器级别的过滤器，这样在路由不存在时也会调用这些过滤器
	c.echoServer.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(echoCtx echo.Context) error {

			// 如果 method+path 是 spring-echo 注册过的，那么可以保证 Context
			// 的 Handler 是准确的，否则是不准确的，请优先使用 spring-echo 注册路由。
			key := echoCtx.Request().Method + echoCtx.Path()
			if r, ok := c.routes[key]; ok {
				NewContext(r.fn, r.wildCardName, echoCtx)
			} else {
				NewContext(nil, echoCtx.Path(), echoCtx)
			}

			filters := append(cFilters, SpringWeb.HandlerFilter(Handler(next)))
			chain := SpringWeb.NewDefaultFilterChain(filters)
			chain.Next(WebContext(echoCtx))
			return nil
		}
	})

	// 映射 Web 处理函数
	for _, mapper := range c.Mappers() {
		c.PrintMapper(mapper)

		path, wildCardName := SpringWeb.ToPathStyle(mapper.Path(), SpringWeb.EchoPathStyle)
		fn := HandlerWrapper(mapper.Handler(), wildCardName, mapper.Filters())

		for _, method := range SpringWeb.GetMethod(mapper.Method()) {
			c.echoServer.Add(method, path, fn)
			c.routes[method+path] = route{fn: mapper.Handler(), wildCardName: wildCardName}
		}
	}

	var err error

	if cfg := c.Config(); cfg.EnableSSL {
		err = c.echoServer.StartTLS(c.Address(), cfg.CertFile, cfg.KeyFile)
	} else {
		err = c.echoServer.Start(c.Address())
	}

	SpringLogger.Infof("exit echo server on %s return %s", c.Address(), SpringUtils.ErrorToString(err))
	return err
}

// Stop 停止 Web 容器
func (c *Container) Stop(ctx context.Context) error {
	err := c.echoServer.Shutdown(ctx)
	SpringLogger.Infof("shutdown echo server on %s return %s", c.Address(), SpringUtils.ErrorToString(err))
	return err
}

// HandlerWrapper Web 处理函数包装器
func HandlerWrapper(fn SpringWeb.Handler, wildCardName string, filters []SpringWeb.Filter) echo.HandlerFunc {
	return func(echoCtx echo.Context) error {
		ctx := WebContext(echoCtx)
		if ctx == nil {
			ctx = NewContext(fn, wildCardName, echoCtx)
		}
		SpringWeb.InvokeHandler(ctx, fn, filters)
		return nil
	}
}

/////////////////// handler //////////////////////

// echoHandler 封装 Echo 处理函数
type echoHandler echo.HandlerFunc

func (e echoHandler) Invoke(ctx SpringWeb.Context) {
	if err := e(EchoContext(ctx)); err != nil {
		panic(err)
	}
}

func (e echoHandler) FileLine() (file string, line int, fnName string) {
	return SpringUtils.FileLine(e)
}

// Handler 适配 echo 形式的处理函数
func Handler(fn echo.HandlerFunc) SpringWeb.Handler { return echoHandler(fn) }

/////////////////// filter //////////////////////

// echoFilter 封装 Echo 中间件
type echoFilter echo.MiddlewareFunc

func (filter echoFilter) Invoke(ctx SpringWeb.Context, chain SpringWeb.FilterChain) {

	h := filter(func(echoCtx echo.Context) error {
		chain.Next(ctx)
		return nil
	})

	if err := h(EchoContext(ctx)); err != nil {
		panic(err)
	}
}

// Filter 适配 echo 形式的中间件函数
func Filter(fn echo.MiddlewareFunc) SpringWeb.Filter { return echoFilter(fn) }

// recoveryFilter 适配 echo 的恢复过滤器
type recoveryFilter struct{}

func (f *recoveryFilter) Invoke(ctx SpringWeb.Context, chain SpringWeb.FilterChain) {

	defer func() {
		if err := recover(); err != nil {

			ctxLogger := SpringLogger.WithContext(ctx.Context())
			ctxLogger.Error(err, "\n", string(debug.Stack()))

			httpE := SpringWeb.HttpError{Code: http.StatusInternalServerError}
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
			case *SpringWeb.HttpError:
				httpE = *e
			case SpringWeb.HttpError:
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
					if err := echoCtx.NoContent(httpE.Code); err != nil {
						ctxLogger.Error(err)
					}
				} else {
					SpringWeb.ErrorHandler(ctx, &httpE)
				}
			}
		}
	}()

	chain.Next(ctx)
}
