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
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/go-spring/spring-base/knife"
	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-base/util"
)

type BaseContext struct {
	Logger *log.Logger

	w Response
	r *http.Request

	path    string
	handler Handler
	query   url.Values
}

// NewBaseContext 创建 *BaseContext 对象。
func NewBaseContext(path string, handler Handler, r *http.Request, w Response) *BaseContext {
	if ctx, cached := knife.New(r.Context()); !cached {
		r = r.WithContext(ctx)
	}
	ret := &BaseContext{r: r, w: w, path: path, handler: handler}
	ret.Logger = log.GetLogger(util.TypeName(ret))
	return ret
}

// NativeContext 返回封装的底层上下文对象
func (c *BaseContext) NativeContext() interface{} {
	return nil
}

// Get retrieves data from the context.
func (c *BaseContext) Get(key string) interface{} {
	v, err := knife.Load(c.Context(), key)
	if err != nil {
		c.Logger.WithContext(c.Context()).Error(err)
		return nil
	}
	return v
}

// Set saves data in the context.
func (c *BaseContext) Set(key string, val interface{}) error {
	return knife.Store(c.Context(), key, val)
}

// Request returns `*http.Request`.
func (c *BaseContext) Request() *http.Request {
	return c.r
}

// SetContext sets context.Context.
func (c *BaseContext) SetContext(ctx context.Context) {
	c.r = c.r.WithContext(ctx)
}

// Context 返回 Request 绑定的 context.Context 对象
func (c *BaseContext) Context() context.Context {
	return c.r.Context()
}

// IsTLS returns true if HTTP connection is TLS otherwise false.
func (c *BaseContext) IsTLS() bool {
	return c.r.TLS != nil
}

// IsWebSocket returns true if HTTP connection is WebSocket otherwise false.
func (c *BaseContext) IsWebSocket() bool {
	upgrade := c.r.Header.Get(HeaderUpgrade)
	return strings.EqualFold(upgrade, "websocket")
}

// Scheme returns the HTTP protocol scheme, `http` or `https`.
func (c *BaseContext) Scheme() string {
	if c.IsTLS() {
		return "https"
	}
	if scheme := c.Header(HeaderXForwardedProto); scheme != "" {
		return scheme
	}
	if scheme := c.Header(HeaderXForwardedProtocol); scheme != "" {
		return scheme
	}
	if ssl := c.Header(HeaderXForwardedSsl); ssl == "on" {
		return "https"
	}
	if scheme := c.Header(HeaderXUrlScheme); scheme != "" {
		return scheme
	}
	return "http"
}

// ClientIP implements a best effort algorithm to return the real client IP.
func (c *BaseContext) ClientIP() string {
	if ip := c.Header(HeaderXForwardedFor); ip != "" {
		if i := strings.Index(ip, ","); i > 0 {
			return strings.TrimSpace(ip[:i])
		}
		return ip
	}

	if ip := c.Header(HeaderXRealIP); ip != "" {
		return ip
	}
	host, _, _ := net.SplitHostPort(c.r.RemoteAddr)
	return host
}

// Path returns the registered path for the handler.
func (c *BaseContext) Path() string {
	return c.path
}

// Handler returns the matched handler by router.
func (c *BaseContext) Handler() Handler {
	return c.handler
}

// ContentType returns the Content-Type header of the request.
func (c *BaseContext) ContentType() string {
	s := c.Header("Content-Type")
	return filterFlags(s)
}

// Header returns value from request headers.
func (c *BaseContext) Header(key string) string {
	return c.r.Header.Get(key)
}

// Cookies returns the HTTP cookies sent with the request.
func (c *BaseContext) Cookies() []*http.Cookie {
	return c.r.Cookies()
}

// Cookie returns the named cookie provided in the request.
func (c *BaseContext) Cookie(name string) (*http.Cookie, error) {
	return c.r.Cookie(name)
}

// PathParamNames returns path parameter names.
func (c *BaseContext) PathParamNames() []string {
	panic(util.UnimplementedMethod)
}

// PathParamValues returns path parameter values.
func (c *BaseContext) PathParamValues() []string {
	panic(util.UnimplementedMethod)
}

// PathParam returns path parameter by name.
func (c *BaseContext) PathParam(name string) string {
	panic(util.UnimplementedMethod)
}

// QueryString returns the URL query string.
func (c *BaseContext) QueryString() string {
	return c.r.URL.RawQuery
}

func (c *BaseContext) initQueryCache() {
	if c.query == nil {
		c.query = c.r.URL.Query()
	}
}

// QueryParams returns the query parameters as `url.Values`.
func (c *BaseContext) QueryParams() url.Values {
	c.initQueryCache()
	return c.query
}

// QueryParam returns the query param for the provided name.
func (c *BaseContext) QueryParam(name string) string {
	c.initQueryCache()
	return c.query.Get(name)
}

// FormParams returns the form parameters as `url.Values`.
func (c *BaseContext) FormParams() (url.Values, error) {
	if strings.HasPrefix(c.ContentType(), MIMEMultipartForm) {
		if _, err := c.MultipartForm(); err != nil {
			return nil, err
		}
	} else {
		if err := c.r.ParseForm(); err != nil {
			return nil, err
		}
	}
	return c.r.Form, nil
}

// FormValue returns the form field value for the provided name.
func (c *BaseContext) FormValue(name string) string {
	return c.r.FormValue(name)
}

// MultipartForm returns the multipart form.
func (c *BaseContext) MultipartForm() (*multipart.Form, error) {
	err := c.r.ParseMultipartForm(32 << 20 /* 32MB */)
	return c.r.MultipartForm, err
}

// FormFile returns the multipart form file for the provided name.
func (c *BaseContext) FormFile(name string) (*multipart.FileHeader, error) {
	f, fh, err := c.r.FormFile(name)
	if err != nil {
		return nil, err
	}
	f.Close()
	return fh, nil
}

// SaveUploadedFile uploads the form file to specific dst.
func (c *BaseContext) SaveUploadedFile(file *multipart.FileHeader, dst string) error {
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

// RequestBody return stream data.
func (c *BaseContext) RequestBody() ([]byte, error) {
	return ioutil.ReadAll(c.Request().Body)
}

// Bind binds the request body into provided type `i`.
func (c *BaseContext) Bind(i interface{}) error {
	return Bind(i, c)
}

// Response returns Response.
func (c *BaseContext) Response() Response {
	return c.w
}

// SetStatus sets the HTTP response code.
func (c *BaseContext) SetStatus(code int) {
	c.w.WriteHeader(code)
}

// SetHeader is a intelligent shortcut for c.Writer.Header().Set(key, value).
func (c *BaseContext) SetHeader(key, value string) {
	c.w.Header().Set(key, value)
}

// SetContentType 设置 ResponseWriter 的 ContentType 。
func (c *BaseContext) SetContentType(typ string) {
	c.SetHeader(HeaderContentType, typ)
}

// SetCookie adds a `Set-Cookie` header in HTTP response.
func (c *BaseContext) SetCookie(cookie *http.Cookie) {
	http.SetCookie(c.Response(), cookie)
}

// NoContent sends a response with no body and a status code.
func (c *BaseContext) NoContent(code int) {
	c.SetStatus(code)
}

// String writes the given string into the response body.
func (c *BaseContext) String(format string, values ...interface{}) {
	s := fmt.Sprintf(format, values...)
	c.Blob(MIMETextPlainCharsetUTF8, []byte(s))
}

// HTML sends an HTTP response.
func (c *BaseContext) HTML(html string) {
	c.HTMLBlob([]byte(html))
}

// HTMLBlob sends an HTTP blob response.
func (c *BaseContext) HTMLBlob(b []byte) {
	c.Blob(MIMETextHTMLCharsetUTF8, b)
}

// JSON sends a JSON response.
func (c *BaseContext) JSON(i interface{}) {
	var (
		b   []byte
		err error
	)
	if _, pretty := c.QueryParams()["pretty"]; pretty {
		b, err = json.MarshalIndent(i, "", "  ")
	} else {
		b, err = json.Marshal(i)
	}
	util.Panic(err).When(err != nil)
	c.Blob(MIMEApplicationJSONCharsetUTF8, b)
}

// JSONPretty sends a pretty-print JSON.
func (c *BaseContext) JSONPretty(i interface{}, indent string) {
	b, err := json.MarshalIndent(i, "", indent)
	util.Panic(err).When(err != nil)
	c.Blob(MIMEApplicationJSONCharsetUTF8, b)
}

// JSONBlob sends a JSON blob response.
func (c *BaseContext) JSONBlob(b []byte) {
	c.Blob(MIMEApplicationJSONCharsetUTF8, b)
}

func (c *BaseContext) jsonPBlob(callback string, data func(http.ResponseWriter) error) error {
	c.SetContentType(MIMEApplicationJavaScriptCharsetUTF8)
	if _, err := c.w.Write([]byte(callback + "(")); err != nil {
		return err
	}
	if err := data(c.w); err != nil {
		return err
	}
	if _, err := c.w.Write([]byte(");")); err != nil {
		return err
	}
	return nil
}

// JSONP sends a JSONP response.
func (c *BaseContext) JSONP(callback string, i interface{}) {
	err := c.jsonPBlob(callback, func(response http.ResponseWriter) error {
		var (
			data []byte
			err  error
		)
		if _, pretty := c.QueryParams()["pretty"]; pretty {
			data, err = json.MarshalIndent(i, "", "  ")
		} else {
			data, err = json.Marshal(i)
		}
		if err != nil {
			return err
		}
		_, err = response.Write(data)
		return err
	})
	util.Panic(err).When(err != nil)
}

// JSONPBlob sends a JSONP blob response.
func (c *BaseContext) JSONPBlob(callback string, b []byte) {
	err := c.jsonPBlob(callback, func(response http.ResponseWriter) error {
		_, err := response.Write(b)
		return err
	})
	util.Panic(err).When(err != nil)
}

func (c *BaseContext) xml(i interface{}, indent string) error {
	c.SetContentType(MIMEApplicationXMLCharsetUTF8)
	enc := xml.NewEncoder(c.w)
	if indent != "" {
		enc.Indent("", indent)
	}
	if _, err := c.w.Write([]byte(xml.Header)); err != nil {
		return err
	}
	return enc.Encode(i)
}

// XML sends an XML response.
func (c *BaseContext) XML(i interface{}) {
	indent := ""
	if _, ok := c.QueryParams()["pretty"]; ok {
		indent = "  "
	}
	err := c.xml(i, indent)
	util.Panic(err).When(err != nil)
}

// XMLPretty sends a pretty-print XML.
func (c *BaseContext) XMLPretty(i interface{}, indent string) {
	err := c.xml(i, indent)
	util.Panic(err).When(err != nil)
}

// XMLBlob sends an XML blob response.
func (c *BaseContext) XMLBlob(b []byte) {
	c.SetContentType(MIMEApplicationXMLCharsetUTF8)
	_, err := c.w.Write([]byte(xml.Header))
	util.Panic(err).When(err != nil)
	_, err = c.w.Write(b)
	util.Panic(err).When(err != nil)
}

// Blob sends a blob response with content type.
func (c *BaseContext) Blob(contentType string, b []byte) {
	c.SetContentType(contentType)
	_, err := c.w.Write(b)
	util.Panic(err).When(err != nil)
}

// File sends a response with the content of the file.
func (c *BaseContext) File(file string) {
	http.ServeFile(c.w, c.r, file)
}

func (c *BaseContext) contentDisposition(file, name, dispositionType string) {
	s := fmt.Sprintf("%s; filename=%q", dispositionType, name)
	c.SetHeader(HeaderContentDisposition, s)
	c.File(file)
}

// Attachment sends a response as attachment
func (c *BaseContext) Attachment(file string, name string) {
	c.contentDisposition(file, name, "attachment")
}

// Inline sends a response as inline
func (c *BaseContext) Inline(file string, name string) {
	c.contentDisposition(file, name, "inline")
}

// Redirect redirects the request to a provided URL with status code.
func (c *BaseContext) Redirect(code int, url string) {
	if (code < http.StatusMultipleChoices || code > http.StatusPermanentRedirect) && code != http.StatusCreated {
		panic(fmt.Sprintf("cann't redirect with status code %d", code))
	}
	http.Redirect(c.w, c.r, url, code)
}

// SSEvent writes a Server-Sent Event into the body stream.
func (c *BaseContext) SSEvent(name string, message interface{}) {
	panic(util.UnimplementedMethod)
}
