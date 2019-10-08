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

package SpringEcho

import (
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"

	"github.com/didi/go-spring/spring-trace"
	"github.com/didi/go-spring/spring-web"
	"github.com/labstack/echo"
)

//
// 适配 echo 的 Web 上下文
//
type Context struct {
	*SpringTrace.DefaultTraceContext

	// echo 上下文对象
	EchoContext echo.Context

	// Web 处理函数
	HandlerFunc SpringWeb.Handler
}

func (ctx *Context) NativeContext() interface{} {
	return ctx.EchoContext
}

func (ctx *Context) Get(key string) interface{} {
	return ctx.EchoContext.Get(key)
}

func (ctx *Context) Set(key string, val interface{}) {
	ctx.EchoContext.Set(key, val)
}

func (ctx *Context) Request() *http.Request {
	return ctx.EchoContext.Request()
}

func (ctx *Context) IsTLS() bool {
	return ctx.EchoContext.IsTLS()
}

func (ctx *Context) IsWebSocket() bool {
	return ctx.EchoContext.IsWebSocket()
}

func (ctx *Context) Scheme() string {
	return ctx.EchoContext.Scheme()
}

func (ctx *Context) ClientIP() string {
	return ctx.EchoContext.RealIP()
}

func (ctx *Context) Path() string {
	return ctx.EchoContext.Path()
}

func (ctx *Context) Handler() SpringWeb.Handler {
	return ctx.HandlerFunc
}

func filterFlags(content string) string {
	for i, char := range content {
		if char == ' ' || char == ';' {
			return content[:i]
		}
	}
	return content
}

func (ctx *Context) ContentType() string {
	// NOTE: 这一段逻辑使用 gin 的实现

	s := ctx.GetHeader("Content-Type")
	return filterFlags(s)
}

func (ctx *Context) GetHeader(key string) string {
	return ctx.Request().Header.Get(key)
}

func (ctx *Context) GetRawData() ([]byte, error) {
	return ioutil.ReadAll(ctx.Request().Body)
}

func (ctx *Context) PathParam(name string) string {
	return ctx.EchoContext.Param(name)
}

func (ctx *Context) PathParamNames() []string {
	return ctx.EchoContext.ParamNames()
}

func (ctx *Context) PathParamValues() []string {
	return ctx.EchoContext.ParamValues()
}

func (ctx *Context) QueryParam(name string) string {
	return ctx.EchoContext.QueryParam(name)
}

func (ctx *Context) QueryParams() url.Values {
	return ctx.EchoContext.QueryParams()
}

func (ctx *Context) QueryString() string {
	return ctx.EchoContext.QueryString()
}

func (ctx *Context) FormValue(name string) string {
	return ctx.EchoContext.FormValue(name)
}

func (ctx *Context) FormParams() (url.Values, error) {
	return ctx.EchoContext.FormParams()
}

func (ctx *Context) FormFile(name string) (*multipart.FileHeader, error) {
	return ctx.EchoContext.FormFile(name)
}

func (ctx *Context) SaveUploadedFile(file *multipart.FileHeader, dst string) error {
	// NOTE: 这一段逻辑使用 gin 的实现

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

func (ctx *Context) MultipartForm() (*multipart.Form, error) {
	return ctx.EchoContext.MultipartForm()
}

func (ctx *Context) Cookie(name string) (*http.Cookie, error) {
	return ctx.EchoContext.Cookie(name)
}

func (ctx *Context) Cookies() []*http.Cookie {
	return ctx.EchoContext.Cookies()
}

func (ctx *Context) Bind(i interface{}) error {
	return ctx.EchoContext.Bind(i)
}

func (ctx *Context) Status(code int) {
	ctx.EchoContext.Response().WriteHeader(code)
}

func (ctx *Context) Header(key, value string) {
	ctx.EchoContext.Response().Header().Set(key, value)
}

func (ctx *Context) SetCookie(cookie *http.Cookie) {
	ctx.EchoContext.SetCookie(cookie)
}

func (ctx *Context) NoContent(code int) {
	err := ctx.EchoContext.NoContent(code)
	if err != nil {
		ctx.Error(err)
	}
}

func (ctx *Context) String(code int, format string, values ...interface{}) {
	err := ctx.EchoContext.String(code, fmt.Sprintf(format, values...))
	if err != nil {
		ctx.Error(err)
	}
}

func (ctx *Context) HTML(code int, html string) {
	err := ctx.EchoContext.HTML(code, html)
	if err != nil {
		ctx.Error(err)
	}
}

func (ctx *Context) HTMLBlob(code int, b []byte) {
	err := ctx.EchoContext.HTMLBlob(code, b)
	if err != nil {
		ctx.Error(err)
	}
}

func (ctx *Context) JSON(code int, i interface{}) {
	err := ctx.EchoContext.JSON(code, i)
	if err != nil {
		ctx.Error(err)
	}
}

func (ctx *Context) JSONPretty(code int, i interface{}, indent string) {
	err := ctx.EchoContext.JSONPretty(code, i, indent)
	if err != nil {
		ctx.Error(err)
	}
}

func (ctx *Context) JSONBlob(code int, b []byte) {
	err := ctx.EchoContext.JSONBlob(code, b)
	if err != nil {
		ctx.Error(err)
	}
}

func (ctx *Context) JSONP(code int, callback string, i interface{}) {
	err := ctx.EchoContext.JSONP(code, callback, i)
	if err != nil {
		ctx.Error(err)
	}
}

func (ctx *Context) JSONPBlob(code int, callback string, b []byte) {
	err := ctx.EchoContext.JSONPBlob(code, callback, b)
	if err != nil {
		ctx.Error(err)
	}
}

func (ctx *Context) XML(code int, i interface{}) {
	err := ctx.EchoContext.XML(code, i)
	if err != nil {
		ctx.Error(err)
	}
}

func (ctx *Context) XMLPretty(code int, i interface{}, indent string) {
	err := ctx.EchoContext.XMLPretty(code, i, indent)
	if err != nil {
		ctx.Error(err)
	}
}

func (ctx *Context) XMLBlob(code int, b []byte) {
	err := ctx.EchoContext.XMLBlob(code, b)
	if err != nil {
		ctx.Error(err)
	}
}

func (ctx *Context) Blob(code int, contentType string, b []byte) {
	err := ctx.EchoContext.Blob(code, contentType, b)
	if err != nil {
		ctx.Error(err)
	}
}

func (ctx *Context) Stream(code int, contentType string, r io.Reader) {
	err := ctx.EchoContext.Stream(code, contentType, r)
	if err != nil {
		ctx.Error(err)
	}
}

func (ctx *Context) File(file string) {
	err := ctx.EchoContext.File(file)
	if err != nil {
		ctx.Error(err)
	}
}

func (ctx *Context) Attachment(file string, name string) {
	err := ctx.EchoContext.Attachment(file, name)
	if err != nil {
		ctx.Error(err)
	}
}

func (ctx *Context) Inline(file string, name string) {
	err := ctx.EchoContext.Inline(file, name)
	if err != nil {
		ctx.Error(err)
	}
}

func (ctx *Context) Redirect(code int, url string) {
	err := ctx.EchoContext.Redirect(code, url)
	if err != nil {
		ctx.Error(err)
	}
}

func (ctx *Context) SSEvent(name string, message interface{}) {
	panic(SpringWeb.UNIMPLEMENTED_METHOD)
}

func (ctx *Context) Error(err error) {
	panic(err)
}
