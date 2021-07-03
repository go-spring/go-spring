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
	"context"
	"reflect"

	"github.com/go-spring/spring-core/arg"
	"github.com/go-spring/spring-core/web"
)

var app = NewApp()

// Banner 自定义 banner 字符串。
func Banner(banner string) {
	app.Banner(banner)
}

// ShowBanner 设置是否显示 banner。
func ShowBanner(show bool) {
	app.ShowBanner(show)
}

func ExpectSystemEnv(pattern ...string) {
	app.ExpectSystemEnv(pattern...)
}

// Property 设置 key 对应的属性值，如果 key 对应的属性值已经存在则 Set 方法会
// 覆盖旧值。Set 方法除了支持 string 类型的属性值，还支持 int、uint、bool 等
// 其他基础数据类型的属性值。特殊情况下，Set 方法也支持 slice 、map 与基础数据
// 类型组合构成的属性值，其处理方式是将组合结构层层展开，可以将组合结构看成一棵树，
// 那么叶子结点的路径就是属性的 key，叶子结点的值就是属性的值。
func Property(key string, value interface{}) {
	app.Property(key, value)
}

func OnProperty(key string, fn interface{}) {
	app.OnProperty(key, fn)
}

// Object 注册对象形式的 bean ，需要注意的是该方法在注入开始后就不能再调用了。
func Object(i interface{}) *BeanDefinition {
	return app.c.register(NewBean(reflect.ValueOf(i)))
}

// Provide 注册构造函数形式的 bean ，需要注意的是该方法在注入开始后就不能再调用了。
func Provide(ctor interface{}, args ...arg.Arg) *BeanDefinition {
	return app.c.register(NewBean(ctor, args...))
}

// Go 创建安全可等待的 goroutine，fn 要求的 ctx 对象由 IoC 容器提供，当 IoC 容
// 器关闭时 ctx会 发出 Done 信号， fn 在接收到此信号后应当立即退出。
func Go(fn func(ctx context.Context)) {
	app.Go(fn)
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
func Consume(fn interface{}, topics ...string) {
	app.Consume(fn, topics...)
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
