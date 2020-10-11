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

package SpringWeb

import (
	"io"
	"mime/multipart"
	"net/http"
	"net/url"

	"github.com/go-spring/spring-logger"
)

// WebContextKey WebContext 和 NativeContext 相互转换的 Key
const WebContextKey = "@WebCtx"

// ResponseWriter Override http.ResponseWriter to supply more method.
type ResponseWriter interface {
	http.ResponseWriter

	// Returns the HTTP response status code of the current request.
	Status() int

	// Returns the number of bytes already written into the response http body.
	// See Written()
	Size() int
}

// WebContext 上下文接口，设计理念：为社区中优秀的 Web 服务器提供一个抽象层，
// 使得底层可以灵活切换，因此在功能上取这些 Web 服务器功能的交集，同时提供获取
// 底层对象的接口，以便在不能满足用户要求的时候使用底层实现的能力，当然要慎用。
type WebContext interface {
	/////////////////////////////////////////
	// 通用能力部分

	// LoggerContext 日志接口上下文
	SpringLogger.LoggerContext

	// NativeContext 返回封装的底层上下文对象
	NativeContext() interface{}

	// Get retrieves data from the context.
	Get(key string) interface{}

	// Set saves data in the context.
	Set(key string, val interface{})

	/////////////////////////////////////////
	// Request Part

	// Request returns `*http.Request`.
	Request() *http.Request

	// SetRequest sets `*http.Request`.
	SetRequest(r *http.Request)

	// IsTLS returns true if HTTP connection is TLS otherwise false.
	IsTLS() bool

	// IsWebSocket returns true if HTTP connection is WebSocket otherwise false.
	IsWebSocket() bool

	// Scheme returns the HTTP protocol scheme, `http` or `https`.
	Scheme() string

	// ClientIP implements a best effort algorithm to return the real client IP,
	// it parses X-Real-IP and X-Forwarded-For in order to work properly with
	// reverse-proxies such us: nginx or haproxy. Use X-Forwarded-For before
	// X-Real-Ip as nginx uses X-Real-Ip with the proxy's IP.
	ClientIP() string

	// Path returns the registered path for the handler.
	Path() string

	// Handler returns the matched handler by router.
	Handler() Handler

	// ContentType returns the Content-Type header of the request.
	ContentType() string

	// GetHeader returns value from request headers.
	GetHeader(key string) string

	// GetRawData return stream data.
	GetRawData() ([]byte, error)

	// PathParam returns path parameter by name.
	PathParam(name string) string

	// PathParamNames returns path parameter names.
	PathParamNames() []string

	// PathParamValues returns path parameter values.
	PathParamValues() []string

	// QueryParam returns the query param for the provided name.
	QueryParam(name string) string

	// QueryParams returns the query parameters as `url.Values`.
	QueryParams() url.Values

	// QueryString returns the URL query string.
	QueryString() string

	// FormValue returns the form field value for the provided name.
	FormValue(name string) string

	// FormParams returns the form parameters as `url.Values`.
	FormParams() (url.Values, error)

	// FormFile returns the multipart form file for the provided name.
	FormFile(name string) (*multipart.FileHeader, error)

	// SaveUploadedFile uploads the form file to specific dst.
	SaveUploadedFile(file *multipart.FileHeader, dst string) error

	// MultipartForm returns the multipart form.
	MultipartForm() (*multipart.Form, error)

	// Cookie returns the named cookie provided in the request.
	Cookie(name string) (*http.Cookie, error)

	// Cookies returns the HTTP cookies sent with the request.
	Cookies() []*http.Cookie

	// Bind binds the request body into provided type `i`. The default binder
	// does it based on Content-Type header.
	Bind(i interface{}) error

	/////////////////////////////////////////
	// Response Part

	// IsAborted 当前处理过程是否终止，为了适配 gin 的模型，未来底层统一了会去掉.
	IsAborted() bool

	// Abort 终止当前处理过程，为了适配 gin 的模型，未来底层统一了会去掉.
	Abort()

	// ResponseWriter returns `http.ResponseWriter`.
	ResponseWriter() ResponseWriter

	// Status sets the HTTP response code.
	Status(code int)

	// Header is a intelligent shortcut for c.Writer.Header().Set(key, value).
	// It writes a header in the response.
	// If value == "", this method removes the header `c.Writer.Header().Del(key)`
	Header(key, value string)

	// SetCookie adds a `Set-Cookie` header in HTTP response.
	SetCookie(cookie *http.Cookie)

	// NoContent sends a response with no body and a status code.
	NoContent(code int)

	// String writes the given string into the response body.
	String(code int, format string, values ...interface{}) error

	// HTML sends an HTTP response with status code.
	HTML(code int, html string) error

	// HTMLBlob sends an HTTP blob response with status code.
	HTMLBlob(code int, b []byte) error

	// JSON sends a JSON response with status code.
	JSON(code int, i interface{}) error

	// JSONPretty sends a pretty-print JSON with status code.
	JSONPretty(code int, i interface{}, indent string) error

	// JSONBlob sends a JSON blob response with status code.
	JSONBlob(code int, b []byte) error

	// JSONP sends a JSONP response with status code. It uses `callback`
	// to construct the JSONP payload.
	JSONP(code int, callback string, i interface{}) error

	// JSONPBlob sends a JSONP blob response with status code. It uses
	// `callback` to construct the JSONP payload.
	JSONPBlob(code int, callback string, b []byte) error

	// XML sends an XML response with status code.
	XML(code int, i interface{}) error

	// XMLPretty sends a pretty-print XML with status code.
	XMLPretty(code int, i interface{}, indent string) error

	// XMLBlob sends an XML blob response with status code.
	XMLBlob(code int, b []byte) error

	// Blob sends a blob response with status code and content type.
	Blob(code int, contentType string, b []byte) error

	// Stream sends a streaming response with status code and content type.
	Stream(code int, contentType string, r io.Reader) error

	// File sends a response with the content of the file.
	File(file string) error

	// Attachment sends a response as attachment, prompting client to save the file.
	Attachment(file string, name string) error

	// Inline sends a response as inline, opening the file in the browser.
	Inline(file string, name string) error

	// Redirect redirects the request to a provided URL with status code.
	Redirect(code int, url string) error

	// SSEvent writes a Server-Sent Event into the body stream.
	SSEvent(name string, message interface{}) error
}
