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
	"context"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-spring/spring-core/log"
	"github.com/go-spring/spring-core/util"
	"github.com/go-spring/spring-core/web"
)

func init() {
	gin.SetMode(gin.ReleaseMode)
	binding.Validator = nil // 关闭 gin 的校验器
}

type route struct {
	fn           web.Handler // Web 处理函数
	wildCardName string      // 通配符的名称
}

// Container gin 实现的 WebContainer
type Container struct {
	*web.AbstractContainer
	httpServer *http.Server
	ginEngine  *gin.Engine
	routes     map[string]route // 记录所有通过 spring gin 注册的路由
}

// NewContainer 创建 gin 实现的 WebContainer
func NewContainer(config web.ContainerConfig) *Container {
	c := &Container{}
	c.ginEngine = gin.New()
	c.ginEngine.HandleMethodNotAllowed = true
	c.routes = make(map[string]route)
	c.AbstractContainer = web.NewAbstractContainer(config)
	return c
}

// Start 启动 Web 容器
func (c *Container) Start() error {

	if err := c.AbstractContainer.Start(); err != nil {
		return err
	}

	var cFilters []web.Filter
	{
		if loggerFilter := c.GetLoggerFilter(); loggerFilter != nil {
			cFilters = append(cFilters, loggerFilter)
		} else {
			cFilters = append(cFilters, web.LoggerFilter)
		}

		cFilters = append(cFilters, &recoveryFilter{})
		cFilters = append(cFilters, c.GetFilters()...)
	}

	urlPatterns, err := web.URLPatterns(cFilters)
	if err != nil {
		return err
	}

	// 添加容器级别的过滤器，这样在路由不存在时也会调用这些过滤器
	c.ginEngine.Use(func(ginCtx *gin.Context) {

		// 如果 method+path 是 spring gin 注册过的，那么可以保证 Context
		// 的 Handler 是准确的，否则是不准确的，请优先使用 spring gin 注册路由。
		key := ginCtx.Request.Method + ginCtx.FullPath()
		if r, ok := c.routes[key]; ok {
			NewContext(r.fn, r.wildCardName, ginCtx)
		} else {
			NewContext(nil, ginCtx.FullPath(), ginCtx)
		}
	})

	//for _, filter := range cFilters {
	//	f := filter // 避免延迟绑定
	//	c.ginEngine.Use(func(ginCtx *gin.Context) {
	//		f.Invoke(WebContext(ginCtx), &ginFilterChain{ginCtx})
	//	})
	//}

	// 映射 Web 处理函数
	for _, mapper := range c.Mappers() {
		c.PrintMapper(mapper)

		path, wildCardName := web.ToPathStyle(mapper.Path(), web.GinPathStyle)
		handlers := HandlerWrapper(mapper.Handler(), wildCardName, urlPatterns.Get(mapper.Path()))

		for _, method := range web.GetMethod(mapper.Method()) {
			c.ginEngine.Handle(method, path, handlers...)
			c.routes[method+path] = route{
				fn:           mapper.Handler(),
				wildCardName: wildCardName,
			}
		}
	}

	cfg := c.Config()
	c.httpServer = &http.Server{
		Addr:         c.Address(),
		Handler:      c.ginEngine,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	log.Info("⇨ http server started on ", c.Address())

	if cfg.EnableSSL {
		err = c.httpServer.ListenAndServeTLS(cfg.CertFile, cfg.KeyFile)
	} else {
		err = c.httpServer.ListenAndServe()
	}

	log.Infof("exit gin server on %s return %s", c.Address(), util.Error(err))
	return err
}

// Stop 停止 Web 容器
func (c *Container) Stop(ctx context.Context) error {
	err := c.httpServer.Shutdown(ctx)
	log.Infof("shutdown gin server on %s return %s", c.Address(), util.Error(err))
	return err
}

// HandlerWrapper Web 处理函数包装器
func HandlerWrapper(fn web.Handler, wildCardName string, filters []web.Filter) []gin.HandlerFunc {
	var handlers []gin.HandlerFunc

	// 建立 Context 和 GinContext 之间的关联
	handlers = append(handlers, func(ginCtx *gin.Context) {
		if WebContext(ginCtx) == nil {
			NewContext(fn, wildCardName, ginCtx)
		}
	})

	// 封装过滤器
	for _, filter := range filters {
		f := filter // 避免延迟绑定
		handlers = append(handlers, func(ginCtx *gin.Context) {
			f.Invoke(WebContext(ginCtx), &ginFilterChain{ginCtx})
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

// recoveryFilter 适配 gin 的恢复过滤器
type recoveryFilter struct{}

func (f *recoveryFilter) Invoke(webCtx web.Context, chain web.FilterChain) {

	defer func() {
		if err := recover(); err != nil {

			ctxLogger := log.Ctx(webCtx.Context())
			ctxLogger.Error(err, "\n", string(debug.Stack()))

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
			ginCtx.Abort()

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

			web.ErrorHandler(webCtx, &httpE)
		}
	}()

	chain.Next(webCtx)
}
