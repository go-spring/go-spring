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
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-spring/spring-boost/log"
)

// ContextKey Context 和 NativeContext 相互转换的 Key
const ContextKey = "@WebCtx"

// ErrorHandler 用户自定义错误处理函数
var ErrorHandler = func(ctx Context, err *HttpError) {

	defer func() {
		if r := recover(); r != nil {
			log.Ctx(ctx.Context()).Error(r)
		}
	}()

	if err.Internal == nil {
		ctx.Status(err.Code)
		ctx.String(err.Message)
	} else {
		ctx.JSON(err.Internal)
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
	}
	return fmt.Sprintf("code=%d, message=%s, error=%v", e.Code, e.Message, e.Internal)
}

// SetInternal sets error to HTTPError.Internal
func (e *HttpError) SetInternal(err error) *HttpError {
	e.Internal = err
	return e
}

// ResponseWriter Override http.ResponseWriter to supply more method.
type ResponseWriter interface {
	http.ResponseWriter

	// Status Returns the HTTP response status code of the current request.
	Status() int

	// Size Returns the number of bytes already written into the response http body.
	Size() int

	// Body 返回发送给客户端的数据，当前仅支持 MIMEApplicationJSON 格式.
	Body() []byte
}

// Context 封装 *http.Request 和 http.ResponseWriter 对象，简化操作接口。
type Context interface {

	// NativeContext 返回封装的底层上下文对象
	NativeContext() interface{}

	/////////////////////////////////////////
	// Request Part

	// Request returns `*http.Request`.
	Request() *http.Request

	// SetRequest sets `*http.Request`.
	SetRequest(r *http.Request)

	// Context 返回 Request 绑定的 context.Context 对象
	Context() context.Context

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
	String(format string, values ...interface{})

	// HTML sends an HTTP response. Maybe panic.
	HTML(html string)

	// HTMLBlob sends an HTTP blob response. Maybe panic.
	HTMLBlob(b []byte)

	// JSON sends a JSON response. Maybe panic.
	JSON(i interface{})

	// JSONPretty sends a pretty-print JSON. Maybe panic.
	JSONPretty(i interface{}, indent string)

	// JSONBlob sends a JSON blob response. Maybe panic.
	JSONBlob(b []byte)

	// JSONP sends a JSONP response. It uses `callback`
	// to construct the JSONP payload. Maybe panic.
	JSONP(callback string, i interface{})

	// JSONPBlob sends a JSONP blob response. It uses
	// `callback` to construct the JSONP payload. Maybe panic.
	JSONPBlob(callback string, b []byte)

	// XML sends an XML response. Maybe panic.
	XML(i interface{})

	// XMLPretty sends a pretty-print XML. Maybe panic.
	XMLPretty(i interface{}, indent string)

	// XMLBlob sends an XML blob response. Maybe panic.
	XMLBlob(b []byte)

	// Blob sends a blob response and content type. Maybe panic.
	Blob(contentType string, b []byte)

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
	size   int
}

func (w *BufferedResponseWriter) Size() int { return w.size }

func (w *BufferedResponseWriter) Body() []byte { return w.buffer.Bytes() }

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
