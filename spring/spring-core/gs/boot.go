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

package gs

import (
	"reflect"

	"github.com/go-spring/spring-core/arg"
	"github.com/go-spring/spring-core/web"
)

var app = NewApp()

func ExpectSysProperties(pattern ...string) {
	app.expectSysProperties = pattern
}

func BannerMode(mode int) {
	app.bannerMode = mode
}

// Banner 设置自定义 Banner 字符串
func Banner(banner string) {
	app.banner = banner
}

// Property 设置属性值，属性名称统一转成小写。
func Property(key string, value interface{}) {
	app.Property(key, value)
}

// Object 注册对象形式的 bean 。
func Object(i interface{}) *BeanDefinition {
	return app.c.register(NewBean(reflect.ValueOf(i)))
}

// Provide 注册构造函数形式的 bean 。
func Provide(ctor interface{}, args ...arg.Arg) *BeanDefinition {
	return app.c.register(NewBean(ctor, args...))
}

func Config(fn interface{}, args ...arg.Arg) *Configer {
	return app.c.config(NewConfiger(fn, args...))
}

// Route 返回和 Mapping 绑定的路由分组。
func Route(basePath string) *web.Router {
	return app.Route(basePath)
}

// HandleRequest 注册任意 HTTP 方法处理函数。
func HandleRequest(method uint32, path string, fn web.Handler) *web.Mapper {
	return app.HandleRequest(method, path, fn)
}

// RequestMapping 注册任意 HTTP 方法处理函数。
func RequestMapping(method uint32, path string, fn web.HandlerFunc) *web.Mapper {
	return app.RequestMapping(method, path, fn)
}

// RequestBinding 注册任意 HTTP 方法处理函数。
func RequestBinding(method uint32, path string, fn interface{}) *web.Mapper {
	return app.RequestBinding(method, path, fn)
}

// HandleGet 注册 GET 方法处理函数。
func HandleGet(path string, fn web.Handler) *web.Mapper {
	return app.HandleGet(path, fn)
}

// GetMapping 注册 GET 方法处理函数。
func GetMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return app.GetMapping(path, fn)
}

// GetBinding 注册 GET 方法处理函数。
func GetBinding(path string, fn interface{}) *web.Mapper {
	return app.GetBinding(path, fn)
}

// HandlePost 注册 POST 方法处理函数。
func HandlePost(path string, fn web.Handler) *web.Mapper {
	return app.HandlePost(path, fn)
}

// PostMapping 注册 POST 方法处理函数。
func PostMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return app.PostMapping(path, fn)
}

// PostBinding 注册 POST 方法处理函数。
func PostBinding(path string, fn interface{}) *web.Mapper {
	return app.PostBinding(path, fn)
}

// HandlePut 注册 PUT 方法处理函数。
func HandlePut(path string, fn web.Handler) *web.Mapper {
	return app.HandlePut(path, fn)
}

// PutMapping 注册 PUT 方法处理函数。
func PutMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return app.PutMapping(path, fn)
}

// PutBinding 注册 PUT 方法处理函数。
func PutBinding(path string, fn interface{}) *web.Mapper {
	return app.PutBinding(path, fn)
}

// HandleDelete 注册 DELETE 方法处理函数。
func HandleDelete(path string, fn web.Handler) *web.Mapper {
	return app.HandleDelete(path, fn)
}

// DeleteMapping 注册 DELETE 方法处理函数。
func DeleteMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return app.DeleteMapping(path, fn)
}

// DeleteBinding 注册 DELETE 方法处理函数。
func DeleteBinding(path string, fn interface{}) *web.Mapper {
	return app.DeleteBinding(path, fn)
}

// Filter 注册 web.Filter 对象。
func Filter(objOrCtor interface{}, ctorArgs ...arg.Arg) *BeanDefinition {
	b := NewBean(objOrCtor, ctorArgs...)
	return app.c.register(b).Export((*web.Filter)(nil))
}

// Consume 注册 MQ 消费者。
func Consume(topic string, fn interface{}) {
	app.Consume(topic, fn)
}

// GRPCClient 注册 gRPC 服务客户端，fn 是 gRPC 自动生成的客户端构造函数。
func GRPCClient(fn interface{}, endpoint string) *BeanDefinition {
	return app.c.register(NewBean(fn, endpoint))
}

// GRPCServer 注册 gRPC 服务提供者，fn 是 gRPC 自动生成的服务注册函数，
// serviceName 是服务名称，必须对应 *_grpc.pg.go 文件里面 grpc.ServerDesc
// 的 ServiceName 字段，server 是服务提供者对象。
func GRPCServer(serviceName string, fn interface{}, service interface{}) {
	app.GRPCServer(serviceName, fn, service)
}
