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

// Package SpringGin 封装 github.com/gin-gonic/gin 实现的 Web 框架
package SpringGin

import (
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-core/web"
)

func init() {
	gin.SetMode(gin.ReleaseMode)
	binding.Validator = nil // 关闭 gin 的校验器
}

type route struct {
	handler  web.Handler // Web 处理函数
	path     string      // 注册时候的路径
	wildcard string      // 通配符的名称
}

// serverHandler gin 实现的 web 服务器
type serverHandler struct {
	engine *gin.Engine
	routes map[string]route
}

// New 创建 gin 实现的 web 服务器
func New(config web.ServerConfig) web.Server {
	h := new(serverHandler)
	h.engine = gin.New()
	h.engine.HandleMethodNotAllowed = true
	h.routes = make(map[string]route)
	return web.NewServer(config, h)
}

func (h *serverHandler) RecoveryFilter(errHandler web.ErrorHandler) web.Filter {
	return &recoveryFilter{errHandler: errHandler}
}

func (h *serverHandler) Start(s web.Server) error {

	// 添加服务器级别的过滤器，这样在路由不存在时也会调用这些过滤器
	h.engine.Use(func(ginCtx *gin.Context) {
		var webCtx web.Context

		// 如果 method+path 是 spring gin 注册过的，那么可以保证 Context
		// 的 Handler 是准确的，否则是不准确的，请优先使用 spring gin 注册路由。
		key := ginCtx.Request.Method + ginCtx.FullPath()
		if r, ok := h.routes[key]; ok {
			webCtx = newContext(r.handler, r.path, r.wildcard, ginCtx)
		} else {
			webCtx = newContext(nil, "", "", ginCtx)
		}

		// 流量录制
		web.StartRecord(webCtx)
		defer func() { web.StopRecord(webCtx) }()

		// 流量回放
		web.StartReplay(webCtx)
		defer func() { web.StopReplay(webCtx) }()

		ginCtx.Next()
	})

	urlPatterns, err := web.URLPatterns(s.Filters())
	if err != nil {
		return err
	}

	// 映射 Web 处理函数
	for _, mapper := range s.Mappers() {
		filters := urlPatterns.Get(mapper.Path())
		handlers := wrapperHandler(mapper.Handler(), filters)
		path, wildcard := web.ToPathStyle(mapper.Path(), web.GinPathStyle)
		for _, method := range web.GetMethod(mapper.Method()) {
			h.engine.Handle(method, path, handlers...)
			h.routes[method+path] = route{mapper.Handler(), mapper.Path(), wildcard}
		}
	}
	return nil
}

func (h *serverHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.engine.ServeHTTP(w, r)
}

// wrapperHandler Web 处理函数包装器
func wrapperHandler(fn web.Handler, filters []web.Filter) []gin.HandlerFunc {
	var handlers []gin.HandlerFunc

	// 封装过滤器
	for _, filter := range filters {
		f := filter // 避免延迟绑定
		handlers = append(handlers, func(ginCtx *gin.Context) {
			f.Invoke(WebContext(ginCtx), &ginFilterChain{ginCtx})
			ginCtx.Abort()
		})
	}

	// 封装 Web 处理函数
	handlers = append(handlers, func(ginCtx *gin.Context) {
		fn.Invoke(WebContext(ginCtx))
	})

	return handlers
}

/////////////////// handler //////////////////////

// ginHandler 封装 Gin 处理函数
type ginHandler gin.HandlerFunc

func (g ginHandler) Invoke(ctx web.Context) {
	g(GinContext(ctx))
}

func (g ginHandler) FileLine() (file string, line int, fnName string) {
	return util.FileLine(g)
}

// Handler 适配 gin 形式的处理函数
func Handler(fn gin.HandlerFunc) web.Handler { return ginHandler(fn) }

/////////////////// filter //////////////////////

// ginFilter 封装 Gin 中间件
type ginFilter gin.HandlerFunc

func (filter ginFilter) Invoke(ctx web.Context, _ web.FilterChain) {
	filter(GinContext(ctx))
}

// Filter Web Gin 中间件适配器
func Filter(fn gin.HandlerFunc) web.Filter { return ginFilter(fn) }

// ginFilterChain gin 适配的过滤器链条
type ginFilterChain struct {
	ginCtx *gin.Context
}

// Next 内部调用 gin.Context 对象的 Next 函数驱动链条向后执行
func (chain *ginFilterChain) Next(_ web.Context) { chain.ginCtx.Next() }

func (chain *ginFilterChain) Continue(_ web.Context) { chain.ginCtx.Next() }

// recoveryFilter 适配 gin 的恢复过滤器
type recoveryFilter struct {
	errHandler web.ErrorHandler
}

func (f *recoveryFilter) Invoke(webCtx web.Context, chain web.FilterChain) {

	defer func() {
		if err := recover(); err != nil {

			ctxLogger := log.Ctx(webCtx.Context())
			ctxLogger.Error(nil, err, "\n", string(debug.Stack()))

			// Check for a broken connection, as it is not really a
			// condition that warrants a panic stack trace.
			var brokenPipe bool
			if ne, ok := err.(*net.OpError); ok {
				if se, ok := ne.Err.(*os.SyscallError); ok {
					if strings.Contains(strings.ToLower(se.Error()), "broken pipe") || strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
						brokenPipe = true
					}
				}
			}

			ginCtx := GinContext(webCtx)
			if ginCtx != nil {
				ginCtx.Abort()
			}

			// If the connection is dead, we can't write a status to it.
			if brokenPipe {
				return
			}

			httpE := web.HttpError{Code: http.StatusInternalServerError}
			switch e := err.(type) {
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

			f.errHandler.Invoke(webCtx, &httpE)
		}
	}()

	chain.Next(webCtx)
}
