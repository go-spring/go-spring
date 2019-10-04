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
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"

	"github.com/didi/go-spring/spring-trace"
	"github.com/gin-gonic/gin"
	"github.com/didi/go-spring/spring-web"
)

type Context struct {
	*SpringTrace.DefaultTraceContext

	GinContext *gin.Context
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
	panic(SpringWeb.UNSUPPORTED_METHOD)
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
	return ctx.GinContext.Query(name)
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
	panic(SpringWeb.UNSUPPORTED_METHOD)
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
	return ctx.GinContext.Bind(i)
}

func (ctx *Context) Header(key, value string) {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) SetAccepted(formats ...string) {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) Render(code int, name string, data interface{}) error {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) HTML(code int, html string) error {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) HTMLBlob(code int, b []byte) error {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}

func (ctx *Context) String(code int, format string, values ...interface{}) {
	ctx.GinContext.String(code, fmt.Sprintf(format, values...))
}

func (ctx *Context) JSON(code int, i interface{}) error {
	panic(SpringWeb.UNSUPPORTED_METHOD)
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

func (ctx *Context) Error(err error) {
	panic(SpringWeb.UNSUPPORTED_METHOD)
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
