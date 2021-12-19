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

package web

import (
	"context"
	"mime/multipart"
	"net/http"
	"net/url"

	"github.com/go-spring/spring-base/knife"
	"github.com/go-spring/spring-base/util"
)

// Context 适配 echo 的 Web 上下文
type httpContext struct {
	h Handler
	r *http.Request
	w ResponseWriter

	query url.Values
}

func NewHttpContext(h Handler, w http.ResponseWriter, r *http.Request) *httpContext {
	req := r.WithContext(knife.New(r.Context()))
	bufRW := &BufferedResponseWriter{ResponseWriter: w}
	return &httpContext{h: h, r: req, w: bufRW}
}

// NativeContext 返回封装的底层上下文对象
func (ctx *httpContext) NativeContext() interface{} {
	return nil
}

// Get retrieves data from the context.
func (ctx *httpContext) Get(key string) interface{} {
	v, _ := knife.Get(ctx.Context(), key)
	return v
}

// Set saves data in the context.
func (ctx *httpContext) Set(key string, val interface{}) {
	err := knife.Set(ctx.Context(), key, val)
	util.Panic(err).When(err != nil)
}

// Request returns `*http.Request`.
func (ctx *httpContext) Request() *http.Request {
	return ctx.r
}

// SetRequest sets `*http.Request`.
func (ctx *httpContext) SetRequest(r *http.Request) {
	ctx.r = r
}

// Context 返回 Request 绑定的 context.Context 对象
func (ctx *httpContext) Context() context.Context {
	return ctx.r.Context()
}

// IsTLS returns true if HTTP connection is TLS otherwise false.
func (ctx *httpContext) IsTLS() bool {
	panic(util.UnimplementedMethod)
}

// IsWebSocket returns true if HTTP connection is WebSocket otherwise false.
func (ctx *httpContext) IsWebSocket() bool {
	panic(util.UnimplementedMethod)
}

// Scheme returns the HTTP protocol scheme, `http` or `https`.
func (ctx *httpContext) Scheme() string {
	panic(util.UnimplementedMethod)
}

// ClientIP implements a best effort algorithm to return the real client IP.
func (ctx *httpContext) ClientIP() string {
	panic(util.UnimplementedMethod)
}

// Path returns the registered path for the handler.
func (ctx *httpContext) Path() string {
	panic(util.UnimplementedMethod)
}

// Handler returns the matched handler by router.
func (ctx *httpContext) Handler() Handler {
	return ctx.h
}

// ContentType returns the Content-Type header of the request.
func (ctx *httpContext) ContentType() string {
	panic(util.UnimplementedMethod)
}

// GetHeader returns value from request headers.
func (ctx *httpContext) GetHeader(key string) string {
	return ctx.r.Header.Get(key)
}

// GetRawData return stream data.
func (ctx *httpContext) GetRawData() ([]byte, error) {
	panic(util.UnimplementedMethod)
}

// PathParam returns path parameter by name.
func (ctx *httpContext) PathParam(name string) string {
	panic(util.UnimplementedMethod)
}

// PathParamNames returns path parameter names.
func (ctx *httpContext) PathParamNames() []string {
	panic(util.UnimplementedMethod)
}

// PathParamValues returns path parameter values.
func (ctx *httpContext) PathParamValues() []string {
	panic(util.UnimplementedMethod)
}

// QueryParam returns the query param for the provided name.
func (ctx *httpContext) QueryParam(name string) string {
	if ctx.query == nil {
		ctx.query = ctx.r.URL.Query()
	}
	return ctx.query.Get(name)
}

// QueryParams returns the query parameters as `url.Values`.
func (ctx *httpContext) QueryParams() url.Values {
	panic(util.UnimplementedMethod)
}

// QueryString returns the URL query string.
func (ctx *httpContext) QueryString() string {
	panic(util.UnimplementedMethod)
}

// FormValue returns the form field value for the provided name.
func (ctx *httpContext) FormValue(name string) string {
	panic(util.UnimplementedMethod)
}

// FormParams returns the form parameters as `url.Values`.
func (ctx *httpContext) FormParams() (url.Values, error) {
	panic(util.UnimplementedMethod)
}

// FormFile returns the multipart form file for the provided name.
func (ctx *httpContext) FormFile(name string) (*multipart.FileHeader, error) {
	panic(util.UnimplementedMethod)
}

// SaveUploadedFile uploads the form file to specific dst.
func (ctx *httpContext) SaveUploadedFile(file *multipart.FileHeader, dst string) error {
	panic(util.UnimplementedMethod)
}

// MultipartForm returns the multipart form.
func (ctx *httpContext) MultipartForm() (*multipart.Form, error) {
	panic(util.UnimplementedMethod)
}

// Cookie returns the named cookie provided in the request.
func (ctx *httpContext) Cookie(name string) (*http.Cookie, error) {
	panic(util.UnimplementedMethod)
}

// Cookies returns the HTTP cookies sent with the request.
func (ctx *httpContext) Cookies() []*http.Cookie {
	panic(util.UnimplementedMethod)
}

// Bind binds the request body into provided type `i`.
func (ctx *httpContext) Bind(i interface{}) error {
	panic(util.UnimplementedMethod)
}

// ResponseWriter returns `http.ResponseWriter`.
func (ctx *httpContext) ResponseWriter() ResponseWriter {
	return ctx.w
}

// Status sets the HTTP response code.
func (ctx *httpContext) Status(code int) {
	panic(util.UnimplementedMethod)
}

// Header is a intelligent shortcut for c.Writer.Header().Set(key, value).
func (ctx *httpContext) Header(key, value string) {
	ctx.w.Header().Set(key, value)
}

// SetCookie adds a `Set-Cookie` header in HTTP response.
func (ctx *httpContext) SetCookie(cookie *http.Cookie) {
	panic(util.UnimplementedMethod)
}

// NoContent sends a response with no body and a status code.
func (ctx *httpContext) NoContent(code int) {
	panic(util.UnimplementedMethod)
}

// String writes the given string into the response body.
func (ctx *httpContext) String(format string, values ...interface{}) {
	panic(util.UnimplementedMethod)
}

// HTML sends an HTTP response.
func (ctx *httpContext) HTML(html string) {
	panic(util.UnimplementedMethod)
}

// HTMLBlob sends an HTTP blob response.
func (ctx *httpContext) HTMLBlob(b []byte) {
	panic(util.UnimplementedMethod)
}

// JSON sends a JSON response.
func (ctx *httpContext) JSON(i interface{}) {
	panic(util.UnimplementedMethod)
}

// JSONPretty sends a pretty-print JSON.
func (ctx *httpContext) JSONPretty(i interface{}, indent string) {
	panic(util.UnimplementedMethod)
}

// JSONBlob sends a JSON blob response.
func (ctx *httpContext) JSONBlob(b []byte) {
	panic(util.UnimplementedMethod)
}

// JSONP sends a JSONP response.
func (ctx *httpContext) JSONP(callback string, i interface{}) {
	panic(util.UnimplementedMethod)
}

// JSONPBlob sends a JSONP blob response.
func (ctx *httpContext) JSONPBlob(callback string, b []byte) {
	panic(util.UnimplementedMethod)
}

// XML sends an XML response.
func (ctx *httpContext) XML(i interface{}) {
	panic(util.UnimplementedMethod)
}

// XMLPretty sends a pretty-print XML.
func (ctx *httpContext) XMLPretty(i interface{}, indent string) {
	panic(util.UnimplementedMethod)
}

// XMLBlob sends an XML blob response.
func (ctx *httpContext) XMLBlob(b []byte) {
	panic(util.UnimplementedMethod)
}

// Blob sends a blob response with content type.
func (ctx *httpContext) Blob(contentType string, b []byte) {
	panic(util.UnimplementedMethod)
}

// File sends a response with the content of the file.
func (ctx *httpContext) File(file string) {
	panic(util.UnimplementedMethod)
}

// Attachment sends a response as attachment
func (ctx *httpContext) Attachment(file string, name string) {
	panic(util.UnimplementedMethod)
}

// Inline sends a response as inline
func (ctx *httpContext) Inline(file string, name string) {
	panic(util.UnimplementedMethod)
}

// Redirect redirects the request to a provided URL with status code.
func (ctx *httpContext) Redirect(code int, url string) {
	panic(util.UnimplementedMethod)
}

// SSEvent writes a Server-Sent Event into the body stream.
func (ctx *httpContext) SSEvent(name string, message interface{}) {
	panic(util.UnimplementedMethod)
}
