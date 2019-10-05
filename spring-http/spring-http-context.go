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

package SpringHttp

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/didi/go-spring/spring-web"
	"github.com/didi/go-spring/spring-trace"
)

const (
	defaultMemory = 32 << 20 // 32 MB
)

type Context struct {
	*SpringTrace.DefaultTraceContext

	R *http.Request
	W http.ResponseWriter
}

func NewContext(r *http.Request, w http.ResponseWriter) *Context {
	return &Context{
		R: r,
		W: w,
	}
}

func (ctx *Context) NativeContext() interface{} {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) Get(key string) interface{} {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) Set(key string, val interface{}) {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) Request() *http.Request {
	return ctx.R
}

func (ctx *Context) IsTLS() bool {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) IsWebSocket() bool {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) Scheme() string {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) ClientIP() string {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) Path() string {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) Handler() SpringWeb.Handler {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) ContentType() string {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) GetHeader(key string) string {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) GetRawData() ([]byte, error) {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) Param(name string) string {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) ParamNames() []string {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) ParamValues() []string {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) QueryParam(name string) string {
	return ctx.R.URL.Query().Get(name)
}

func (ctx *Context) QueryParams() url.Values {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) QueryString() string {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) FormValue(name string) string {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) FormParams() (url.Values, error) {
	if strings.HasPrefix(ctx.R.Header.Get(HeaderContentType), MIMEMultipartForm) {
		if err := ctx.R.ParseMultipartForm(defaultMemory); err != nil {
			return nil, err
		}
	} else {
		if err := ctx.R.ParseForm(); err != nil {
			return nil, err
		}
	}
	return ctx.R.Form, nil
}

func (ctx *Context) FormFile(name string) (*multipart.FileHeader, error) {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) MultipartForm() (*multipart.Form, error) {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) Cookie(name string) (*http.Cookie, error) {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) Cookies() []*http.Cookie {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) Bind(i interface{}) error {
	return (&DefaultBinder{}).Bind(i, ctx)
}

func (ctx *Context) Header(key, value string) {
	ctx.W.Header().Set(key, value)
}

func (ctx *Context) SetAccepted(formats ...string) {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) Render(code int, name string, data interface{}) error {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) writeContentType(value string) {
	header := ctx.W.Header()
	if header.Get(HeaderContentType) == "" {
		header.Set(HeaderContentType, value)
	}
}

func (ctx *Context) HTML(code int, html string) error {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) HTMLBlob(code int, b []byte) error {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) String(code int, format string, values ...interface{}) {
	ctx.W.WriteHeader(code)
	ctx.W.Write([]byte(fmt.Sprintf(format, values...)))
}

func (ctx *Context) JSON(code int, i interface{}) error {
	enc := json.NewEncoder(ctx.W)
	ctx.writeContentType(MIMEApplicationJSONCharsetUTF8)
	ctx.W.WriteHeader(code)
	return enc.Encode(i)
}

func (ctx *Context) JSONPretty(code int, i interface{}, indent string) error {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) JSONBlob(code int, b []byte) error {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) JSONP(code int, callback string, i interface{}) error {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) JSONPBlob(code int, callback string, b []byte) error {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) XML(code int, i interface{}) error {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) XMLPretty(code int, i interface{}, indent string) error {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) XMLBlob(code int, b []byte) error {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) Blob(code int, contentType string, b []byte) error {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) Stream(code int, contentType string, r io.Reader) error {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) File(file string) error {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) Attachment(file string, name string) error {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) Inline(file string, name string) error {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) NoContent(code int) error {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) Redirect(code int, url string) error {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func DefaultHTTPErrorHandler(err error, ctx *Context) {
	var (
		code = http.StatusInternalServerError
		msg  interface{}
	)

	if he, ok := err.(*HTTPError); ok {
		code = he.Code
		msg = he.Message
		if he.Internal != nil {
			err = fmt.Errorf("%v, %v", err, he.Internal)
		}
	} else {
		msg = http.StatusText(code)
	}
	if _, ok := msg.(string); ok {
		msg = map[string]interface{}{"message": msg}
	}

	if ctx.R.Method == http.MethodHead {
		err = ctx.NoContent(code)
	} else {
		err = ctx.JSON(code, msg)
	}

	if err != nil {
		ctx.LogError(err)
	}
}

func (ctx *Context) Error(err error) {
	DefaultHTTPErrorHandler(err, ctx)
}

func (ctx *Context) Data(code int, contentType string, data []byte) {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) SSEvent(name string, message interface{}) {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) SetCookie(cookie *http.Cookie) {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}
