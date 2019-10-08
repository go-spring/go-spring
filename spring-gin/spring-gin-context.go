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

	"github.com/didi/go-spring/spring-trace"
	"github.com/didi/go-spring/spring-web"
	"github.com/gin-gonic/gin"
)

const (
	defaultMemory = 32 << 20 // 32 MB
)

const (
	HeaderContentType        = "Content-Type"
	HeaderContentDisposition = "Content-Disposition"
	HeaderXForwardedProto    = "X-Forwarded-Proto"
	HeaderXForwardedProtocol = "X-Forwarded-Protocol"
	HeaderXForwardedSsl      = "X-Forwarded-Ssl"
	HeaderXUrlScheme         = "X-Url-Scheme"
)

const (
	charsetUTF8 = "charset=UTF-8"
)

const (
	MIMEApplicationJSON                  = "application/json"
	MIMEApplicationJSONCharsetUTF8       = MIMEApplicationJSON + "; " + charsetUTF8
	MIMEApplicationXML                   = "application/xml"
	MIMEApplicationXMLCharsetUTF8        = MIMEApplicationXML + "; " + charsetUTF8
	MIMEApplicationJavaScript            = "application/javascript"
	MIMEApplicationJavaScriptCharsetUTF8 = MIMEApplicationJavaScript + "; " + charsetUTF8
	MIMEMultipartForm                    = "multipart/form-data"
	MIMETextHTML                         = "text/html"
	MIMETextHTMLCharsetUTF8              = MIMETextHTML + "; " + charsetUTF8
)

//
// 适配 gin 的 Web 上下文
//
type Context struct {
	*SpringTrace.DefaultTraceContext

	// gin 上下文对象
	GinContext *gin.Context

	// 处理器 Path
	HandlerPath string

	// Web 处理函数
	HandlerFunc SpringWeb.Handler

	paramNames  []string
	paramValues []string
}

func (ctx *Context) NativeContext() interface{} {
	return ctx.GinContext
}

func (ctx *Context) Get(key string) interface{} {
	val, _ := ctx.GinContext.Get(key)
	return val
}

func (ctx *Context) Set(key string, val interface{}) {
	ctx.GinContext.Set(key, val)
}

func (ctx *Context) Request() *http.Request {
	return ctx.GinContext.Request
}

func (ctx *Context) IsTLS() bool {
	return ctx.GinContext.Request.TLS != nil
}

func (ctx *Context) IsWebSocket() bool {
	return ctx.GinContext.IsWebsocket()
}

func (ctx *Context) Scheme() string {
	// NOTE: 这一段逻辑使用 echo 的实现

	r := ctx.Request()

	// Can't use `r.Request.URL.Scheme`
	// See: https://groups.google.com/forum/#!topic/golang-nuts/pMUkBlQBDF0

	if r.TLS != nil {
		return "https"
	}

	if scheme := r.Header.Get(HeaderXForwardedProto); scheme != "" {
		return scheme
	}

	if scheme := r.Header.Get(HeaderXForwardedProtocol); scheme != "" {
		return scheme
	}

	if ssl := r.Header.Get(HeaderXForwardedSsl); ssl == "on" {
		return "https"
	}

	if scheme := r.Header.Get(HeaderXUrlScheme); scheme != "" {
		return scheme
	}
	return "http"
}

func (ctx *Context) ClientIP() string {
	return ctx.GinContext.ClientIP()
}

func (ctx *Context) Path() string {
	return ctx.HandlerPath
}

func (ctx *Context) Handler() SpringWeb.Handler {
	return ctx.HandlerFunc
}

func (ctx *Context) ContentType() string {
	return ctx.GinContext.ContentType()
}

func (ctx *Context) GetHeader(key string) string {
	return ctx.GinContext.GetHeader(key)
}

func (ctx *Context) GetRawData() ([]byte, error) {
	return ctx.GinContext.GetRawData()
}

func (ctx *Context) PathParam(name string) string {
	return ctx.GinContext.Param(name)
}

func (ctx *Context) PathParamNames() []string {
	if ctx.paramNames == nil {
		ctx.paramNames = make([]string, 0)
		for _, entry := range ctx.GinContext.Params {
			ctx.paramNames = append(ctx.paramNames, entry.Key)
		}
	}
	return ctx.paramNames
}

func (ctx *Context) PathParamValues() []string {
	if ctx.paramValues == nil {
		ctx.paramValues = make([]string, 0)
		for _, entry := range ctx.GinContext.Params {
			ctx.paramValues = append(ctx.paramValues, entry.Value)
		}
	}
	return ctx.paramValues
}

func (ctx *Context) QueryParam(name string) string {
	return ctx.GinContext.Query(name)
}

func (ctx *Context) QueryParams() url.Values {
	return ctx.GinContext.Request.URL.Query()
}

func (ctx *Context) QueryString() string {
	return ctx.GinContext.Request.URL.RawQuery
}

func (ctx *Context) FormValue(name string) string {
	return ctx.GinContext.Request.FormValue(name)
}

func (ctx *Context) FormParams() (url.Values, error) {
	// NOTE: 这一段逻辑使用 echo 的实现

	if strings.HasPrefix(ctx.GetHeader(HeaderContentType), MIMEMultipartForm) {
		if err := ctx.GinContext.Request.ParseMultipartForm(defaultMemory); err != nil {
			return nil, err
		}
	} else {
		if err := ctx.GinContext.Request.ParseForm(); err != nil {
			return nil, err
		}
	}
	return ctx.GinContext.Request.Form, nil
}

func (ctx *Context) FormFile(name string) (*multipart.FileHeader, error) {
	return ctx.GinContext.FormFile(name)
}

func (ctx *Context) SaveUploadedFile(file *multipart.FileHeader, dst string) error {
	return ctx.GinContext.SaveUploadedFile(file, dst)
}

func (ctx *Context) MultipartForm() (*multipart.Form, error) {
	return ctx.GinContext.MultipartForm()
}

func (ctx *Context) Cookie(name string) (*http.Cookie, error) {
	return ctx.GinContext.Request.Cookie(name)
}

func (ctx *Context) Cookies() []*http.Cookie {
	return ctx.GinContext.Request.Cookies()
}

func (ctx *Context) Bind(i interface{}) error {
	return ctx.GinContext.Bind(i)
}

func (ctx *Context) Status(code int) {
	ctx.GinContext.Status(code)
}

func (ctx *Context) Header(key, value string) {
	ctx.GinContext.Header(key, value)
}

func (ctx *Context) SetCookie(cookie *http.Cookie) {
	http.SetCookie(ctx.GinContext.Writer, cookie)
}

func (ctx *Context) NoContent(code int) {
	ctx.Status(code)
}

func (ctx *Context) String(code int, format string, values ...interface{}) {
	ctx.GinContext.String(code, fmt.Sprintf(format, values...))
}

func (ctx *Context) HTML(code int, html string) {
	ctx.Blob(code, MIMETextHTMLCharsetUTF8, []byte(html))
}

func (ctx *Context) HTMLBlob(code int, b []byte) {
	ctx.Blob(code, MIMETextHTMLCharsetUTF8, b)
}

func (ctx *Context) JSON(code int, i interface{}) {
	ctx.GinContext.JSON(code, i)
}

func (ctx *Context) JSONPretty(code int, i interface{}, indent string) {

	b, err := json.MarshalIndent(i, "", indent)
	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.Blob(code, MIMEApplicationJSONCharsetUTF8, b)
}

func (ctx *Context) JSONBlob(code int, b []byte) {
	ctx.Blob(code, MIMEApplicationJSONCharsetUTF8, b)
}

func (ctx *Context) jsonPBlob(code int, callback string, data func(http.ResponseWriter) error) error {
	// NOTE: 这一段逻辑使用了 echo 的实现

	ctx.Header(HeaderContentType, MIMEApplicationJavaScriptCharsetUTF8)
	ctx.Status(code)

	response := ctx.GinContext.Writer

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

	if err != nil {
		ctx.Error(err)
	}
}

func (ctx *Context) JSONPBlob(code int, callback string, b []byte) {

	err := ctx.jsonPBlob(code, callback, func(response http.ResponseWriter) error {
		if _, err := response.Write(b); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		ctx.Error(err)
	}
}

func (ctx *Context) XML(code int, i interface{}) {
	ctx.GinContext.XML(code, i)
}

func (ctx *Context) xmlBlob(code int, data func(http.ResponseWriter) error) error {
	// NOTE: 这一段逻辑使用了 echo 的实现

	ctx.Header(HeaderContentType, MIMEApplicationJavaScriptCharsetUTF8)
	ctx.Status(code)

	response := ctx.GinContext.Writer

	if _, err := response.Write([]byte(xml.Header)); err != nil {
		return err
	}

	return data(response)
}

func (ctx *Context) XMLPretty(code int, i interface{}, indent string) {

	err := ctx.xmlBlob(code, func(response http.ResponseWriter) error {

		enc := xml.NewEncoder(response)
		if indent != "" {
			enc.Indent("", indent)
		}

		return enc.Encode(i)
	})

	if err != nil {
		ctx.Error(err)
	}
}

func (ctx *Context) XMLBlob(code int, b []byte) {

	err := ctx.xmlBlob(code, func(response http.ResponseWriter) error {
		_, err := response.Write(b)
		return err
	})

	if err != nil {
		ctx.Error(err)
	}
}

func (ctx *Context) Blob(code int, contentType string, b []byte) {
	// NOTE: 这一段逻辑使用了 echo 的实现

	ctx.Header(HeaderContentType, contentType)
	ctx.Status(code)

	response := ctx.GinContext.Writer

	if _, err := response.Write(b); err != nil {
		ctx.Error(err)
	}
}

func (ctx *Context) Stream(code int, contentType string, r io.Reader) {
	// NOTE: 这一段逻辑使用了 echo 的实现

	ctx.Header(HeaderContentType, contentType)
	ctx.Status(code)

	if _, err := io.Copy(ctx.GinContext.Writer, r); err != nil {
		ctx.Error(err)
	}
}

func (ctx *Context) contentDisposition(file, name, dispositionType string) {
	// NOTE: 这一段逻辑使用了 echo 的实现

	s := fmt.Sprintf("%s; filename=%q", dispositionType, name)
	ctx.Header(HeaderContentDisposition, s)
	ctx.File(file)
}

func (ctx *Context) File(file string) {
	ctx.GinContext.File(file)
}

func (ctx *Context) Attachment(file string, name string) {
	ctx.contentDisposition(file, name, "attachment")
}

func (ctx *Context) Inline(file string, name string) {
	ctx.contentDisposition(file, name, "inline")
}

func (ctx *Context) Redirect(code int, url string) {
	ctx.GinContext.Redirect(code, url)
}

func (ctx *Context) SSEvent(name string, message interface{}) {
	ctx.GinContext.SSEvent(name, message)
}

func (ctx *Context) Error(err error) {
	panic(err)
}
