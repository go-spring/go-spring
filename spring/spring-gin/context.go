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
	"context"
	"encoding/xml"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-spring/spring-boost/json"
	"github.com/go-spring/spring-boost/knife"
	"github.com/go-spring/spring-core/validator"
	"github.com/go-spring/spring-core/web"
)

const (
	defaultMemory = 32 << 20 // 32 MB
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
	writer *web.BufferedResponseWriter
}

func (w *responseWriter) Size() int {
	return w.writer.Size()
}

func (w *responseWriter) Body() []byte {
	return w.writer.Body()
}

func (w *responseWriter) Write(data []byte) (n int, err error) {
	return w.writer.Write(data)
}

// Context 适配 gin 的 Web 上下文
type Context struct {

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

	{
		req := ginCtx.Request
		ctx := knife.New(req.Context())
		ginCtx.Request = req.WithContext(ctx)
	}

	ginCtx.Writer = &responseWriter{
		writer: &web.BufferedResponseWriter{
			ResponseWriter: ginCtx.Writer,
		},
		ResponseWriter: ginCtx.Writer,
	}

	webCtx := &Context{
		handlerFunc:  fn,
		ginContext:   ginCtx,
		wildCardName: wildCardName,
	}

	ginCtx.Set(web.ContextKey, webCtx)
	return webCtx
}

// NativeContext 返回封装的底层上下文对象
func (ctx *Context) NativeContext() interface{} {
	return ctx.ginContext
}

// Request returns `*http.Request`.
func (ctx *Context) Request() *http.Request {
	return ctx.ginContext.Request
}

// SetRequest sets `*http.Request`.
func (ctx *Context) SetRequest(r *http.Request) {
	ctx.ginContext.Request = r
}

// Context 返回 Request 绑定的 context.Context 对象
func (ctx *Context) Context() context.Context {
	return ctx.ginContext.Request.Context()
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

	if scheme := r.Header.Get(web.HeaderXForwardedProto); scheme != "" {
		return scheme
	}

	if scheme := r.Header.Get(web.HeaderXForwardedProtocol); scheme != "" {
		return scheme
	}

	if ssl := r.Header.Get(web.HeaderXForwardedSsl); ssl == "on" {
		return "https"
	}

	if scheme := r.Header.Get(web.HeaderXUrlScheme); scheme != "" {
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
func (ctx *Context) Handler() web.Handler {
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

	if strings.HasPrefix(ctx.ContentType(), web.MIMEMultipartForm) {
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
	err := ctx.ginContext.ShouldBind(i)
	if err != nil {
		return err
	}
	return validator.Validate(i)
}

// ResponseWriter returns `http.ResponseWriter`.
func (ctx *Context) ResponseWriter() web.ResponseWriter {
	return ctx.ginContext.Writer.(*responseWriter)
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
	ctx.ginContext.Status(code)
}

// String writes the given string into the response body.
func (ctx *Context) String(format string, values ...interface{}) {
	statusCode := ctx.ginContext.Writer.Status()
	ctx.ginContext.String(statusCode, fmt.Sprintf(format, values...))
}

// HTML sends an HTTP response.
func (ctx *Context) HTML(html string) {
	statusCode := ctx.ginContext.Writer.Status()
	ctx.ginContext.Data(statusCode, web.MIMETextHTMLCharsetUTF8, []byte(html))
}

// HTMLBlob sends an HTTP blob response.
func (ctx *Context) HTMLBlob(b []byte) {
	statusCode := ctx.ginContext.Writer.Status()
	ctx.ginContext.Data(statusCode, web.MIMETextHTMLCharsetUTF8, b)
}

// JSON sends a JSON response.
func (ctx *Context) JSON(i interface{}) {
	statusCode := ctx.ginContext.Writer.Status()
	ctx.ginContext.JSON(statusCode, i)
}

// JSONPretty sends a pretty-print JSON.
func (ctx *Context) JSONPretty(i interface{}, indent string) {
	b, err := json.MarshalIndent(i, "", indent)
	if err != nil {
		panic(err)
	}
	statusCode := ctx.ginContext.Writer.Status()
	ctx.ginContext.Data(statusCode, web.MIMEApplicationJSONCharsetUTF8, b)
}

// JSONBlob sends a JSON blob response.
func (ctx *Context) JSONBlob(b []byte) {
	statusCode := ctx.ginContext.Writer.Status()
	ctx.ginContext.Data(statusCode, web.MIMEApplicationJSONCharsetUTF8, b)
}

func (ctx *Context) jsonPBlob(code int, callback string, data func(http.ResponseWriter) error) {
	rw := ctx.ginContext.Writer

	ctx.Header(web.HeaderContentType, web.MIMEApplicationJavaScriptCharsetUTF8)
	ctx.Status(code)

	if _, err := rw.Write([]byte(callback + "(")); err != nil {
		panic(err)
	}

	if err := data(rw); err != nil {
		panic(err)
	}

	if _, err := rw.Write([]byte(");")); err != nil {
		panic(err)
	}
}

// JSONP sends a JSONP response.
func (ctx *Context) JSONP(callback string, i interface{}) {
	statusCode := ctx.ginContext.Writer.Status()
	ctx.jsonPBlob(statusCode, callback, func(response http.ResponseWriter) error {
		var (
			data []byte
			err  error
		)
		if _, pretty := ctx.QueryParams()["pretty"]; pretty {
			data, err = json.MarshalIndent(i, "", "  ")
		} else {
			data, err = json.Marshal(i)
		}
		if err == nil {
			return err
		}
		_, err = response.Write(data)
		return err
	})
}

// JSONPBlob sends a JSONP blob response.
func (ctx *Context) JSONPBlob(callback string, b []byte) {
	statusCode := ctx.ginContext.Writer.Status()
	ctx.jsonPBlob(statusCode, callback, func(response http.ResponseWriter) error {
		_, err := response.Write(b)
		return err
	})
}

// XML sends an XML response.
func (ctx *Context) XML(i interface{}) {
	statusCode := ctx.ginContext.Writer.Status()
	ctx.ginContext.XML(statusCode, i)
}

func (ctx *Context) xmlBlob(code int, data func(http.ResponseWriter) error) {
	rw := ctx.ginContext.Writer

	ctx.Header(web.HeaderContentType, web.MIMEApplicationXMLCharsetUTF8)
	ctx.Status(code)

	if _, err := rw.Write([]byte(xml.Header)); err != nil {
		panic(err)
	}

	if err := data(rw); err != nil {
		panic(err)
	}
}

// XMLPretty sends a pretty-print XML.
func (ctx *Context) XMLPretty(i interface{}, indent string) {
	statusCode := ctx.ginContext.Writer.Status()
	ctx.xmlBlob(statusCode, func(rw http.ResponseWriter) error {
		enc := xml.NewEncoder(rw)
		if indent != "" {
			enc.Indent("", indent)
		}
		return enc.Encode(i)
	})
}

// XMLBlob sends an XML blob response.
func (ctx *Context) XMLBlob(b []byte) {
	statusCode := ctx.ginContext.Writer.Status()
	ctx.xmlBlob(statusCode, func(rw http.ResponseWriter) error {
		_, err := rw.Write(b)
		return err
	})
}

// Blob sends a blob response with content type.
func (ctx *Context) Blob(contentType string, b []byte) {
	statusCode := ctx.ginContext.Writer.Status()
	ctx.ginContext.Data(statusCode, contentType, b)
}

// File sends a response with the content of the file.
func (ctx *Context) File(file string) {
	ctx.ginContext.File(file)
}

func (ctx *Context) contentDisposition(file, name, dispositionType string) {
	s := fmt.Sprintf("%s; filename=%q", dispositionType, name)
	ctx.Header(web.HeaderContentDisposition, s)
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
