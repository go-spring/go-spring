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

var gApp = NewApp()

func ExpectSysProperties(pattern ...string) {
	gApp.expectSysProperties = pattern
}

func BannerMode(mode int) {
	gApp.bannerMode = mode
}

// Banner 设置自定义 Banner 字符串
func Banner(banner string) {
	gApp.banner = banner
}

// Property 设置属性值，属性名称统一转成小写。
func Property(key string, value interface{}) {
	gApp.Property(key, value)
}

// Object 注册对象形式的 bean 。
func Object(i interface{}) *BeanDefinition {
	return gApp.Register(NewBean(reflect.ValueOf(i)))
}

// Provide 注册构造函数形式的 bean 。
func Provide(ctor interface{}, args ...arg.Arg) *BeanDefinition {
	return gApp.Register(NewBean(ctor, args...))
}

// Register 注册元数据形式的 bean 。
func Register(b *BeanDefinition) *BeanDefinition {
	return gApp.Register(b)
}

func Config(fn interface{}, args ...arg.Arg) *Configer {
	return gApp.Config(fn, args...)
}

// GRpcServer 注册 gRPC 服务提供者，fn 是 gRPC 自动生成的服务注册函数，serviceName 是服务名称，
// 必须对应 *_grpc.pg.go 文件里面 grpc.ServiceDesc 的 ServiceName 字段，server 是服务具体提供者对象。
func GRpcServer(serviceName string, fn interface{}, server interface{}) {
	gApp.GRpcServer(serviceName, fn, server)
}

// GRpcClient 注册 gRPC 服务客户端，fn 是 gRPC 自动生成的客户端构造函数
func GRpcClient(fn interface{}, endpoint string) *BeanDefinition {
	return gApp.Register(NewBean(fn, endpoint))
}

// Route 返回和 RootRouter 绑定的路由分组
func Route(basePath string) *web.Router {
	return gApp.Route(basePath)
}

// HandleRequest 注册任意 HTTP 方法处理函数
func HandleRequest(method uint32, path string, fn web.Handler) *web.Mapper {
	return gApp.HandleRequest(method, path, fn)
}

// RequestMapping 注册任意 HTTP 方法处理函数
func RequestMapping(method uint32, path string, fn web.HandlerFunc) *web.Mapper {
	return gApp.RequestMapping(method, path, fn)
}

// RequestBinding 注册任意 HTTP 方法处理函数
func RequestBinding(method uint32, path string, fn interface{}) *web.Mapper {
	return gApp.RequestBinding(method, path, fn)
}

// HandleGet 注册 GET 方法处理函数
func HandleGet(path string, fn web.Handler) *web.Mapper {
	return gApp.HandleGet(path, fn)
}

// GetMapping 注册 GET 方法处理函数
func GetMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return gApp.GetMapping(path, fn)
}

// GetBinding 注册 GET 方法处理函数
func GetBinding(path string, fn interface{}) *web.Mapper {
	return gApp.GetBinding(path, fn)
}

// HandlePost 注册 POST 方法处理函数
func HandlePost(path string, fn web.Handler) *web.Mapper {
	return gApp.HandlePost(path, fn)
}

// PostMapping 注册 POST 方法处理函数
func PostMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return gApp.PostMapping(path, fn)
}

// PostBinding 注册 POST 方法处理函数
func PostBinding(path string, fn interface{}) *web.Mapper {
	return gApp.PostBinding(path, fn)
}

// HandlePut 注册 PUT 方法处理函数
func HandlePut(path string, fn web.Handler) *web.Mapper {
	return gApp.HandlePut(path, fn)
}

// PutMapping 注册 PUT 方法处理函数
func PutMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return gApp.PutMapping(path, fn)
}

// PutBinding 注册 PUT 方法处理函数
func PutBinding(path string, fn interface{}) *web.Mapper {
	return gApp.PutBinding(path, fn)
}

// HandleDelete 注册 DELETE 方法处理函数
func HandleDelete(path string, fn web.Handler) *web.Mapper {
	return gApp.HandleDelete(path, fn)
}

// DeleteMapping 注册 DELETE 方法处理函数
func DeleteMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return gApp.DeleteMapping(path, fn)
}

// DeleteBinding 注册 DELETE 方法处理函数
func DeleteBinding(path string, fn interface{}) *web.Mapper {
	return gApp.DeleteBinding(path, fn)
}

// Filter 注册 web.Filter 对象
func Filter(objOrCtor interface{}, ctorArgs ...arg.Arg) *BeanDefinition {
	b := NewBean(objOrCtor, ctorArgs...)
	return gApp.Register(b).Export((*web.Filter)(nil))
}

// Consume 注册 BIND 形式的消息消费者
func Consume(topic string, fn interface{}) {
	gApp.Consume(topic, fn)
}
