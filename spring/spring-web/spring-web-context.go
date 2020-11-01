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
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-spring/spring-logger"
)

// WebContextKey WebContext 和 NativeContext 相互转换的 Key
const WebContextKey = "@WebCtx"

// ErrorHandler 用户自定义错误处理函数
var ErrorHandler = func(webCtx WebContext, err *HttpError) {

	defer func() {
		if r := recover(); r != nil {
			webCtx.LogError(r)
		}
	}()

	if err.Internal == nil {
		webCtx.String(err.Code, err.Message)
	} else {
		webCtx.JSON(http.StatusOK, err.Internal)
	}
}

// HttpError represents an error that occurred while handling a request.
type HttpError struct {
	Code     int         // HTTP 错误码
	Message  string      // 自定义错误消息
	Internal interface{} // 保存的原始异常
}

// NewHttpError creates a new HttpError instance.
func NewHttpError(code int, message ...string) *HttpError {
	e := &HttpError{Code: code}
	if len(message) > 0 {
		e.Message = message[0]
	} else {
		e.Message = http.StatusText(code)
	}
	return e
}

// Error makes it compatible with `error` interface.
func (e *HttpError) Error() string {
	if e.Internal == nil {
		return fmt.Sprintf("code=%d, message=%s", e.Code, e.Message)
	} else {
		return fmt.Sprintf("code=%d, message=%s, error=%v", e.Code, e.Message, e.Internal)
	}
}

// SetInternal sets error to HTTPError.Internal
func (e *HttpError) SetInternal(err error) *HttpError {
	e.Internal = err
	return e
}

// ResponseWriter Override http.ResponseWriter to supply more method.
type ResponseWriter interface {
	http.ResponseWriter

	// Returns the HTTP response status code of the current request.
	Status() int

	// Returns the number of bytes already written into the response http body.
	Size() int

	// 返回发送给客户端的数据，当前仅支持 MIMEApplicationJSON 格式.
	Body() []byte
}

// WebContext 上下文接口，设计理念：为社区中优秀的 Web 服务器提供一个抽象层，
// 使得底层可以灵活切换，因此在功能上取这些 Web 服务器功能的交集，同时提供获取
// 底层对象的接口，以便在不能满足用户要求的时候使用底层实现的能力，当然要慎用。
type WebContext interface {
	/////////////////////////////////////////
	// 通用能力部分

	// LoggerContext 日志接口上下文
	SpringLogger.LoggerContext

	// SetLoggerContext 设置日志接口上下文对象
	SetLoggerContext(logCtx SpringLogger.LoggerContext)

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

	// NoContent sends a response with no body and a status code. Maybe panic.
	NoContent(code int)

	// String writes the given string into the response body. Maybe panic.
	String(code int, format string, values ...interface{})

	// HTML sends an HTTP response with status code. Maybe panic.
	HTML(code int, html string)

	// HTMLBlob sends an HTTP blob response with status code. Maybe panic.
	HTMLBlob(code int, b []byte)

	// JSON sends a JSON response with status code. Maybe panic.
	JSON(code int, i interface{})

	// JSONPretty sends a pretty-print JSON with status code. Maybe panic.
	JSONPretty(code int, i interface{}, indent string)

	// JSONBlob sends a JSON blob response with status code. Maybe panic.
	JSONBlob(code int, b []byte)

	// JSONP sends a JSONP response with status code. It uses `callback`
	// to construct the JSONP payload. Maybe panic.
	JSONP(code int, callback string, i interface{})

	// JSONPBlob sends a JSONP blob response with status code. It uses
	// `callback` to construct the JSONP payload. Maybe panic.
	JSONPBlob(code int, callback string, b []byte)

	// XML sends an XML response with status code. Maybe panic.
	XML(code int, i interface{})

	// XMLPretty sends a pretty-print XML with status code. Maybe panic.
	XMLPretty(code int, i interface{}, indent string)

	// XMLBlob sends an XML blob response with status code. Maybe panic.
	XMLBlob(code int, b []byte)

	// Blob sends a blob response with status code and content type. Maybe panic.
	Blob(code int, contentType string, b []byte)

	// File sends a response with the content of the file. Maybe panic.
	File(file string)

	// Attachment sends a response as attachment, prompting client to
	// save the file. Maybe panic.
	Attachment(file string, name string)

	// Inline sends a response as inline, opening the file in the browser. Maybe panic.
	Inline(file string, name string)

	// Redirect redirects the request to a provided URL with status code. Maybe panic.
	Redirect(code int, url string)

	// SSEvent writes a Server-Sent Event into the body stream. Maybe panic.
	SSEvent(name string, message interface{})
}

// BufferedResponseWriter http.ResponseWriter 的一种增强型实现.
type BufferedResponseWriter struct {
	http.ResponseWriter
	buffer bytes.Buffer
	status int
	size   int
}

func (w *BufferedResponseWriter) Status() int {
	return w.status
}

func (w *BufferedResponseWriter) Size() int {
	return w.size
}

func (w *BufferedResponseWriter) Body() []byte {
	return w.buffer.Bytes()
}

func (w *BufferedResponseWriter) WriteHeader(statusCode int) {
	if statusCode > 0 { // TODO 加重复设置的告警日志
		w.ResponseWriter.WriteHeader(statusCode)
		w.status = statusCode
	}
}

func filterFlags(content string) string {
	for i, char := range strings.ToLower(content) {
		if char == ' ' || char == ';' {
			return content[:i]
		}
	}
	return content
}

func canPrintResponse(response http.ResponseWriter) bool {
	switch filterFlags(response.Header().Get(HeaderContentType)) {
	case MIMEApplicationJSON, MIMEApplicationXML, MIMETextPlain, MIMETextXML:
		return true
	case MIMEApplicationJavaScript, MIMETextHTML:
		return true
	}
	return false
}

func (w *BufferedResponseWriter) Write(data []byte) (n int, err error) {
	if n, err = w.ResponseWriter.Write(data); err == nil {
		if canPrintResponse(w.ResponseWriter) {
			w.buffer.Write(data[:n])
		}
	}
	w.size += n
	return
}
