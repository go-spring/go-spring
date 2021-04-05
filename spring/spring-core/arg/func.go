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

package arg

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"

	"github.com/go-spring/spring-core/util"
)

// errorType error 的反射类型。
var errorType = reflect.TypeOf((*error)(nil)).Elem()

// FileLine 返回文件行号的接口
type FileLine interface {
	FileLine() string
}

// Callable 有返回值的函数。
type Callable interface {
	Call(ctx Context, receiver ...reflect.Value) ([]reflect.Value, error)
}

// Caller 绑定函数及其参数。
func Caller(fn interface{}, withReceiver bool, args []Arg) Callable {
	return bind(fn, withReceiver, args)
}

// Runnable 无返回值的函数。
type Runnable interface {
	Run(ctx Context, receiver ...reflect.Value) error
}

// IsRunnerType 返回是否是 Runnable 绑定函数类型
func IsRunnerType(t reflect.Type) bool {
	return util.FuncType(t) && (util.ReturnNothing(t) || util.ReturnOnlyError(t))
}

// Runner 绑定函数及其参数。
func Runner(fn interface{}, withReceiver bool, args []Arg) Runnable {
	return bind(fn, withReceiver, args)
}

// functor 仿函数，绑定函数及其参数。
type functor struct {
	fn   interface{}
	arg  *argList
	file string // 注册点所在文件
	line int    // 注册点所在行数
}

// bind 绑定函数及其参数。
func bind(fn interface{}, withReceiver bool, args []Arg) *functor {

	var (
		file string
		line int
	)

	for i := 2; i < 10; i++ {
		_, f, l, _ := runtime.Caller(i)
		if strings.Contains(f, "/spring-core/") {
			if !strings.HasSuffix(f, "_test.go") {
				continue
			}
		}
		file = f
		line = l
		break
	}

	fnType := reflect.TypeOf(fn)
	argList := newArgList(fnType, withReceiver, args)
	return &functor{fn: fn, arg: argList, file: file, line: line}
}

func (r *functor) FileLine() string {
	return fmt.Sprintf("%s:%d", r.file, r.line)
}

func (r *functor) Call(ctx Context, receiver ...reflect.Value) ([]reflect.Value, error) {

	var in []reflect.Value

	if r.arg.WithReceiver() {
		in = append(in, receiver[0])
	}

	if r.arg != nil {
		v, err := r.arg.Get(ctx, r.FileLine())
		if err != nil {
			return nil, err
		}
		if len(v) > 0 {
			in = append(in, v...)
		}
	}

	// 调用 fn 函数
	out := reflect.ValueOf(r.fn).Call(in)

	if n := len(out); n > 0 {
		if o := out[n-1]; o.Type() == errorType {
			if i := o.Interface(); i == nil {
				return out[:n-1], nil
			} else {
				return out[:n-1], i.(error)
			}
		}
	}

	return out, nil
}

func (r *functor) Run(ctx Context, receiver ...reflect.Value) error {
	if out, err := r.Call(ctx, receiver...); err != nil {
		return err
	} else if len(out) == 0 {
		return nil
	}
	return errors.New("error func type")
}
