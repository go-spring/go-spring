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
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"

	"github.com/go-spring/spring-const"
	"github.com/go-spring/spring-logger"
	"github.com/go-spring/spring-web"
	"github.com/labstack/echo"
)

// EchoContext 将 SpringWeb.WebContext 转换为 echo.Context
func EchoContext(webCtx SpringWeb.WebContext) echo.Context {
	return webCtx.NativeContext().(echo.Context)
}

// WebContext 将 echo.Context 转换为 SpringWeb.WebContext
func WebContext(echoCtx echo.Context) SpringWeb.WebContext {
	if webCtx := echoCtx.Get(SpringWeb.WebContextKey); webCtx != nil {
		return webCtx.(SpringWeb.WebContext)
	}
	return nil
}

// responseWriter SpringWeb.ResponseWriter 的 echo 适配.
type responseWriter struct {
	*echo.Response
}

// Returns the HTTP response status code of the current request.
func (r *responseWriter) Status() int {
	return r.Response.Status
}

// Returns the number of bytes already written into the response http body.
// See Written()
func (r *responseWriter) Size() int {
	return int(r.Response.Size)
}

// Context 适配 echo 的 Web 上下文
type Context struct {
	// LoggerContext 日志接口上下文
	SpringLogger.LoggerContext

	// echoContext echo 上下文对象
	echoContext echo.Context

	// handlerFunc Web 处理函数
	handlerFunc SpringWeb.Handler

	// wildCardName 通配符的名称
	wildCardName string

	// aborted 处理过程是否终止
	aborted bool
}

// NewContext Context 的构造函数
func NewContext(fn SpringWeb.Handler, wildCardName string, echoCtx echo.Context) *Context {

	ctx := echoCtx.Request().Context()
	logCtx := SpringLogger.NewDefaultLoggerContext(ctx)

	webCtx := &Context{
		LoggerContext: logCtx,
		echoContext:   echoCtx,
		handlerFunc:   fn,
		wildCardName:  wildCardName,
	}

	webCtx.Set(SpringWeb.WebContextKey, webCtx)
	return webCtx
}

// NativeContext 返回封装的底层上下文对象
func (ctx *Context) NativeContext() interface{} {
	return ctx.echoContext
}

// Get retrieves data from the context.
func (ctx *Context) Get(key string) interface{} {
	return ctx.echoContext.Get(key)
}

// Set saves data in the context.
func (ctx *Context) Set(key string, val interface{}) {
	ctx.echoContext.Set(key, val)
}

// Request returns `*http.Request`.
func (ctx *Context) Request() *http.Request {
	return ctx.echoContext.Request()
}

// SetRequest sets `*http.Request`.
func (ctx *Context) SetRequest(r *http.Request) {
	ctx.echoContext.SetRequest(r)
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
func (ctx *Context) Handler() SpringWeb.Handler {
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
	return SpringWeb.Validate(i)
}

// IsAborted 当前处理过程是否终止，为了适配 gin 的模型，未来底层统一了会去掉.
func (ctx *Context) IsAborted() bool {
	return ctx.aborted
}

// Abort 终止当前处理过程，为了适配 gin 的模型，未来底层统一了会去掉.
func (ctx *Context) Abort() {
	ctx.aborted = true
}

// ResponseWriter returns `http.ResponseWriter`.
func (ctx *Context) ResponseWriter() SpringWeb.ResponseWriter {
	return &responseWriter{ctx.echoContext.Response()}
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
	_ = ctx.echoContext.NoContent(code)
}

// String writes the given string into the response body.
func (ctx *Context) String(code int, format string, values ...interface{}) error {
	return ctx.echoContext.String(code, fmt.Sprintf(format, values...))
}

// HTML sends an HTTP response with status code.
func (ctx *Context) HTML(code int, html string) error {
	return ctx.echoContext.HTML(code, html)
}

// HTMLBlob sends an HTTP blob response with status code.
func (ctx *Context) HTMLBlob(code int, b []byte) error {
	return ctx.echoContext.HTMLBlob(code, b)
}

// JSON sends a JSON response with status code.
func (ctx *Context) JSON(code int, i interface{}) error {
	return ctx.echoContext.JSON(code, i)
}

// JSONPretty sends a pretty-print JSON with status code.
func (ctx *Context) JSONPretty(code int, i interface{}, indent string) error {
	return ctx.echoContext.JSONPretty(code, i, indent)
}

// JSONBlob sends a JSON blob response with status code.
func (ctx *Context) JSONBlob(code int, b []byte) error {
	return ctx.echoContext.JSONBlob(code, b)
}

// JSONP sends a JSONP response with status code.
func (ctx *Context) JSONP(code int, callback string, i interface{}) error {
	return ctx.echoContext.JSONP(code, callback, i)
}

// JSONPBlob sends a JSONP blob response with status code.
func (ctx *Context) JSONPBlob(code int, callback string, b []byte) error {
	return ctx.echoContext.JSONPBlob(code, callback, b)
}

// XML sends an XML response with status code.
func (ctx *Context) XML(code int, i interface{}) error {
	return ctx.echoContext.XML(code, i)
}

// XMLPretty sends a pretty-print XML with status code.
func (ctx *Context) XMLPretty(code int, i interface{}, indent string) error {
	return ctx.echoContext.XMLPretty(code, i, indent)
}

// XMLBlob sends an XML blob response with status code.
func (ctx *Context) XMLBlob(code int, b []byte) error {
	return ctx.echoContext.XMLBlob(code, b)
}

// Blob sends a blob response with status code and content type.
func (ctx *Context) Blob(code int, contentType string, b []byte) error {
	return ctx.echoContext.Blob(code, contentType, b)
}

// Stream sends a streaming response with status code and content type.
func (ctx *Context) Stream(code int, contentType string, r io.Reader) error {
	return ctx.echoContext.Stream(code, contentType, r)
}

// File sends a response with the content of the file.
func (ctx *Context) File(file string) error {
	return ctx.echoContext.File(file)
}

// Attachment sends a response as attachment
func (ctx *Context) Attachment(file string, name string) error {
	return ctx.echoContext.Attachment(file, name)
}

// Inline sends a response as inline
func (ctx *Context) Inline(file string, name string) error {
	return ctx.echoContext.Inline(file, name)
}

// Redirect redirects the request to a provided URL with status code.
func (ctx *Context) Redirect(code int, url string) error {
	return ctx.echoContext.Redirect(code, url)
}

// SSEvent writes a Server-Sent Event into the body stream.
func (ctx *Context) SSEvent(name string, message interface{}) error {
	panic(SpringConst.UnimplementedMethod)
}
