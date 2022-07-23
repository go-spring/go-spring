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

	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-base/knife"
	"github.com/go-spring/spring-base/util"
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

// ServerConfig 定义 web 服务器配置
type ServerConfig struct {
	Host         string `value:"${host:=}"`            // 监听 IP
	Port         int    `value:"${port:=8080}"`        // HTTP 端口
	EnableSSL    bool   `value:"${ssl.enable:=false}"` // 是否启用 HTTPS
	KeyFile      string `value:"${ssl.key:=}"`         // SSL 秘钥
	CertFile     string `value:"${ssl.cert:=}"`        // SSL 证书
	BasePath     string `value:"${base-path:=}"`       // 根路径
	Prefix       string `value:"${prefix:=}"`          // 路由前缀
	ReadTimeout  int    `value:"${read-timeout:=0}"`   // 读取超时，毫秒
	WriteTimeout int    `value:"${write-timeout:=0}"`  // 写入超时，毫秒
}

// ErrorHandler 错误处理接口
type ErrorHandler interface {
	Invoke(ctx Context, err *HttpError)
}

// Server web 服务器
type Server interface {
	Router

	// Config 获取 web 服务器配置
	Config() ServerConfig

	// Prefilters 返回前置过滤器列表
	Prefilters() []*Prefilter

	// AddPrefilter 添加前置过滤器
	AddPrefilter(filter ...*Prefilter)

	// Filters 返回过滤器列表
	Filters() []Filter

	// AddFilter 添加过滤器
	AddFilter(filter ...Filter)

	// LoggerFilter 获取 Logger Filter
	LoggerFilter() Filter

	// SetLoggerFilter 设置 Logger Filter
	SetLoggerFilter(filter Filter)

	// ErrorHandler 获取错误处理接口
	ErrorHandler() ErrorHandler

	// SetErrorHandler 设置错误处理接口
	SetErrorHandler(errHandler ErrorHandler)

	// Swagger 设置与服务器绑定的 Swagger 对象
	Swagger(swagger Swagger)

	// Start 启动 web 服务器
	Start() error

	// Stop 停止 web 服务器
	Stop(ctx context.Context) error
}

type ServerHandler interface {
	http.Handler
	Start(s Server) error
	RecoveryFilter(errHandler ErrorHandler) Filter
}

type server struct {
	router

	config  ServerConfig // 容器配置项
	server  *http.Server
	handler ServerHandler

	logger     Filter       // 日志过滤器
	filters    []Filter     // 其他过滤器
	prefilters []*Prefilter // 前置过滤器
	errHandler ErrorHandler // 错误处理接口

	swagger Swagger // Swagger根
}

// NewServer server 的构造函数
func NewServer(config ServerConfig, handler ServerHandler) *server {
	return &server{config: config, handler: handler}
}

// Address 返回监听地址
func (s *server) Address() string {
	return fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
}

// Config 获取 web 服务器配置
func (s *server) Config() ServerConfig {
	return s.config
}

// Prefilters 返回前置过滤器列表
func (s *server) Prefilters() []*Prefilter {
	return s.prefilters
}

// AddPrefilter 添加前置过滤器
func (s *server) AddPrefilter(filter ...*Prefilter) {
	s.prefilters = append(s.prefilters, filter...)
}

// Filters 返回过滤器列表
func (s *server) Filters() []Filter {
	return s.filters
}

// AddFilter 添加过滤器
func (s *server) AddFilter(filters ...Filter) {
	for _, filter := range filters {
		if prefilter, ok := filter.(*Prefilter); ok {
			s.AddPrefilter(prefilter)
		} else {
			s.filters = append(s.filters, filter)
		}
	}
}

// LoggerFilter 获取 Logger Filter
func (s *server) LoggerFilter() Filter {
	if s.logger != nil {
		return s.logger
	}
	return AccessLog()
}

// SetLoggerFilter 设置 Logger Filter
func (s *server) SetLoggerFilter(filter Filter) {
	s.logger = filter
}

// ErrorHandler 获取错误处理接口
func (s *server) ErrorHandler() ErrorHandler {
	return s.errHandler
}

// SetErrorHandler 设置错误处理接口
func (s *server) SetErrorHandler(errHandler ErrorHandler) {
	s.errHandler = errHandler
}

// SwaggerHandler Swagger 处理器
type SwaggerHandler func(router Router, doc string)

// swaggerHandler Swagger 处理器
var swaggerHandler SwaggerHandler

// RegisterSwaggerHandler 注册 Swagger 处理器
func RegisterSwaggerHandler(handler SwaggerHandler) {
	swaggerHandler = handler
}

// Swagger 设置与服务器绑定的 Swagger 对象
func (s *server) Swagger(swagger Swagger) {
	s.swagger = swagger
}

// prepare 启动 web 服务器之前的准备工作
func (s *server) prepare() error {

	// 处理 swagger 注册相关
	if s.swagger != nil && swaggerHandler != nil {
		for _, mapper := range s.Mappers() {
			if mapper.swagger == nil {
				continue
			}
			if err := mapper.swagger.Process(); err != nil {
				return err
			}
			for _, method := range GetMethod(mapper.Method()) {
				s.swagger.AddPath(mapper.Path(), method, mapper.swagger)
			}
		}
		swaggerHandler(&s.router, s.swagger.ReadDoc())
	}

	// 打印所有的路由信息
	for _, mapper := range s.Mappers() {
		logger.Infof("%v :%d %s -> %s:%d %s", func() []interface{} {
			method := GetMethod(mapper.method)
			file, line, fnName := mapper.handler.FileLine()
			return util.T(method, s.config.Port, mapper.path, file, line, fnName)
		})
	}

	return nil
}

// Start 启动 web 服务器
func (s *server) Start() (err error) {
	if err = s.prepare(); err != nil {
		return err
	}
	if err = s.handler.Start(s); err != nil {
		return err
	}
	s.server = &http.Server{
		Handler:      s,
		Addr:         s.Address(),
		ReadTimeout:  time.Duration(s.config.ReadTimeout) * time.Millisecond,
		WriteTimeout: time.Duration(s.config.WriteTimeout) * time.Millisecond,
	}
	logger.Info("⇨ http server started on ", s.Address())
	if !s.config.EnableSSL {
		err = s.server.ListenAndServe()
	} else {
		err = s.server.ListenAndServeTLS(s.config.CertFile, s.config.KeyFile)
	}
	logger.Infof("http server stopped on %s return %s", s.Address(), cast.ToString(err))
	return err
}

// Stop 停止 web 服务器
func (s *server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	writer := &BufferedResponseWriter{ResponseWriter: w, cache: true}
	if ctx, cached := knife.New(r.Context()); !cached {
		r = r.WithContext(ctx)
	}
	prefilters := append([]Filter{}, s.LoggerFilter())
	errHandler := s.errHandler
	if errHandler == nil {
		errHandler = defaultErrorHandler
	}
	prefilters = append(prefilters, s.handler.RecoveryFilter(errHandler))
	for _, f := range s.Prefilters() {
		prefilters = append(prefilters, f)
	}
	prefilters = append(prefilters, s.filters...)
	prefilters = append(prefilters, HandlerFilter(WrapH(s.handler)))
	NewFilterChain(prefilters).Next(NewBaseContext("", nil, r, writer))
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
