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
	"github.com/go-spring/spring-core/validator"
	"github.com/go-spring/spring-core/web"
	"github.com/labstack/echo/v4"
)

// EchoContext 将 web.Context 转换为 echo.Context
func EchoContext(ctx web.Context) echo.Context {
	return ctx.NativeContext().(echo.Context)
}

// WebContext 将 echo.Context 转换为 web.Context
func WebContext(echoCtx echo.Context) web.Context {
	if ctx := echoCtx.Get(web.ContextKey); ctx != nil {
		return ctx.(web.Context)
	}
	return nil
}

// Context 适配 echo 的 Web 上下文
type Context struct {
	*web.BaseContext

	// echoContext echo 上下文对象
	echoContext echo.Context

	// handlerFunc Web 处理函数
	handlerFunc web.Handler

	// wildCardName 通配符的名称
	wildCardName string
}

// NewContext Context 的构造函数
func NewContext(fn web.Handler, wildCardName string, echoCtx echo.Context) *Context {

	r := echoCtx.Request()
	w := &web.BufferedResponseWriter{
		ResponseWriter: echoCtx.Response().Writer,
	}
	echoCtx.Response().Writer = w

	ctx := &Context{
		handlerFunc:  fn,
		echoContext:  echoCtx,
		wildCardName: wildCardName,
		BaseContext:  web.NewBaseContext(r, w),
	}

	echoCtx.Set(web.ContextKey, ctx)
	return ctx
}

// NativeContext 返回封装的底层上下文对象
func (ctx *Context) NativeContext() interface{} {
	return ctx.echoContext
}

// Path returns the registered path for the handler.
func (ctx *Context) Path() string {
	return ctx.echoContext.Path()
}

// Handler returns the matched handler by router.
func (ctx *Context) Handler() web.Handler {
	return ctx.handlerFunc
}

// PathParam returns path parameter by name.
func (ctx *Context) PathParam(name string) string {
	if name == ctx.wildCardName {
		name = "*"
	}
	return ctx.echoContext.Param(name)
}

// PathParamNames returns path parameter names.
func (ctx *Context) PathParamNames() []string {
	return ctx.echoContext.ParamNames()
}

// PathParamValues returns path parameter values.
func (ctx *Context) PathParamValues() []string {
	return ctx.echoContext.ParamValues()
}

// Bind binds the request body into provided type `i`.
func (ctx *Context) Bind(i interface{}) error {
	if err := ctx.echoContext.Bind(i); err != nil {
		return err
	}
	return validator.Validate(i)
}
