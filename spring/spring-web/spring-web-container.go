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

	"github.com/go-spring/spring-const"
	"github.com/go-spring/spring-logger"
	"github.com/go-spring/spring-utils"
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
	// RootRouter 根路由
	RootRouter

	// Config 获取 Web 容器配置
	Config() ContainerConfig

	// GetFilters 返回过滤器列表
	GetFilters() []Filter

	// AddFilter 添加过滤器
	AddFilter(filter ...Filter)

	// SetFilters 设置过滤器列表
	SetFilters(filters []Filter)

	// GetLoggerFilter 获取 Logger Filter
	GetLoggerFilter() Filter

	// SetLoggerFilter 设置 Logger Filter
	SetLoggerFilter(filter Filter)

	// AddRouter 添加新的路由信息
	AddRouter(router RootRouter)

	// Swagger 返回和容器绑定的 Swagger 对象
	Swagger() *Swagger

	// Start 启动 Web 容器
	Start() error

	// Stop 停止 Web 容器
	Stop(ctx context.Context) error
}

// AbstractContainer 抽象的 WebContainer 实现
type AbstractContainer struct {
	RootRouter

	config  ContainerConfig // 容器配置项
	filters []Filter        // 其他过滤器
	logger  Filter          // 日志过滤器
	swagger *Swagger        // Swagger根
}

// NewAbstractContainer AbstractContainer 的构造函数
func NewAbstractContainer(config ContainerConfig) *AbstractContainer {
	return &AbstractContainer{
		RootRouter: NewRootRouter(),
		config:     config,
		logger:     defaultLoggerFilter,
	}
}

// Address 返回监听地址
func (c *AbstractContainer) Address() string {
	return fmt.Sprintf("%s:%d", c.config.IP, c.config.Port)
}

// Config 获取 Web 容器配置
func (c *AbstractContainer) Config() ContainerConfig {
	return c.config
}

// GetFilters 返回过滤器列表
func (c *AbstractContainer) GetFilters() []Filter {
	return c.filters
}

// AddFilter 添加过滤器
func (c *AbstractContainer) AddFilter(filter ...Filter) {
	c.filters = append(c.filters, filter...)
}

// SetFilters 设置过滤器列表
func (c *AbstractContainer) SetFilters(filters []Filter) {
	c.filters = filters
}

// GetLoggerFilter 获取 Logger Filter
func (c *AbstractContainer) GetLoggerFilter() Filter {
	return c.logger
}

// SetLoggerFilter 设置 Logger Filter
func (c *AbstractContainer) SetLoggerFilter(filter Filter) {
	c.logger = filter
}

// AddRouter 添加新的路由信息
func (c *AbstractContainer) AddRouter(router RootRouter) {
	for _, mapper := range router.Mappers() {
		c.AddMapper(mapper)
	}
}

// Swagger 返回和容器绑定的 Swagger 对象
func (c *AbstractContainer) Swagger() *Swagger {
	c.swagger = NewSwagger()
	return c.swagger
}

// SwaggerHandler Swagger 处理器
type SwaggerHandler func(router RootRouter, doc string)

// swaggerHandler Swagger 处理器
var swaggerHandler SwaggerHandler

// RegisterSwaggerHandler 注册 Swagger 处理器
func RegisterSwaggerHandler(handler SwaggerHandler) {
	swaggerHandler = handler
}

// Start 启动 Web 容器
func (c *AbstractContainer) Start() error {
	if c.swagger != nil && swaggerHandler != nil {
		for _, mapper := range c.Mappers() {
			if op := mapper.swagger; op != nil {
				if err := op.parseBind(); err != nil {
					return err
				}
				c.swagger.AddPath(mapper.Path(), mapper.Method(), op)
			}
		}
		swaggerHandler(c.RootRouter, c.swagger.ReadDoc())
	}
	return nil
}

// Stop 停止 Web 容器
func (c *AbstractContainer) Stop(ctx context.Context) error {
	panic(SpringConst.UnimplementedMethod)
}

// PrintMapper 打印路由注册信息
func (c *AbstractContainer) PrintMapper(m *Mapper) {
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

// WrapF 标准 Http 处理函数的辅助函数
func WrapF(fn http.HandlerFunc) Handler {
	return httpHandler(fn)
}

// WrapH 标准 Http 处理函数的辅助函数
func WrapH(h http.Handler) Handler {
	return httpHandler(h.ServeHTTP)
}

/////////////////// Web Filters //////////////////////

var defaultLoggerFilter = &loggerFilter{}

// loggerFilter 日志过滤器
type loggerFilter struct{}

func (f *loggerFilter) Invoke(webCtx WebContext, chain FilterChain) {
	start := time.Now()
	chain.Next(webCtx)
	w := webCtx.ResponseWriter()
	SpringLogger.WithContext(webCtx.Context()).Infof("cost:%v size:%d code:%d %s", time.Since(start), w.Size(), w.Status(), string(w.Body()))
}
