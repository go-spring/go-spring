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

package boot

import (
	"github.com/go-spring/spring-core/arg"
	"github.com/go-spring/spring-core/core"
	"github.com/go-spring/spring-core/web"
)

var RootRouter = web.NewRootRouter()

// Route 返回和 RootRouter 绑定的路由分组
func Route(basePath string) *web.Router {
	return RootRouter.Route(basePath)
}

// HandleRequest 注册任意 HTTP 方法处理函数
func HandleRequest(method uint32, path string, fn web.Handler) *web.Mapper {
	return RootRouter.HandleRequest(method, path, fn)
}

// RequestMapping 注册任意 HTTP 方法处理函数
func RequestMapping(method uint32, path string, fn web.HandlerFunc) *web.Mapper {
	return RootRouter.HandleRequest(method, path, web.FUNC(fn))
}

// RequestBinding 注册任意 HTTP 方法处理函数
func RequestBinding(method uint32, path string, fn interface{}) *web.Mapper {
	return RootRouter.HandleRequest(method, path, web.BIND(fn))
}

// HandleGet 注册 GET 方法处理函数
func HandleGet(path string, fn web.Handler) *web.Mapper {
	return RootRouter.HandleGet(path, fn)
}

// GetMapping 注册 GET 方法处理函数
func GetMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return RootRouter.HandleGet(path, web.FUNC(fn))
}

// GetBinding 注册 GET 方法处理函数
func GetBinding(path string, fn interface{}) *web.Mapper {
	return RootRouter.HandleGet(path, web.BIND(fn))
}

// HandlePost 注册 POST 方法处理函数
func HandlePost(path string, fn web.Handler) *web.Mapper {
	return RootRouter.HandlePost(path, fn)
}

// PostMapping 注册 POST 方法处理函数
func PostMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return RootRouter.HandlePost(path, web.FUNC(fn))
}

// PostBinding 注册 POST 方法处理函数
func PostBinding(path string, fn interface{}) *web.Mapper {
	return RootRouter.HandlePost(path, web.BIND(fn))
}

// HandlePut 注册 PUT 方法处理函数
func HandlePut(path string, fn web.Handler) *web.Mapper {
	return RootRouter.HandlePut(path, fn)
}

// PutMapping 注册 PUT 方法处理函数
func PutMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return RootRouter.HandlePut(path, web.FUNC(fn))
}

// PutBinding 注册 PUT 方法处理函数
func PutBinding(path string, fn interface{}) *web.Mapper {
	return RootRouter.HandlePut(path, web.BIND(fn))
}

// HandleDelete 注册 DELETE 方法处理函数
func HandleDelete(path string, fn web.Handler) *web.Mapper {
	return RootRouter.HandleDelete(path, fn)
}

// DeleteMapping 注册 DELETE 方法处理函数
func DeleteMapping(path string, fn web.HandlerFunc) *web.Mapper {
	return RootRouter.HandleDelete(path, web.FUNC(fn))
}

// DeleteBinding 注册 DELETE 方法处理函数
func DeleteBinding(path string, fn interface{}) *web.Mapper {
	return RootRouter.HandleDelete(path, web.BIND(fn))
}

// Filter 注册 web.Filter 对象
func Filter(objOrCtor interface{}, ctorArgs ...arg.Arg) *core.BeanDefinition {
	return gApp.appCtx.Bean(objOrCtor, ctorArgs...).Export((*web.Filter)(nil))
}
