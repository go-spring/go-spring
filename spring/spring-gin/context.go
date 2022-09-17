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
	"net/http"
	"reflect"

	"github.com/gin-gonic/gin"
	"github.com/go-spring/spring-core/web"
)

// GinContext 将 web.Context 转换为 *gin.Context
func GinContext(c web.Context) *gin.Context {
	v := c.NativeContext()
	if v == nil {
		return nil
	}
	ctx, _ := v.(*gin.Context)
	return ctx
}

// WebContext 将 *gin.Context 转换为 web.Context
func WebContext(c *gin.Context) web.Context {
	v, _ := c.Get(web.ContextKey)
	if v == nil {
		return nil
	}
	ctx, _ := v.(web.Context)
	return ctx
}

type Response struct {
	gin.ResponseWriter
}

func (resp *Response) Get() http.ResponseWriter {
	e := reflect.ValueOf(resp.ResponseWriter).Elem()
	return e.Field(0).Interface().(http.ResponseWriter)
}

func (resp *Response) Set(w http.ResponseWriter) {
	e := reflect.ValueOf(resp.ResponseWriter).Elem()
	e.Field(0).Set(reflect.ValueOf(w))
}

// Context 适配 gin 的 Web 上下文
type Context struct {
	*web.BaseContext

	// ginContext gin 上下文对象
	ginContext *gin.Context

	pathNames  []string
	pathValues []string

	// wildcard 通配符的名称
	wildcard string
}

// newContext Context 的构造函数
func newContext(handler web.Handler, path, wildcard string, ginCtx *gin.Context) *Context {
	r := ginCtx.Request
	w := &Response{ResponseWriter: ginCtx.Writer}
	webCtx := &Context{
		ginContext:  ginCtx,
		wildcard:    wildcard,
		BaseContext: web.NewBaseContext(path, handler, r, w),
	}
	ginCtx.Set(web.ContextKey, webCtx)
	return webCtx
}

// NativeContext 返回封装的底层上下文对象
func (ctx *Context) NativeContext() interface{} {
	return ctx.ginContext
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
		name = ctx.wildcard
	}
	return filterPathValue(ctx.ginContext.Param(name))
}

// PathParamNames returns path parameter names.
func (ctx *Context) PathParamNames() []string {
	if ctx.pathNames == nil {
		ctx.pathNames = make([]string, 0)
		for _, entry := range ctx.ginContext.Params {
			name := entry.Key
			if name == ctx.wildcard {
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

// SSEvent writes a Server-Sent Event into the body stream.
func (ctx *Context) SSEvent(name string, message interface{}) {
	ctx.ginContext.SSEvent(name, message)
}
