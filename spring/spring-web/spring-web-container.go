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

package SpringWeb

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-spring/spring-logger"
	"github.com/go-spring/spring-utils"
	"github.com/swaggo/http-swagger"
)

// HandlerFunc 标准 Web 处理函数
type HandlerFunc func(WebContext)

// Handler Web 处理接口
type Handler interface {
	// Invoke 响应函数
	Invoke(WebContext)

	// FileLine 获取用户函数的文件名、行号以及函数名称
	FileLine() (file string, line int, fnName string)
}

// ContainerConfig Web 容器配置
type ContainerConfig struct {
	IP        string // 监听 IP
	Port      int    // 监听端口
	EnableSSL bool   // 使用 SSL
	KeyFile   string // SSL 证书
	CertFile  string // SSL 秘钥

	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// WebContainer Web 容器
type WebContainer interface {
	// WebMapping 路由表
	WebMapping

	// Config 获取 Web 容器配置
	Config() ContainerConfig

	// GetFilters 返回过滤器列表
	GetFilters() []Filter

	// ResetFilters 重新设置过滤器列表
	ResetFilters(filters []Filter)

	// AddFilter 添加过滤器
	AddFilter(filter ...Filter)

	// GetLoggerFilter 获取 Logger Filter
	GetLoggerFilter() Filter

	// SetLoggerFilter 设置 Logger Filter
	SetLoggerFilter(filter Filter)

	// GetErrorCallback 返回容器自身的错误回调
	GetErrorCallback() func(error)

	// SetErrorCallback 设置容器自身的错误回调
	SetErrorCallback(fn func(error))

	// AddRouter 添加新的路由信息
	AddRouter(router *Router)

	// EnableSwagger 是否启用 Swagger 功能
	EnableSwagger() bool

	// SetEnableSwagger 设置是否启用 Swagger 功能
	SetEnableSwagger(enable bool)

	// Swagger 返回和容器绑定的 Swagger 对象
	Swagger() *Swagger

	// Start 启动 Web 容器，非阻塞
	Start()

	// Stop 停止 Web 容器，阻塞
	Stop(ctx context.Context)
}

// BaseWebContainer WebContainer 的通用部分
type BaseWebContainer struct {
	WebMapping

	config ContainerConfig

	enableSwag bool     // 是否启用 Swagger 功能
	swagger    *Swagger // 和容器绑定的 Swagger 对象

	filters       []Filter    // 其他过滤器
	loggerFilter  Filter      // 日志过滤器
	errorCallback func(error) // 容器自身的错误回调
}

// NewBaseWebContainer BaseWebContainer 的构造函数
func NewBaseWebContainer(config ContainerConfig) *BaseWebContainer {
	return &BaseWebContainer{
		WebMapping:   NewDefaultWebMapping(),
		config:       config,
		enableSwag:   true,
		loggerFilter: defaultLoggerFilter,
	}
}

// Address 返回监听地址
func (c *BaseWebContainer) Address() string {
	return fmt.Sprintf("%s:%d", c.config.IP, c.config.Port)
}

// Config 获取 Web 容器配置
func (c *BaseWebContainer) Config() ContainerConfig {
	return c.config
}

// GetFilters 返回过滤器列表
func (c *BaseWebContainer) GetFilters() []Filter {
	return c.filters
}

// ResetFilters 重新设置过滤器列表
func (c *BaseWebContainer) ResetFilters(filters []Filter) {
	c.filters = filters
}

// AddFilter 添加过滤器
func (c *BaseWebContainer) AddFilter(filter ...Filter) {
	c.filters = append(c.filters, filter...)
}

// GetLoggerFilter 获取 Logger Filter
func (c *BaseWebContainer) GetLoggerFilter() Filter {
	return c.loggerFilter
}

// SetLoggerFilter 设置 Logger Filter
func (c *BaseWebContainer) SetLoggerFilter(filter Filter) {
	c.loggerFilter = filter
}

// GetErrorCallback 返回容器自身的错误回调
func (c *BaseWebContainer) GetErrorCallback() func(error) {
	return c.errorCallback
}

// SetErrorCallback 设置容器自身的错误回调
func (c *BaseWebContainer) SetErrorCallback(fn func(error)) {
	c.errorCallback = fn
}

// AddRouter 添加新的路由信息
func (c *BaseWebContainer) AddRouter(router *Router) {
	for _, mapper := range router.mapping.Mappers() {
		c.AddMapper(mapper)
	}
}

// EnableSwagger 是否启用 Swagger 功能
func (c *BaseWebContainer) EnableSwagger() bool {
	return c.enableSwag
}

// SetEnableSwagger 设置是否启用 Swagger 功能
func (c *BaseWebContainer) SetEnableSwagger(enable bool) {
	c.enableSwag = enable
}

// Swagger 返回和容器绑定的 Swagger 对象
func (c *BaseWebContainer) Swagger() *Swagger {
	if c.swagger == nil {
		c.swagger = NewSwagger()
	}
	return c.swagger
}

// PreStart 执行 Start 之前的准备工作
func (c *BaseWebContainer) PreStart() {

	if c.enableSwag && c.swagger != nil {

		// 注册 path 的 Operation
		for _, mapper := range c.Mappers() {
			if op := mapper.swagger; op != nil {
				if err := op.parseBind(); err != nil {
					panic(err)
				}
				c.swagger.AddPath(mapper.Path(), mapper.Method(), op)
			}
		}

		doc := c.swagger.ReadDoc()
		hSwagger := httpSwagger.Handler(httpSwagger.URL("/swagger/doc.json"))

		// 注册 swagger-ui 和 doc.json 接口
		c.GetMapping("/swagger/*", func(webCtx WebContext) {
			if webCtx.PathParam("*") == "doc.json" {
				webCtx.Header(HeaderContentType, MIMEApplicationJSONCharsetUTF8)
				webCtx.String(doc)
			} else {
				hSwagger(webCtx.ResponseWriter(), webCtx.Request())
			}
		})

		// 注册 redoc 接口
		c.GetMapping("/redoc", ReDoc)
	}

}

// PrintMapper 打印路由注册信息
func (c *BaseWebContainer) PrintMapper(m *Mapper) {
	file, line, fnName := m.handler.FileLine()
	SpringLogger.Infof("%v :%d %s -> %s:%d %s", GetMethod(m.method), c.config.Port, m.path, file, line, fnName)
}

/////////////////// Invoke Handler //////////////////////

// InvokeHandler 执行 Web 处理函数
func InvokeHandler(ctx WebContext, fn Handler, filters []Filter) {
	if len(filters) > 0 {
		filters = append(filters, HandlerFilter(fn))
		chain := NewDefaultFilterChain(filters)
		chain.Next(ctx)
	} else {
		fn.Invoke(ctx)
	}
}

/////////////////// Web Handlers //////////////////////

// fnHandler 封装 Web 处理函数
type fnHandler HandlerFunc

func (f fnHandler) Invoke(ctx WebContext) {
	f(ctx)
}

func (f fnHandler) FileLine() (file string, line int, fnName string) {
	return SpringUtils.FileLine(f)
}

// FUNC 标准 Web 处理函数的辅助函数
func FUNC(fn HandlerFunc) Handler {
	return fnHandler(fn)
}

// httpHandler 标准 Http 处理函数
type httpHandler http.HandlerFunc

func (h httpHandler) Invoke(ctx WebContext) {
	h(ctx.ResponseWriter(), ctx.Request())
}

func (h httpHandler) FileLine() (file string, line int, fnName string) {
	return SpringUtils.FileLine(h)
}

// HTTP 标准 Http 处理函数的辅助函数
func HTTP(fn http.HandlerFunc) Handler {
	return httpHandler(fn)
}

// WrapF 标准 Http 处理函数的辅助函数，兼容 gin 写法
func WrapF(fn http.HandlerFunc) Handler {
	return httpHandler(fn)
}

// WrapH 标准 Http 处理函数的辅助函数，兼容 gin 写法
func WrapH(h http.Handler) Handler {
	return httpHandler(h.ServeHTTP)
}

/////////////////// Web Filters //////////////////////

var defaultLoggerFilter = &loggerFilter{}

// loggerFilter 日志过滤器
type loggerFilter struct{}

func (f *loggerFilter) Invoke(ctx WebContext, chain FilterChain) {
	start := time.Now()
	chain.Next(ctx)
	w := ctx.ResponseWriter() // TODO echo 返回的 Json 数据有换行符，想办法去掉它
	ctx.LogInfof("cost:%v size:%d code:%d %s", time.Since(start), w.Size(), w.Status(), string(w.Body()))
}
