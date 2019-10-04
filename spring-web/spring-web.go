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
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"github.com/didi/go-spring/spring-trace"
	_ "github.com/didi/go-spring/spring-http" // 导入默认的 Web 服务器
)

var UNSUPPORTED_METHOD = errors.New("unsupported method")

type Handler func(WebContext)

//
// Web 上下文接口，设计理念：为社区 top3 的 web 服务器提供一个抽象层，使得底层
// 可以灵活切换，因此在功能上优先取这些服务器功能的交集，同时提供获取底层对象的接
// 口，以便在框架不能满足用户要求的时候使用底层框架的能力，当然这种功能要慎用。
//
type WebContext interface {
	/////////////////////////////////////////
	// 通用能力部分

	SpringTrace.TraceContext

	// 获取封装的底层上下文对象
	NativeContext() interface{}

	// Get retrieves data from the context.
	Get(key string) interface{}

	// Set saves data in the context.
	Set(key string, val interface{})

	/////////////////////////////////////////
	// Request Part

	// Request returns `*http.Request`.
	Request() *http.Request

	// IsTLS returns true if HTTP connection is TLS otherwise false.
	IsTLS() bool

	// IsWebSocket returns true if HTTP connection is WebSocket otherwise false.
	IsWebSocket() bool

	// Scheme returns the HTTP protocol scheme, `http` or `https`.
	Scheme() string

	// ClientIP implements a best effort algorithm to return the real client IP, it parses
	// X-Real-IP and X-Forwarded-For in order to work properly with reverse-proxies such us: nginx or haproxy.
	// Use X-Forwarded-For before X-Real-Ip as nginx uses X-Real-Ip with the proxy's IP.
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

	// Param returns path parameter by name.
	Param(name string) string

	// ParamNames returns path parameter names.
	ParamNames() []string

	// ParamValues returns path parameter values.
	ParamValues() []string

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

	// Header is a intelligent shortcut for c.Writer.Header().Set(key, value).
	// It writes a header in the response.
	// If value == "", this method removes the header `c.Writer.Header().Del(key)`
	Header(key, value string)

	// SetAccepted sets Accept header data.
	SetAccepted(formats ...string)

	// Render renders a template with data and sends a text/html response with status
	// code. Renderer must be registered using `Echo.Renderer`.
	Render(code int, name string, data interface{}) error

	// HTML sends an HTTP response with status code.
	HTML(code int, html string) error

	// HTMLBlob sends an HTTP blob response with status code.
	HTMLBlob(code int, b []byte) error

	// String writes the given string into the response body.
	String(code int, format string, values ...interface{})

	// JSON sends a JSON response with status code.
	JSON(code int, i interface{}) error

	// JSONPretty sends a pretty-print JSON with status code.
	JSONPretty(code int, i interface{}, indent string) error

	// JSONBlob sends a JSON blob response with status code.
	JSONBlob(code int, b []byte) error

	// JSONP sends a JSONP response with status code. It uses `callback` to construct
	// the JSONP payload.
	JSONP(code int, callback string, i interface{}) error

	// JSONPBlob sends a JSONP blob response with status code. It uses `callback`
	// to construct the JSONP payload.
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

	// Attachment sends a response as attachment, prompting client to save the
	// file.
	Attachment(file string, name string) error

	// Inline sends a response as inline, opening the file in the browser.
	Inline(file string, name string) error

	// NoContent sends a response with no body and a status code.
	NoContent(code int) error

	// Redirect redirects the request to a provided URL with status code.
	Redirect(code int, url string) error

	// Error invokes the registered HTTP error handler. Generally used by middleware.
	Error(err error)

	// Data writes some data into the body stream and updates the HTTP code.
	Data(code int, contentType string, data []byte)

	// SSEvent writes a Server-Sent Event into the body stream.
	SSEvent(name string, message interface{})

	// SetCookie adds a `Set-Cookie` header in HTTP response.
	SetCookie(cookie *http.Cookie)
}

//
// Web 容器接口
//
type WebContainer interface {
	Stop()

	Start(address string) error
	StartTLS(address string, certFile, keyFile string) error

	Register(method string, path string, fn Handler)
}

//
// Web Bean 初始化接口
//
type WebBeanInitialization interface {
	InitWebBean(c WebContainer)
}

//
// 定义 WebContainer 的工厂函数
//
type Factory func() WebContainer

//
// 保存 WebContainer 的工厂函数
//
var WebContainerFactory Factory

//
// 注册 WebContainer 的工厂函数
//
func RegisterWebContainer(fn Factory) {
	WebContainerFactory = fn
}
