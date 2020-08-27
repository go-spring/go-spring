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
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-spring/spring-logger"
	"github.com/go-spring/spring-utils"
	"github.com/go-spring/spring-web"
)

const (
	defaultMemory = 32 << 20 // 32 MB
)

// GinContext 将 SpringWeb.WebContext 转换为 *gin.Context
func GinContext(webCtx SpringWeb.WebContext) *gin.Context {
	return webCtx.NativeContext().(*gin.Context)
}

// WebContext 将 *gin.Context 转换为 SpringWeb.WebContext
func WebContext(ginCtx *gin.Context) SpringWeb.WebContext {
	if webCtx, _ := ginCtx.Get(SpringWeb.WebContextKey); webCtx != nil {
		return webCtx.(SpringWeb.WebContext)
	}
	return nil
}

// Context 适配 gin 的 Web 上下文
type Context struct {
	// LoggerContext 日志接口上下文
	SpringLogger.LoggerContext

	// ginContext gin 上下文对象
	ginContext *gin.Context

	// handlerFunc Web 处理函数
	handlerFunc SpringWeb.Handler

	pathNames  []string
	pathValues []string

	// wildCardName 通配符名称
	wildCardName string
}

// NewContext Context 的构造函数
func NewContext(fn SpringWeb.Handler, wildCardName string, ginCtx *gin.Context) *Context {

	ctx := ginCtx.Request.Context()
	logCtx := SpringLogger.NewDefaultLoggerContext(ctx)

	webCtx := &Context{
		LoggerContext: logCtx,
		ginContext:    ginCtx,
		handlerFunc:   fn,
		wildCardName:  wildCardName,
	}

	webCtx.Set(SpringWeb.WebContextKey, webCtx)
	return webCtx
}

// NativeContext 返回封装的底层上下文对象
func (ctx *Context) NativeContext() interface{} {
	return ctx.ginContext
}

// Get retrieves data from the context.
func (ctx *Context) Get(key string) interface{} {
	return ctx.ginContext.MustGet(key)
}

// Set saves data in the context.
func (ctx *Context) Set(key string, val interface{}) {
	ctx.ginContext.Set(key, val)
}

// Request returns `*http.Request`.
func (ctx *Context) Request() *http.Request {
	return ctx.ginContext.Request
}

// SetRequest sets `*http.Request`.
func (ctx *Context) SetRequest(r *http.Request) {
	ctx.ginContext.Request = r
}

// IsTLS returns true if HTTP connection is TLS otherwise false.
func (ctx *Context) IsTLS() bool {
	return ctx.ginContext.Request.TLS != nil
}

// IsWebSocket returns true if HTTP connection is WebSocket otherwise false.
func (ctx *Context) IsWebSocket() bool {
	return ctx.ginContext.IsWebsocket()
}

// Scheme returns the HTTP protocol scheme, `http` or `https`.
func (ctx *Context) Scheme() string {
	// NOTE: 这一段逻辑使用 echo 的实现
	r := ctx.Request()

	// Can't use `r.Request.URL.Scheme`
	// See: https://groups.google.com/forum/#!topic/golang-nuts/pMUkBlQBDF0

	if r.TLS != nil {
		return "https"
	}

	if scheme := r.Header.Get(SpringWeb.HeaderXForwardedProto); scheme != "" {
		return scheme
	}

	if scheme := r.Header.Get(SpringWeb.HeaderXForwardedProtocol); scheme != "" {
		return scheme
	}

	if ssl := r.Header.Get(SpringWeb.HeaderXForwardedSsl); ssl == "on" {
		return "https"
	}

	if scheme := r.Header.Get(SpringWeb.HeaderXUrlScheme); scheme != "" {
		return scheme
	}
	return "http"
}

// ClientIP implements a best effort algorithm to return the real client IP
func (ctx *Context) ClientIP() string {
	return ctx.ginContext.ClientIP()
}

// Path returns the registered path for the handler.
func (ctx *Context) Path() string {
	return ctx.ginContext.FullPath()
}

// Handler returns the matched handler by router.
func (ctx *Context) Handler() SpringWeb.Handler {
	return ctx.handlerFunc
}

// ContentType returns the Content-Type header of the request.
func (ctx *Context) ContentType() string {
	return ctx.ginContext.ContentType()
}

// GetHeader returns value from request headers.
func (ctx *Context) GetHeader(key string) string {
	return ctx.ginContext.GetHeader(key)
}

// GetRawData return stream data.
func (ctx *Context) GetRawData() ([]byte, error) {
	return ctx.ginContext.GetRawData()
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

// QueryParam returns the query param for the provided name.
func (ctx *Context) QueryParam(name string) string {
	return ctx.ginContext.Query(name)
}

// QueryParams returns the query parameters as `url.Values`.
func (ctx *Context) QueryParams() url.Values {
	return ctx.ginContext.Request.URL.Query()
}

// QueryString returns the URL query string.
func (ctx *Context) QueryString() string {
	return ctx.ginContext.Request.URL.RawQuery
}

// FormValue returns the form field value for the provided name.
func (ctx *Context) FormValue(name string) string {
	return ctx.ginContext.Request.FormValue(name)
}

// FormParams returns the form parameters as `url.Values`.
func (ctx *Context) FormParams() (url.Values, error) {
	// NOTE: 这一段逻辑使用 echo 的实现

	r := ctx.ginContext.Request

	if strings.HasPrefix(ctx.ContentType(), SpringWeb.MIMEMultipartForm) {
		if err := r.ParseMultipartForm(defaultMemory); err != nil {
			return nil, err
		}
	} else {
		if err := r.ParseForm(); err != nil {
			return nil, err
		}
	}
	return ctx.ginContext.Request.Form, nil
}

// FormFile returns the multipart form file for the provided name.
func (ctx *Context) FormFile(name string) (*multipart.FileHeader, error) {
	return ctx.ginContext.FormFile(name)
}

// SaveUploadedFile uploads the form file to specific dst.
func (ctx *Context) SaveUploadedFile(file *multipart.FileHeader, dst string) error {
	return ctx.ginContext.SaveUploadedFile(file, dst)
}

// MultipartForm returns the multipart form.
func (ctx *Context) MultipartForm() (*multipart.Form, error) {
	return ctx.ginContext.MultipartForm()
}

// Cookie returns the named cookie provided in the request.
func (ctx *Context) Cookie(name string) (*http.Cookie, error) {
	return ctx.ginContext.Request.Cookie(name)
}

// Cookies returns the HTTP cookies sent with the request.
func (ctx *Context) Cookies() []*http.Cookie {
	return ctx.ginContext.Request.Cookies()
}

// Bind binds the request body into provided type `i`.
func (ctx *Context) Bind(i interface{}) error {
	return ctx.ginContext.ShouldBind(i)
}

// ResponseWriter returns `http.ResponseWriter`.
func (ctx *Context) ResponseWriter() http.ResponseWriter {
	return ctx.ginContext.Writer
}

// Status sets the HTTP response code.
func (ctx *Context) Status(code int) {
	ctx.ginContext.Status(code)
}

// Header is a intelligent shortcut for c.Writer.Header().Set(key, value).
func (ctx *Context) Header(key, value string) {
	ctx.ginContext.Header(key, value)
}

// SetCookie adds a `Set-Cookie` header in HTTP response.
func (ctx *Context) SetCookie(cookie *http.Cookie) {
	http.SetCookie(ctx.ginContext.Writer, cookie)
}

// NoContent sends a response with no body and a status code.
func (ctx *Context) NoContent(code int) {
	ctx.Status(code)
}

// String writes the given string into the response body.
func (ctx *Context) String(code int, format string, values ...interface{}) {
	ctx.ginContext.String(code, fmt.Sprintf(format, values...))
}

// HTML sends an HTTP response with status code.
func (ctx *Context) HTML(code int, html string) {
	ctx.Blob(code, SpringWeb.MIMETextHTMLCharsetUTF8, []byte(html))
}

// HTMLBlob sends an HTTP blob response with status code.
func (ctx *Context) HTMLBlob(code int, b []byte) {
	ctx.Blob(code, SpringWeb.MIMETextHTMLCharsetUTF8, b)
}

// JSON sends a JSON response with status code.
func (ctx *Context) JSON(code int, i interface{}) {
	ctx.ginContext.JSON(code, i)
}

// JSONPretty sends a pretty-print JSON with status code.
func (ctx *Context) JSONPretty(code int, i interface{}, indent string) {

	b, err := json.MarshalIndent(i, "", indent)
	SpringUtils.Panic(err).When(err != nil)

	ctx.Blob(code, SpringWeb.MIMEApplicationJSONCharsetUTF8, b)
}

// JSONBlob sends a JSON blob response with status code.
func (ctx *Context) JSONBlob(code int, b []byte) {
	ctx.Blob(code, SpringWeb.MIMEApplicationJSONCharsetUTF8, b)
}

func (ctx *Context) jsonPBlob(code int, callback string, data func(http.ResponseWriter) error) error {
	// NOTE: 这一段逻辑使用了 echo 的实现

	ctx.Header(SpringWeb.HeaderContentType, SpringWeb.MIMEApplicationJavaScriptCharsetUTF8)
	ctx.Status(code)

	response := ctx.ginContext.Writer

	if _, err := response.Write([]byte(callback + "(")); err != nil {
		return err
	}

	if err := data(response); err != nil {
		return err
	}

	if _, err := response.Write([]byte(");")); err != nil {
		return err
	}
	return nil
}

// JSONP sends a JSONP response with status code.
func (ctx *Context) JSONP(code int, callback string, i interface{}) {

	err := ctx.jsonPBlob(code, callback, func(response http.ResponseWriter) error {

		enc := json.NewEncoder(response)

		_, pretty := ctx.QueryParams()["pretty"]
		if pretty {
			enc.SetIndent("", "  ")
		}

		if err := enc.Encode(i); err != nil {
			return err
		}
		return nil
	})

	SpringUtils.Panic(err).When(err != nil)
}

// JSONPBlob sends a JSONP blob response with status code.
func (ctx *Context) JSONPBlob(code int, callback string, b []byte) {

	err := ctx.jsonPBlob(code, callback, func(response http.ResponseWriter) error {
		if _, err := response.Write(b); err != nil {
			return err
		}
		return nil
	})

	SpringUtils.Panic(err).When(err != nil)
}

// XML sends an XML response with status code.
func (ctx *Context) XML(code int, i interface{}) {
	ctx.ginContext.XML(code, i)
}

func (ctx *Context) xmlBlob(code int, data func(http.ResponseWriter) error) error {
	// NOTE: 这一段逻辑使用了 echo 的实现

	ctx.Header(SpringWeb.HeaderContentType, SpringWeb.MIMEApplicationJavaScriptCharsetUTF8)
	ctx.Status(code)

	response := ctx.ginContext.Writer

	if _, err := response.Write([]byte(xml.Header)); err != nil {
		return err
	}

	return data(response)
}

// XMLPretty sends a pretty-print XML with status code.
func (ctx *Context) XMLPretty(code int, i interface{}, indent string) {

	err := ctx.xmlBlob(code, func(response http.ResponseWriter) error {

		enc := xml.NewEncoder(response)
		if indent != "" {
			enc.Indent("", indent)
		}

		return enc.Encode(i)
	})

	SpringUtils.Panic(err).When(err != nil)
}

// XMLBlob sends an XML blob response with status code.
func (ctx *Context) XMLBlob(code int, b []byte) {

	err := ctx.xmlBlob(code, func(response http.ResponseWriter) error {
		_, err := response.Write(b)
		return err
	})

	SpringUtils.Panic(err).When(err != nil)
}

// Blob sends a blob response with status code and content type.
func (ctx *Context) Blob(code int, contentType string, b []byte) {
	// NOTE: 这一段逻辑使用了 echo 的实现

	ctx.Header(SpringWeb.HeaderContentType, contentType)
	ctx.Status(code)

	response := ctx.ginContext.Writer

	_, err := response.Write(b)
	SpringUtils.Panic(err).When(err != nil)
}

// Stream sends a streaming response with status code and content type.
func (ctx *Context) Stream(code int, contentType string, r io.Reader) {
	// NOTE: 这一段逻辑使用了 echo 的实现

	ctx.Header(SpringWeb.HeaderContentType, contentType)
	ctx.Status(code)

	_, err := io.Copy(ctx.ginContext.Writer, r)
	SpringUtils.Panic(err).When(err != nil)
}

func (ctx *Context) contentDisposition(file, name, dispositionType string) {
	// NOTE: 这一段逻辑使用了 echo 的实现

	s := fmt.Sprintf("%s; filename=%q", dispositionType, name)
	ctx.Header(SpringWeb.HeaderContentDisposition, s)
	ctx.File(file)
}

// File sends a response with the content of the file.
func (ctx *Context) File(file string) {
	ctx.ginContext.File(file)
}

// Attachment sends a response as attachment.
func (ctx *Context) Attachment(file string, name string) {
	ctx.contentDisposition(file, name, "attachment")
}

// Inline sends a response as inline.
func (ctx *Context) Inline(file string, name string) {
	ctx.contentDisposition(file, name, "inline")
}

// Redirect redirects the request to a provided URL with status code.
func (ctx *Context) Redirect(code int, url string) {
	ctx.ginContext.Redirect(code, url)
}

// SSEvent writes a Server-Sent Event into the body stream.
func (ctx *Context) SSEvent(name string, message interface{}) {
	ctx.ginContext.SSEvent(name, message)
}
