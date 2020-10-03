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

package SpringEcho

import (
	"context"
	"net/http"

	"github.com/go-spring/spring-logger"
	"github.com/go-spring/spring-utils"
	"github.com/go-spring/spring-web"
	"github.com/labstack/echo"
)

type route struct {
	fn           SpringWeb.Handler // Web 处理函数
	wildCardName string            // 通配符的名称
}

// Container 适配 echo 的 Web 容器
type Container struct {
	*SpringWeb.BaseWebContainer
	echoServer *echo.Echo
	routes     map[string]route // 记录所有通过 spring echo 注册的路由
}

// NewContainer Container 的构造函数
func NewContainer(config SpringWeb.ContainerConfig) *Container {
	c := &Container{
		BaseWebContainer: SpringWeb.NewBaseWebContainer(config),
		routes:           make(map[string]route),
	}
	return c
}

// Deprecated: Filter 机制可完美替代中间件机制，不再需要定制化
func (c *Container) SetEchoServer(e *echo.Echo) {
	c.echoServer = e
}

// Start 启动 Web 容器，非阻塞
func (c *Container) Start() {

	c.PreStart()

	// 使用默认的 echo 容器
	if c.echoServer == nil {
		e := echo.New()
		e.HideBanner = true
		c.echoServer = e
	}

	var cFilters []SpringWeb.Filter

	if f := c.GetLoggerFilter(); f != nil {
		cFilters = append(cFilters, f)
	}

	if f := c.GetRecoveryFilter(); f != nil {
		cFilters = append(cFilters, &recoveryFilterAdapter{})
	}

	cFilters = append(cFilters, c.GetFilters()...)

	// 添加容器级别的过滤器，这样在路由不存在时也会调用这些过滤器
	c.echoServer.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(echoCtx echo.Context) error {

			// 如果 method+path 是 spring echo 注册过的，那么可以保证 WebContext
			// 的 Handler 是准确的，否则是不准确的，请优先使用 spring echo 注册路由。
			key := echoCtx.Request().Method + echoCtx.Path()
			if r, ok := c.routes[key]; ok {
				NewContext(r.fn, r.wildCardName, echoCtx)
			} else {
				NewContext(nil, echoCtx.Path(), echoCtx)
			}

			filters := append(cFilters, SpringWeb.HandlerFilter(Echo(next)))
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
			c.routes[method+path] = route{
				fn:           mapper.Handler(),
				wildCardName: wildCardName,
			}
		}
	}

	// 启动 echo 容器
	go func() {
		var err error
		// TODO 应用 ReadTimeout 和 WriteTimeout。

		if cfg := c.Config(); cfg.EnableSSL {
			err = c.echoServer.StartTLS(c.Address(), cfg.CertFile, cfg.KeyFile)
		} else {
			err = c.echoServer.Start(c.Address())
		}

		if err != nil && err != http.ErrServerClosed {
			if fn := c.GetErrorCallback(); fn != nil {
				fn(err)
			}
		}

		SpringLogger.Infof("exit echo server on %s return %s", c.Address(), SpringUtils.ErrorToString(err))
	}()
}

// Stop 停止 Web 容器，阻塞
func (c *Container) Stop(ctx context.Context) {
	err := c.echoServer.Shutdown(ctx)
	SpringLogger.Infof("shutdown echo server on %s return %s", c.Address(), SpringUtils.ErrorToString(err))
}

// HandlerWrapper Web 处理函数包装器
func HandlerWrapper(fn SpringWeb.Handler, wildCardName string, filters []SpringWeb.Filter) echo.HandlerFunc {
	return func(echoCtx echo.Context) error {
		webCtx := WebContext(echoCtx)
		if webCtx == nil {
			webCtx = NewContext(fn, wildCardName, echoCtx)
		}
		SpringWeb.InvokeHandler(webCtx, fn, filters)
		return nil
	}
}

// recoveryFilterAdapter 对 echo 的恢复组件适配
type recoveryFilterAdapter struct {
}

func (f *recoveryFilterAdapter) Invoke(webCtx SpringWeb.WebContext, chain SpringWeb.FilterChain) {
	defer func() {
		if err := recover(); err != nil {
			httpError := err.(*echo.HTTPError)
			webCtx.Status(httpError.Code)
			webCtx.String("%d %s", httpError.Code, httpError.Message)
		}
	}()

	chain.Next(webCtx)
}

/////////////////// handler //////////////////////

// echoHandler 封装 Echo 处理函数
type echoHandler echo.HandlerFunc

func (e echoHandler) Invoke(ctx SpringWeb.WebContext) {
	if err := e(EchoContext(ctx)); err != nil {
		panic(err)
	}
}

func (e echoHandler) FileLine() (file string, line int, fnName string) {
	return SpringUtils.FileLine(e)
}

// Echo Web Echo 适配函数
func Echo(fn echo.HandlerFunc) SpringWeb.Handler {
	return echoHandler(fn)
}

/////////////////// filter //////////////////////

// echoFilter 封装 Echo 中间件
type echoFilter echo.MiddlewareFunc

func (filter echoFilter) Invoke(ctx SpringWeb.WebContext, chain SpringWeb.FilterChain) {

	h := filter(func(echoCtx echo.Context) error {
		chain.Next(ctx)
		return nil
	})

	if err := h(EchoContext(ctx)); err != nil {
		panic(err)
	}
}

// Filter Web Echo 中间件适配器
func Filter(fn echo.MiddlewareFunc) SpringWeb.Filter {
	return echoFilter(fn)
}
