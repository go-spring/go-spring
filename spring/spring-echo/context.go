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

type responseWriter struct {
	*echo.Response
}

// Status Returns the HTTP response status code of the current request.
func (w *responseWriter) Status() int {
	return w.Response.Status
}

// Size Returns the number of bytes already written into the response http body.
func (w *responseWriter) Size() int {
	return int(w.Response.Size)
}

// Body 返回发送给客户端的数据，当前仅支持 MIMEApplicationJSON 格式.
func (w *responseWriter) Body() string {
	return w.Response.Writer.(web.ResponseWriter).Body()
}

// context 适配 echo 的 Web 上下文
type context struct {
	*web.BaseContext

	echoCtx  echo.Context // echo 上下文对象
	handler  web.Handler  // web 处理函数
	wildcard string       // 通配符的名称
}

// newContext Context 的构造函数
func newContext(h web.Handler, wildcard string, echoCtx echo.Context) *context {

	r := echoCtx.Request()
	w := &responseWriter{echoCtx.Response()}

	ctx := &context{
		handler:     h,
		echoCtx:     echoCtx,
		wildcard:    wildcard,
		BaseContext: web.NewBaseContext(r, w),
	}

	echoCtx.Set(web.ContextKey, ctx)
	return ctx
}

// NativeContext 返回封装的底层上下文对象
func (c *context) NativeContext() interface{} {
	return c.echoCtx
}

// Path returns the registered path for the handler.
func (c *context) Path() string {
	return c.echoCtx.Path()
}

// Handler returns the matched handler by router.
func (c *context) Handler() web.Handler {
	return c.handler
}

// PathParam returns path parameter by name.
func (c *context) PathParam(name string) string {
	if name == c.wildcard {
		name = "*"
	}
	return c.echoCtx.Param(name)
}

// PathParamNames returns path parameter names.
func (c *context) PathParamNames() []string {
	return c.echoCtx.ParamNames()
}

// PathParamValues returns path parameter values.
func (c *context) PathParamValues() []string {
	return c.echoCtx.ParamValues()
}

// Bind binds the request body into provided type `i`.
func (c *context) Bind(i interface{}) error {
	if err := c.echoCtx.Bind(i); err != nil {
		return err
	}
	return validator.Validate(i)
}
