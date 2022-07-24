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

// Package SpringEcho 封装 github.com/labstack/echo/v4 实现的 Web 框架
package SpringEcho

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-core/web"
	"github.com/labstack/echo/v4"
)

var (
	logger = log.GetLogger()
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
	handler  web.Handler
	path     string
	wildcard string
}

// serverHandler echo 实现的 web 服务器
type serverHandler struct {
	basePath string
	echo     *echo.Echo
	routes   map[string]route
}

// New 创建 echo 实现的 web 服务器
func New(config web.ServerConfig) web.Server {
	h := new(serverHandler)
	h.echo = echo.New()
	h.echo.HideBanner = true
	h.routes = make(map[string]route)
	h.basePath = config.BasePath
	return web.NewServer(config, h)
}

func (h *serverHandler) RecoveryFilter(errHandler web.ErrorHandler) web.Filter {
	return &recoveryFilter{errHandler: errHandler}
}

func (h *serverHandler) Start(s web.Server) error {

	// 添加服务器级别的过滤器，这样在路由不存在时也会调用这些过滤器
	h.echo.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(echoCtx echo.Context) error {
			var webCtx web.Context

			// 如果 method+path 是 spring-echo 注册过的，那么可以保证 Context
			// 的 Handler 是准确的，否则是不准确的，请优先使用 spring-echo 注册路由。
			key := echoCtx.Request().Method + echoCtx.Path()
			if r, ok := h.routes[key]; ok {
				webCtx = newContext(r.handler, r.path, r.wildcard, echoCtx)
			} else {
				webCtx = newContext(nil, "", "", echoCtx)
			}

			// 流量录制
			web.StartRecord(webCtx)
			defer func() { web.StopRecord(webCtx) }()

			// 流量回放
			web.StartReplay(webCtx)
			defer func() { web.StopReplay(webCtx) }()

			return next(EchoContext(webCtx))
		}
	})

	urlPatterns, err := web.URLPatterns(s.Filters())
	if err != nil {
		return err
	}

	// 映射 Web 处理函数
	for _, m := range s.Mappers() {
		filters := urlPatterns.Get(m.Path())
		handler := wrapperHandler(m.Handler(), filters)
		path, wildCardName := web.ToPathStyle(m.Path(), web.EchoPathStyle)
		{
			path = h.basePath + path
			if f, ok := m.Handler().(*web.FileHandler); ok {
				f.Prefix = h.basePath + f.Prefix
			}
		}
		for _, method := range web.GetMethod(m.Method()) {
			h.echo.Add(method, path, handler)
			h.routes[method+path] = route{m.Handler(), m.Path(), wildCardName}
		}
	}
	return nil
}

func (h *serverHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.echo.ServeHTTP(w, r)
}

// wrapperHandler Web 处理函数包装器
func wrapperHandler(fn web.Handler, filters []web.Filter) echo.HandlerFunc {
	return func(c echo.Context) error {
		filters = append(filters, web.HandlerFilter(fn))
		web.NewFilterChain(filters).Next(WebContext(c))
		return nil
	}
}

/////////////////// handler //////////////////////

// echoHandler 封装 Echo 处理函数
type echoHandler echo.HandlerFunc

func (e echoHandler) Invoke(ctx web.Context) {
	err := e(EchoContext(ctx))
	util.Panic(err).When(err != nil)
}

func (e echoHandler) FileLine() (file string, line int, fnName string) {
	return util.FileLine(e)
}

// Handler 适配 echo 形式的处理函数
func Handler(fn echo.HandlerFunc) web.Handler {
	return echoHandler(fn)
}

/////////////////// filter //////////////////////

// echoFilter 封装 Echo 中间件
type echoFilter echo.MiddlewareFunc

func (filter echoFilter) Invoke(ctx web.Context, chain web.FilterChain) {
	next := func(echoCtx echo.Context) error {
		chain.Next(ctx)
		return nil
	}
	err := filter(next)(EchoContext(ctx))
	util.Panic(err).When(err != nil)
}

// Filter 适配 echo 形式的中间件函数
func Filter(fn echo.MiddlewareFunc) web.Filter {
	return echoFilter(fn)
}

// recoveryFilter 适配 echo 的恢复过滤器
type recoveryFilter struct {
	errHandler web.ErrorHandler
}

func (f *recoveryFilter) Invoke(ctx web.Context, chain web.FilterChain) {

	defer func() {
		if err := recover(); err != nil {

			ctxLogger := logger.WithContext(ctx.Context())
			ctxLogger.Error(nil, err, "\n", string(debug.Stack()))

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
			if echoCtx == nil {
				f.errHandler.Invoke(ctx, &httpE)
				return
			}
			if echoCtx.Response().Committed {
				return
			}
			if echoCtx.Request().Method != http.MethodHead { // Issue #608
				f.errHandler.Invoke(ctx, &httpE)
				return
			}
			if err = echoCtx.NoContent(httpE.Code); err != nil {
				ctxLogger.Error(nil, err)
			}
		}
	}()

	chain.Next(ctx)
}
