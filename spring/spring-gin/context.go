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

package SpringGin

import (
	"github.com/gin-gonic/gin"
	"github.com/go-spring/spring-core/validator"
	"github.com/go-spring/spring-core/web"
)

// GinContext 将 web.Context 转换为 *gin.Context
func GinContext(webCtx web.Context) *gin.Context {
	return webCtx.NativeContext().(*gin.Context)
}

// WebContext 将 *gin.Context 转换为 web.Context
func WebContext(ginCtx *gin.Context) web.Context {
	if webCtx, _ := ginCtx.Get(web.ContextKey); webCtx != nil {
		return webCtx.(web.Context)
	}
	return nil
}

// 同时继承了 web.ResponseWriter 接口
type responseWriter struct {
	gin.ResponseWriter
	bufRW *web.BufferedResponseWriter
}

func (w *responseWriter) Size() int {
	return w.bufRW.Size()
}

func (w *responseWriter) Body() string {
	return w.bufRW.Body()
}

func (w *responseWriter) Write(data []byte) (n int, err error) {
	return w.bufRW.Write(data)
}

// Context 适配 gin 的 Web 上下文
type Context struct {
	*web.BaseContext

	// ginContext gin 上下文对象
	ginContext *gin.Context

	// handlerFunc Web 处理函数
	handlerFunc web.Handler

	pathNames  []string
	pathValues []string

	// wildCardName 通配符名称
	wildCardName string
}

// NewContext Context 的构造函数
func NewContext(fn web.Handler, wildCardName string, ginCtx *gin.Context) *Context {

	r := ginCtx.Request
	w := &web.BufferedResponseWriter{
		ResponseWriter: ginCtx.Writer,
	}
	ginCtx.Writer = &responseWriter{
		bufRW:          w,
		ResponseWriter: ginCtx.Writer,
	}

	webCtx := &Context{
		handlerFunc:  fn,
		ginContext:   ginCtx,
		wildCardName: wildCardName,
		BaseContext:  web.NewBaseContext(r, w),
	}

	ginCtx.Set(web.ContextKey, webCtx)
	return webCtx
}

// NativeContext 返回封装的底层上下文对象
func (ctx *Context) NativeContext() interface{} {
	return ctx.ginContext
}

// Path returns the registered path for the handler.
func (ctx *Context) Path() string {
	return ctx.ginContext.FullPath()
}

// Handler returns the matched handler by router.
func (ctx *Context) Handler() web.Handler {
	return ctx.handlerFunc
}

// filterPathValue gin 的路由比较怪，* 路由多一个 /
func filterPathValue(v string) string {
	if len(v) > 0 && v[0] == '/' {
		return v[1:]
	}
	return v
}

// PathParam returns path parameter by name.
func (ctx *Context) PathParam(name string) string {
	if name == "*" {
		name = ctx.wildCardName
	}
	return filterPathValue(ctx.ginContext.Param(name))
}

// PathParamNames returns path parameter names.
func (ctx *Context) PathParamNames() []string {
	if ctx.pathNames == nil {
		ctx.pathNames = make([]string, 0)
		for _, entry := range ctx.ginContext.Params {
			name := entry.Key
			if name == ctx.wildCardName {
				name = "*"
			}
			ctx.pathNames = append(ctx.pathNames, name)
		}
	}
	return ctx.pathNames
}

// PathParamValues returns path parameter values.
func (ctx *Context) PathParamValues() []string {
	if ctx.pathValues == nil {
		ctx.pathValues = make([]string, 0)
		for _, entry := range ctx.ginContext.Params {
			v := filterPathValue(entry.Value)
			ctx.pathValues = append(ctx.pathValues, v)
		}
	}
	return ctx.pathValues
}

// Bind binds the request body into provided type `i`.
func (ctx *Context) Bind(i interface{}) error {
	err := ctx.ginContext.ShouldBind(i)
	if err != nil {
		return err
	}
	return validator.Validate(i)
}

// SSEvent writes a Server-Sent Event into the body stream.
func (ctx *Context) SSEvent(name string, message interface{}) {
	ctx.ginContext.SSEvent(name, message)
}
