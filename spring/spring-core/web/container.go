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

// Package web 为社区优秀的 Web 服务器提供一个抽象层，使得底层可以灵活切换。
package web

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-core/internal"
)

// HandlerFunc 标准 Web 处理函数
type HandlerFunc func(Context)

// Handler 标准 Web 处理接口
type Handler interface {

	// Invoke 响应函数
	Invoke(Context)

	// FileLine 获取用户函数的文件名、行号以及函数名称
	FileLine() (file string, line int, fnName string)
}

// ContainerConfig 定义 Web 容器配置
type ContainerConfig = internal.WebServerConfig

// Container Web 容器
type Container interface {
	Router

	// Config 获取 Web 容器配置
	Config() ContainerConfig

	// Filters 返回过滤器列表
	Filters() []Filter

	// AddFilter 添加过滤器
	AddFilter(filter ...Filter)

	// LoggerFilter 获取 Logger Filter
	LoggerFilter() Filter

	// SetLoggerFilter 设置 Logger Filter
	SetLoggerFilter(filter Filter)

	// Swagger 设置与容器绑定的 Swagger 对象
	Swagger(swagger Swagger)

	// Start 启动 Web 容器
	Start() error

	// Stop 停止 Web 容器
	Stop(ctx context.Context) error

	// File 定义单个文件资源
	File(path string, file string)

	// Static 定义一组文件资源
	Static(prefix string, root string)
}

// AbstractContainer 抽象的 Container 实现
type AbstractContainer struct {
	router

	config  ContainerConfig // 容器配置项
	filters []Filter        // 其他过滤器
	logger  Filter          // 日志过滤器
	swagger Swagger         // Swagger根
}

// NewAbstractContainer AbstractContainer 的构造函数
func NewAbstractContainer(config ContainerConfig) *AbstractContainer {
	return &AbstractContainer{config: config}
}

// Address 返回监听地址
func (c *AbstractContainer) Address() string {
	return fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)
}

// Config 获取 Web 容器配置
func (c *AbstractContainer) Config() ContainerConfig {
	return c.config
}

// Filters 返回过滤器列表
func (c *AbstractContainer) Filters() []Filter {
	return c.filters
}

// AddFilter 添加过滤器
func (c *AbstractContainer) AddFilter(filter ...Filter) {
	c.filters = append(c.filters, filter...)
}

// LoggerFilter 获取 Logger Filter
func (c *AbstractContainer) LoggerFilter() Filter {
	if c.logger != nil {
		return c.logger
	}
	return FuncFilter(func(ctx Context, chain FilterChain) {
		start := time.Now()
		chain.Next(ctx)
		w := ctx.ResponseWriter()
		log.Ctx(ctx.Context()).Infof("cost:%v size:%d code:%d %s", time.Since(start), w.Size(), w.Status(), w.Body())
	})
}

// SetLoggerFilter 设置 Logger Filter
func (c *AbstractContainer) SetLoggerFilter(filter Filter) {
	c.logger = filter
}

// Swagger 设置与容器绑定的 Swagger 对象
func (c *AbstractContainer) Swagger(swagger Swagger) {
	c.swagger = swagger
}

// SwaggerHandler Swagger 处理器
type SwaggerHandler func(router Router, doc string)

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
			if mapper.swagger == nil {
				continue
			}
			if err := mapper.swagger.Process(); err != nil {
				return err
			}
			for _, method := range GetMethod(mapper.Method()) {
				c.swagger.AddPath(mapper.Path(), method, mapper.swagger)
			}
		}
		swaggerHandler(&c.router, c.swagger.ReadDoc())
	}

	for _, mapper := range c.Mappers() {
		log.Infof("%v :%d %s -> %s:%d %s", func() []interface{} {
			method := GetMethod(mapper.method)
			file, line, fnName := mapper.handler.FileLine()
			return log.T(method, c.config.Port, mapper.path, file, line, fnName)
		})
	}

	return nil
}

// Stop 停止 Web 容器
func (c *AbstractContainer) Stop(ctx context.Context) error {
	panic(util.UnimplementedMethod)
}

// File 定义单个文件资源
func (c *AbstractContainer) File(path string, file string) {
	c.router.GetMapping(path, func(ctx Context) {
		ctx.File(file)
	})
}

// Static 定义文件服务器
func (c *AbstractContainer) Static(prefix string, root string) {
	fileServer := http.FileServer(http.Dir(root))
	h := WrapH(http.StripPrefix(prefix, fileServer))
	c.router.HandleGet(prefix+"/*", h)
}

/////////////////// Web Handlers //////////////////////

// fnHandler 封装 Web 处理函数
type fnHandler HandlerFunc

func (f fnHandler) Invoke(ctx Context) { f(ctx) }

func (f fnHandler) FileLine() (file string, line int, fnName string) {
	return util.FileLine(f)
}

// FUNC 标准 Web 处理函数的辅助函数
func FUNC(fn HandlerFunc) Handler { return fnHandler(fn) }

// httpFuncHandler 标准 Http 处理函数
type httpFuncHandler http.HandlerFunc

func (h httpFuncHandler) Invoke(ctx Context) {
	h(ctx.ResponseWriter(), ctx.Request())
}

func (h httpFuncHandler) FileLine() (file string, line int, fnName string) {
	return util.FileLine(h)
}

// HTTP 标准 Http 处理函数的辅助函数
func HTTP(fn http.HandlerFunc) Handler {
	return httpFuncHandler(fn)
}

// WrapF 标准 Http 处理函数的辅助函数
func WrapF(fn http.HandlerFunc) Handler {
	return httpFuncHandler(fn)
}

// httpHandler 标准 Http 处理函数
type httpHandler struct{ http.Handler }

func (h httpHandler) Invoke(ctx Context) {
	h.Handler.ServeHTTP(ctx.ResponseWriter(), ctx.Request())
}

func (h httpHandler) FileLine() (file string, line int, fnName string) {
	t := reflect.TypeOf(h.Handler)
	m, _ := t.MethodByName("ServeHTTP")
	return util.FileLine(m.Func.Interface())
}

// WrapH 标准 Http 处理函数的辅助函数
func WrapH(h http.Handler) Handler {
	return &httpHandler{h}
}
