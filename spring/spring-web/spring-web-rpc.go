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

package SpringWeb

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/go-spring/spring-error"
	"github.com/go-spring/spring-utils"
)

// contextType context.Context 的反射类型
var contextType = reflect.TypeOf((*context.Context)(nil)).Elem()

// bindHandler BIND 形式的 Web 处理接口
type bindHandler struct {
	fn       interface{}
	fnType   reflect.Type
	fnValue  reflect.Value
	bindType reflect.Type
}

func (b *bindHandler) Invoke(ctx WebContext) {
	RpcInvoke(ctx, b.call)
}

func (b *bindHandler) call(ctx WebContext) interface{} {

	// 反射创建需要绑定请求参数
	bindVal := reflect.New(b.bindType.Elem())
	err := ctx.Bind(bindVal.Interface())
	SpringError.ERROR.Panic(err).When(err != nil)

	// 执行处理函数，并返回结果
	in := []reflect.Value{reflect.ValueOf(ctx.Context()), bindVal}
	return b.fnValue.Call(in)[0].Interface()
}

func (b *bindHandler) FileLine() (file string, line int, fnName string) {
	return SpringUtils.FileLine(b.fn)
}

func validBindFn(fnType reflect.Type) bool {

	// 必须是函数，必须有两个入参，必须有一个返回值
	if fnType.Kind() != reflect.Func || fnType.NumIn() != 2 || fnType.NumOut() != 1 {
		return false
	}

	// 第一个入参必须是 context.Context 类型
	if fnType.In(0) != contextType {
		return false
	}

	req := fnType.In(1) // 第二个入参必须是结构体指针
	return req.Kind() == reflect.Ptr && req.Elem().Kind() == reflect.Struct
}

// BIND 转换成 BIND 形式的 Web 处理接口
func BIND(fn interface{}) Handler {
	if fnType := reflect.TypeOf(fn); validBindFn(fnType) {
		return &bindHandler{
			fn:       fn,
			fnType:   fnType,
			fnValue:  reflect.ValueOf(fn),
			bindType: fnType.In(1),
		}
	}
	panic(errors.New("fn should be func(context.Context, *struct})anything"))
}

// RpcInvoke 可自定义的 rpc 执行函数
var RpcInvoke = defaultRpcInvoke

// defaultRpcInvoke 默认的 rpc 执行函数
func defaultRpcInvoke(webCtx WebContext, fn func(WebContext) interface{}) {

	// 目前 HTTP RPC 只能返回 json 格式的数据
	webCtx.Header("Content-Type", "application/json")

	defer func() {
		if r := recover(); r != nil {
			result, ok := r.(*SpringError.RpcResult)
			if !ok {
				var err error
				if err, ok = r.(error); !ok {
					err = errors.New(fmt.Sprint(r))
				}
				result = SpringError.ERROR.Error(err)
			}
			webCtx.JSON(result)
		}
	}()

	result := SpringError.SUCCESS.Data(fn(webCtx))
	webCtx.JSON(result)
}
