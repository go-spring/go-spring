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
	"net/http"

	"github.com/go-spring/spring-base/util"
)

const (
	MethodGet     = 0x0001 // "GET"
	MethodHead    = 0x0002 // "HEAD"
	MethodPost    = 0x0004 // "POST"
	MethodPut     = 0x0008 // "PUT"
	MethodPatch   = 0x0010 // "PATCH"
	MethodDelete  = 0x0020 // "DELETE"
	MethodConnect = 0x0040 // "CONNECT"
	MethodOptions = 0x0080 // "OPTIONS"
	MethodTrace   = 0x0100 // "TRACE"
	MethodAny     = 0xffff
	MethodGetPost = MethodGet | MethodPost
)

var httpMethods = map[uint32]string{
	MethodGet:     http.MethodGet,
	MethodHead:    http.MethodHead,
	MethodPost:    http.MethodPost,
	MethodPut:     http.MethodPut,
	MethodPatch:   http.MethodPatch,
	MethodDelete:  http.MethodDelete,
	MethodConnect: http.MethodConnect,
	MethodOptions: http.MethodOptions,
	MethodTrace:   http.MethodTrace,
}

// GetMethod 返回 method 对应的 HTTP 方法
func GetMethod(method uint32) (r []string) {
	for k, v := range httpMethods {
		if method&k == k {
			r = append(r, v)
		}
	}
	return
}

// Mapper 路由映射器
type Mapper struct {
	method  uint32    // 请求方法
	path    string    // 路由地址
	handler Handler   // 处理函数
	swagger Operation // 描述文档
}

// NewMapper Mapper 的构造函数
func NewMapper(method uint32, path string, h Handler) *Mapper {
	return &Mapper{method: method, path: path, handler: h}
}

// Method 返回 Mapper 的方法
func (m *Mapper) Method() uint32 {
	return m.method
}

// Path 返回 Mapper 的路径
func (m *Mapper) Path() string {
	return m.path
}

// Handler 返回 Mapper 的处理函数
func (m *Mapper) Handler() Handler {
	return m.handler
}

// Operation 设置与 Mapper 绑定的 Operation 对象
func (m *Mapper) Operation(op Operation) {
	m.swagger = op
}

// Router 路由注册接口
type Router interface {

	// Mappers 返回映射器列表
	Mappers() []*Mapper

	// AddMapper 添加一个 Mapper
	AddMapper(m *Mapper)

	// HandleGet 注册 GET 方法处理函数
	HandleGet(path string, h Handler) *Mapper

	// GetMapping 注册 GET 方法处理函数
	GetMapping(path string, fn HandlerFunc) *Mapper

	// GetBinding 注册 GET 方法处理函数
	GetBinding(path string, fn interface{}) *Mapper

	// HandlePost 注册 POST 方法处理函数
	HandlePost(path string, h Handler) *Mapper

	// PostMapping 注册 POST 方法处理函数
	PostMapping(path string, fn HandlerFunc) *Mapper

	// PostBinding 注册 POST 方法处理函数
	PostBinding(path string, fn interface{}) *Mapper

	// HandlePut 注册 PUT 方法处理函数
	HandlePut(path string, h Handler) *Mapper

	// PutMapping 注册 PUT 方法处理函数
	PutMapping(path string, fn HandlerFunc) *Mapper

	// PutBinding 注册 PUT 方法处理函数
	PutBinding(path string, fn interface{}) *Mapper

	// HandleDelete 注册 DELETE 方法处理函数
	HandleDelete(path string, h Handler) *Mapper

	// DeleteMapping 注册 DELETE 方法处理函数
	DeleteMapping(path string, fn HandlerFunc) *Mapper

	// DeleteBinding 注册 DELETE 方法处理函数
	DeleteBinding(path string, fn interface{}) *Mapper

	// HandleRequest 注册任意 HTTP 方法处理函数
	HandleRequest(method uint32, path string, h Handler) *Mapper

	// RequestMapping 注册任意 HTTP 方法处理函数
	RequestMapping(method uint32, path string, fn HandlerFunc) *Mapper

	// RequestBinding 注册任意 HTTP 方法处理函数
	RequestBinding(method uint32, path string, fn interface{}) *Mapper

	// File 定义单个文件资源
	File(path string, file string) *Mapper

	// Static 定义一组文件资源
	Static(prefix string, dir string) *Mapper

	// StaticFS 定义一组文件资源
	StaticFS(prefix string, fs http.FileSystem) *Mapper
}

// router 路由注册接口的默认实现
type router struct {
	mappers []*Mapper
}

// NewRouter router 的构造函数。
func NewRouter() *router {
	return &router{}
}

// Mappers 返回映射器列表
func (r *router) Mappers() []*Mapper {
	return r.mappers
}

// AddMapper 添加一个 Mapper
func (r *router) AddMapper(m *Mapper) {
	r.mappers = append(r.mappers, m)
}

func (r *router) request(method uint32, path string, h Handler) *Mapper {
	m := NewMapper(method, path, h)
	r.AddMapper(m)
	return m
}

// HandleGet 注册 GET 方法处理函数
func (r *router) HandleGet(path string, h Handler) *Mapper {
	return r.request(MethodGet, path, h)
}

// GetMapping 注册 GET 方法处理函数
func (r *router) GetMapping(path string, fn HandlerFunc) *Mapper {
	return r.request(MethodGet, path, FUNC(fn))
}

// GetBinding 注册 GET 方法处理函数
func (r *router) GetBinding(path string, fn interface{}) *Mapper {
	return r.request(MethodGet, path, BIND(fn))
}

// HandlePost 注册 POST 方法处理函数
func (r *router) HandlePost(path string, h Handler) *Mapper {
	return r.request(MethodPost, path, h)
}

// PostMapping 注册 POST 方法处理函数
func (r *router) PostMapping(path string, fn HandlerFunc) *Mapper {
	return r.request(MethodPost, path, FUNC(fn))
}

// PostBinding 注册 POST 方法处理函数
func (r *router) PostBinding(path string, fn interface{}) *Mapper {
	return r.request(MethodPost, path, BIND(fn))
}

// HandlePut 注册 PUT 方法处理函数
func (r *router) HandlePut(path string, h Handler) *Mapper {
	return r.request(MethodPut, path, h)
}

// PutMapping 注册 PUT 方法处理函数
func (r *router) PutMapping(path string, fn HandlerFunc) *Mapper {
	return r.request(MethodPut, path, FUNC(fn))
}

// PutBinding 注册 PUT 方法处理函数
func (r *router) PutBinding(path string, fn interface{}) *Mapper {
	return r.request(MethodPut, path, BIND(fn))
}

// HandleDelete 注册 DELETE 方法处理函数
func (r *router) HandleDelete(path string, h Handler) *Mapper {
	return r.request(MethodDelete, path, h)
}

// DeleteMapping 注册 DELETE 方法处理函数
func (r *router) DeleteMapping(path string, fn HandlerFunc) *Mapper {
	return r.request(MethodDelete, path, FUNC(fn))
}

// DeleteBinding 注册 DELETE 方法处理函数
func (r *router) DeleteBinding(path string, fn interface{}) *Mapper {
	return r.request(MethodDelete, path, BIND(fn))
}

// HandleRequest 注册任意 HTTP 方法处理函数
func (r *router) HandleRequest(method uint32, path string, h Handler) *Mapper {
	return r.request(method, path, h)
}

// RequestMapping 注册任意 HTTP 方法处理函数
func (r *router) RequestMapping(method uint32, path string, fn HandlerFunc) *Mapper {
	return r.request(method, path, FUNC(fn))
}

// RequestBinding 注册任意 HTTP 方法处理函数
func (r *router) RequestBinding(method uint32, path string, fn interface{}) *Mapper {
	return r.request(method, path, BIND(fn))
}

// File 定义单个文件资源
func (r *router) File(path string, file string) *Mapper {
	return r.GetMapping(path, func(ctx Context) {
		ctx.File(file)
	})
}

// Static 定义一组文件资源
func (r *router) Static(prefix string, dir string) *Mapper {
	return r.StaticFS(prefix, http.Dir(dir))
}

// StaticFS 定义一组文件资源
func (r *router) StaticFS(prefix string, fs http.FileSystem) *Mapper {
	return r.HandleGet(prefix+"/*", &FileHandler{
		Prefix: prefix,
		Server: http.FileServer(fs),
	})
}

type FileHandler struct {
	Prefix string
	Server http.Handler
}

func (f *FileHandler) Invoke(ctx Context) {
	h := http.StripPrefix(f.Prefix, f.Server)
	h.ServeHTTP(ctx.ResponseWriter(), ctx.Request())
}

func (f *FileHandler) FileLine() (file string, line int, fnName string) {
	return util.FileLine(f.Invoke)
}
