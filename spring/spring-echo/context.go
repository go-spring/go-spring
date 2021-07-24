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
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"

	"github.com/go-spring/spring-core/validator"
	"github.com/go-spring/spring-core/web"
	"github.com/go-spring/spring-stl/json"
	"github.com/go-spring/spring-stl/knife"
	"github.com/go-spring/spring-stl/util"
	"github.com/labstack/echo"
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

// 同时继承了 web.ResponseWriter 接口
type responseWriter struct {
	response *echo.Response
	writer   *web.BufferedResponseWriter
}

func (w *responseWriter) Header() http.Header {
	return w.writer.Header()
}

func (w *responseWriter) Write(data []byte) (n int, err error) {
	return w.writer.Write(data)
}

func (w *responseWriter) WriteHeader(code int) {
	w.writer.WriteHeader(code)
}

func (w *responseWriter) Status() int {
	return w.response.Status
}

func (w *responseWriter) Size() int {
	return w.writer.Size()
}

func (w *responseWriter) Body() []byte {
	return w.writer.Body()
}

// Context 适配 echo 的 Web 上下文
type Context struct {

	// echoContext echo 上下文对象
	echoContext echo.Context

	// handlerFunc Web 处理函数
	handlerFunc web.Handler

	// wildCardName 通配符的名称
	wildCardName string
}

// NewContext Context 的构造函数
func NewContext(fn web.Handler, wildCardName string, echoCtx echo.Context) *Context {

	{
		req := echoCtx.Request()
		ctx := knife.New(req.Context())
		echoCtx.SetRequest(req.WithContext(ctx))
	}

	echoCtx.Response().Writer = &responseWriter{
		writer: &web.BufferedResponseWriter{
			ResponseWriter: echoCtx.Response().Writer,
		},
		response: echoCtx.Response(),
	}

	ctx := &Context{
		handlerFunc:  fn,
		echoContext:  echoCtx,
		wildCardName: wildCardName,
	}

	echoCtx.Set(web.ContextKey, ctx)
	return ctx
}

// NativeContext 返回封装的底层上下文对象
func (ctx *Context) NativeContext() interface{} {
	return ctx.echoContext
}

// Request returns `*http.Request`.
func (ctx *Context) Request() *http.Request {
	return ctx.echoContext.Request()
}

// SetRequest sets `*http.Request`.
func (ctx *Context) SetRequest(r *http.Request) {
	ctx.echoContext.SetRequest(r)
}

// Context 返回 Request 绑定的 context.Context 对象
func (ctx *Context) Context() context.Context {
	return ctx.echoContext.Request().Context()
}

// IsTLS returns true if HTTP connection is TLS otherwise false.
func (ctx *Context) IsTLS() bool {
	return ctx.echoContext.IsTLS()
}

// IsWebSocket returns true if HTTP connection is WebSocket otherwise false.
func (ctx *Context) IsWebSocket() bool {
	return ctx.echoContext.IsWebSocket()
}

// Scheme returns the HTTP protocol scheme, `http` or `https`.
func (ctx *Context) Scheme() string {
	return ctx.echoContext.Scheme()
}

// ClientIP implements a best effort algorithm to return the real client IP.
func (ctx *Context) ClientIP() string {
	return ctx.echoContext.RealIP()
}

// Path returns the registered path for the handler.
func (ctx *Context) Path() string {
	return ctx.echoContext.Path()
}

// Handler returns the matched handler by router.
func (ctx *Context) Handler() web.Handler {
	return ctx.handlerFunc
}

func filterFlags(content string) string {
	for i, char := range content {
		if char == ' ' || char == ';' {
			return content[:i]
		}
	}
	return content
}

// ContentType returns the Content-Type header of the request.
func (ctx *Context) ContentType() string {
	// NOTE: 这一段逻辑使用 gin 的实现

	s := ctx.GetHeader("Content-Type")
	return filterFlags(s)
}

// GetHeader returns value from request headers.
func (ctx *Context) GetHeader(key string) string {
	return ctx.Request().Header.Get(key)
}

// GetRawData return stream data.
func (ctx *Context) GetRawData() ([]byte, error) {
	return ioutil.ReadAll(ctx.Request().Body)
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

// QueryParam returns the query param for the provided name.
func (ctx *Context) QueryParam(name string) string {
	return ctx.echoContext.QueryParam(name)
}

// QueryParams returns the query parameters as `url.Values`.
func (ctx *Context) QueryParams() url.Values {
	return ctx.echoContext.QueryParams()
}

// QueryString returns the URL query string.
func (ctx *Context) QueryString() string {
	return ctx.echoContext.QueryString()
}

// FormValue returns the form field value for the provided name.
func (ctx *Context) FormValue(name string) string {
	return ctx.echoContext.FormValue(name)
}

// FormParams returns the form parameters as `url.Values`.
func (ctx *Context) FormParams() (url.Values, error) {
	return ctx.echoContext.FormParams()
}

// FormFile returns the multipart form file for the provided name.
func (ctx *Context) FormFile(name string) (*multipart.FileHeader, error) {
	return ctx.echoContext.FormFile(name)
}

// SaveUploadedFile uploads the form file to specific dst.
func (ctx *Context) SaveUploadedFile(file *multipart.FileHeader, dst string) error {
	// NOTE: 这一段逻辑使用 gin 的实现

	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, src)
	return err
}

// MultipartForm returns the multipart form.
func (ctx *Context) MultipartForm() (*multipart.Form, error) {
	return ctx.echoContext.MultipartForm()
}

// Cookie returns the named cookie provided in the request.
func (ctx *Context) Cookie(name string) (*http.Cookie, error) {
	return ctx.echoContext.Cookie(name)
}

// Cookies returns the HTTP cookies sent with the request.
func (ctx *Context) Cookies() []*http.Cookie {
	return ctx.echoContext.Cookies()
}

// Bind binds the request body into provided type `i`.
func (ctx *Context) Bind(i interface{}) error {

	if req := ctx.Request(); req.ContentLength == 0 && req.Method == http.MethodPost {
		return nil
	}

	if err := ctx.echoContext.Bind(i); err != nil {
		return err
	}
	return validator.Validate(i)
}

// ResponseWriter returns `http.ResponseWriter`.
func (ctx *Context) ResponseWriter() web.ResponseWriter {
	return ctx.echoContext.Response().Writer.(*responseWriter)
}

// Status sets the HTTP response code.
func (ctx *Context) Status(code int) {
	ctx.echoContext.Response().WriteHeader(code)
}

// Header is a intelligent shortcut for c.Writer.Header().Set(key, value).
func (ctx *Context) Header(key, value string) {
	ctx.echoContext.Response().Header().Set(key, value)
}

// SetCookie adds a `Set-Cookie` header in HTTP response.
func (ctx *Context) SetCookie(cookie *http.Cookie) {
	ctx.echoContext.SetCookie(cookie)
}

// NoContent sends a response with no body and a status code.
func (ctx *Context) NoContent(code int) {
	if err := ctx.echoContext.NoContent(code); err != nil {
		panic(err)
	}
}

// String writes the given string into the response body.
func (ctx *Context) String(format string, values ...interface{}) {
	statusCode := ctx.echoContext.Response().Status
	if err := ctx.echoContext.String(statusCode, fmt.Sprintf(format, values...)); err != nil {
		panic(err)
	}
}

// HTML sends an HTTP response.
func (ctx *Context) HTML(html string) {
	statusCode := ctx.echoContext.Response().Status
	if err := ctx.echoContext.HTML(statusCode, html); err != nil {
		panic(err)
	}
}

// HTMLBlob sends an HTTP blob response.
func (ctx *Context) HTMLBlob(b []byte) {
	statusCode := ctx.echoContext.Response().Status
	if err := ctx.echoContext.HTMLBlob(statusCode, b); err != nil {
		panic(err)
	}
}

// JSON sends a JSON response.
func (ctx *Context) JSON(i interface{}) {
	b, err := json.Marshal(i)
	if err != nil {
		panic(err)
	}
	ctx.Blob(web.MIMEApplicationJSONCharsetUTF8, b)
}

// JSONPretty sends a pretty-print JSON.
func (ctx *Context) JSONPretty(i interface{}, indent string) {
	b, err := json.MarshalIndent(i, "", indent)
	if err != nil {
		panic(err)
	}
	ctx.Blob(web.MIMEApplicationJSONCharsetUTF8, b)
}

// JSONBlob sends a JSON blob response.
func (ctx *Context) JSONBlob(b []byte) {
	statusCode := ctx.echoContext.Response().Status
	if err := ctx.echoContext.JSONBlob(statusCode, b); err != nil {
		panic(err)
	}
}

// JSONP sends a JSONP response.
func (ctx *Context) JSONP(callback string, i interface{}) {
	statusCode := ctx.echoContext.Response().Status
	if err := ctx.echoContext.JSONP(statusCode, callback, i); err != nil {
		panic(err)
	}
}

// JSONPBlob sends a JSONP blob response.
func (ctx *Context) JSONPBlob(callback string, b []byte) {
	statusCode := ctx.echoContext.Response().Status
	if err := ctx.echoContext.JSONPBlob(statusCode, callback, b); err != nil {
		panic(err)
	}
}

// XML sends an XML response.
func (ctx *Context) XML(i interface{}) {
	statusCode := ctx.echoContext.Response().Status
	if err := ctx.echoContext.XML(statusCode, i); err != nil {
		panic(err)
	}
}

// XMLPretty sends a pretty-print XML.
func (ctx *Context) XMLPretty(i interface{}, indent string) {
	statusCode := ctx.echoContext.Response().Status
	if err := ctx.echoContext.XMLPretty(statusCode, i, indent); err != nil {
		panic(err)
	}
}

// XMLBlob sends an XML blob response.
func (ctx *Context) XMLBlob(b []byte) {
	statusCode := ctx.echoContext.Response().Status
	if err := ctx.echoContext.XMLBlob(statusCode, b); err != nil {
		panic(err)
	}
}

// Blob sends a blob response with content type.
func (ctx *Context) Blob(contentType string, b []byte) {
	statusCode := ctx.echoContext.Response().Status
	if err := ctx.echoContext.Blob(statusCode, contentType, b); err != nil {
		panic(err)
	}
}

// File sends a response with the content of the file.
func (ctx *Context) File(file string) {
	if err := ctx.echoContext.File(file); err != nil {
		panic(err)
	}
}

// Attachment sends a response as attachment
func (ctx *Context) Attachment(file string, name string) {
	if err := ctx.echoContext.Attachment(file, name); err != nil {
		panic(err)
	}
}

// Inline sends a response as inline
func (ctx *Context) Inline(file string, name string) {
	if err := ctx.echoContext.Inline(file, name); err != nil {
		panic(err)
	}
}

// Redirect redirects the request to a provided URL with status code.
func (ctx *Context) Redirect(code int, url string) {
	if err := ctx.echoContext.Redirect(code, url); err != nil {
		panic(err)
	}
}

// SSEvent writes a Server-Sent Event into the body stream.
func (ctx *Context) SSEvent(name string, message interface{}) {
	panic(util.UnimplementedMethod)
}
