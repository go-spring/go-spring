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
	"os"
	"reflect"
	"strings"

	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-core/grpc"
	"github.com/go-spring/spring-core/gs/arg"
	"github.com/go-spring/spring-core/web"
)

func init() {
	// 如果发现是调试模式则设置日志级别为 Debug 级别。
	{
		s := os.Getenv("CGO_CFLAGS")
		if strings.Contains(s, "-O0") && strings.Contains(s, "-g") {
			if !log.EnableDebug() {
				log.SetLevel(log.DebugLevel)
			}
		}
	}
}

var gApp *App

func app() *App {
	if gApp == nil {
		gApp = NewApp()
	}
	return gApp
}

// Setenv 封装 os.Setenv 函数，如果发生 error 会 panic 。
func Setenv(key string, value string) {
	err := os.Setenv(key, value)
	util.Panic(err).When(err != nil)
}

// Run 启动程序。
func Run() error {
	return gApp.Run()
}

// ShutDown 停止程序。
func ShutDown(err error) {
	gApp.ShutDown(err)
}

// Banner 参考 App.Banner 的解释。
func Banner(banner string) {
	gApp.Banner(banner)
}

// Bootstrap 参考 App.Bootstrap 的解释。
func Bootstrap() *bootstrap {
	return app().Bootstrap()
}

// OnProperty 参考 App.OnProperty 的解释。
func OnProperty(key string, fn interface{}) {
	app().OnProperty(key, fn)
}

// Property 参考 Container.Property 的解释。
func Property(key string, value interface{}) {
	app().Property(key, value)
}

// Object 参考 Container.Object 的解释。
func Object(i interface{}) *BeanDefinition {
	return app().c.register(NewBean(reflect.ValueOf(i)))
}

// Provide 参考 Container.Provide 的解释。
func Provide(ctor interface{}, args ...arg.Arg) *BeanDefinition {
	return app().c.register(NewBean(ctor, args...))
}

// HandleGet 参考 App.HandleGet 的解释。
func HandleGet(path string, h web.Handler) *web.Mapper {
	return app().HandleGet(path, h)
}

// GetMapping 参考 App.GetMapping 的解释。
func GetMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return app().GetMapping(path, fn)
}

// GetBinding 参考 App.GetBinding 的解释。
func GetBinding(path string, fn interface{}) *web.Mapper {
	return app().GetBinding(path, fn)
}

// HandlePost 参考 App.HandlePost 的解释。
func HandlePost(path string, h web.Handler) *web.Mapper {
	return app().HandlePost(path, h)
}

// PostMapping 参考 App.PostMapping 的解释。
func PostMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return app().PostMapping(path, fn)
}

// PostBinding 参考 App.PostBinding 的解释。
func PostBinding(path string, fn interface{}) *web.Mapper {
	return app().PostBinding(path, fn)
}

// HandlePut 参考 App.HandlePut 的解释。
func HandlePut(path string, h web.Handler) *web.Mapper {
	return app().HandlePut(path, h)
}

// PutMapping 参考 App.PutMapping 的解释。
func PutMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return app().PutMapping(path, fn)
}

// PutBinding 参考 App.PutBinding 的解释。
func PutBinding(path string, fn interface{}) *web.Mapper {
	return app().PutBinding(path, fn)
}

// HandleDelete 参考 App.HandleDelete 的解释。
func HandleDelete(path string, h web.Handler) *web.Mapper {
	return app().HandleDelete(path, h)
}

// DeleteMapping 参考 App.DeleteMapping 的解释。
func DeleteMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return app().DeleteMapping(path, fn)
}

// DeleteBinding 参考 App.DeleteBinding 的解释。
func DeleteBinding(path string, fn interface{}) *web.Mapper {
	return app().DeleteBinding(path, fn)
}

// HandleRequest 参考 App.HandleRequest 的解释。
func HandleRequest(method uint32, path string, h web.Handler) *web.Mapper {
	return app().HandleRequest(method, path, h)
}

// RequestMapping 参考 App.RequestMapping 的解释。
func RequestMapping(method uint32, path string, fn web.HandlerFunc) *web.Mapper {
	return app().RequestMapping(method, path, fn)
}

// RequestBinding 参考 App.RequestBinding 的解释。
func RequestBinding(method uint32, path string, fn interface{}) *web.Mapper {
	return app().RequestBinding(method, path, fn)
}

// Consume 参考 App.Consume 的解释。
func Consume(fn interface{}, topics ...string) {
	app().Consume(fn, topics...)
}

// GrpcServer 参考 App.GrpcServer 的解释。
func GrpcServer(serviceName string, server *grpc.Server) {
	app().GrpcServer(serviceName, server)
}

// GrpcClient 参考 App.GrpcClient 的解释。
func GrpcClient(fn interface{}, endpoint string) *BeanDefinition {
	return app().c.register(NewBean(fn, endpoint))
}
