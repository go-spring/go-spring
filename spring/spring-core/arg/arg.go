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

// Package arg 用于实现函数参数绑定。
package arg

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"

	"github.com/go-spring/spring-core/bean"
	"github.com/go-spring/spring-core/cond"
	"github.com/go-spring/spring-core/log"
	"github.com/go-spring/spring-core/util"
)

// Context IoC 容器对 arg 模块提供的最小功能集。
type Context interface {

	// Matches 条件成立返回 true，否则返回 false。
	Matches(c cond.Condition) (bool, error)

	// Bind 根据 tag 的内容进行属性绑定。
	Bind(v reflect.Value, tag string) error

	// Wire 根据 tag 的内容自动注入。
	Wire(v reflect.Value, tag string) error
}

// Arg 定义一个函数参数，可以是 bean.Selector 类型，表示注入一个 Bean；
// 可以是 ${X:=Y} 形式的字符串，表示绑定一个属性值；可以是 ValueArg 类型，
// 表示一个不从 IoC 容器获取而是用户传入的数据；可以是 IndexArg 类型，表
// 示一个带有下标的参数，IndexArg 是一个组合参数，接受普通类型的 Arg 参数；
// 可以是 *optionArg 类型，专门用于 Option 形式的函数参数，*optionArg
// 也是一个组合参数，可以接受非 *optionArg 类型的任意类型的参数。
type Arg interface{}

// IndexArg 包含下标的函数参数。
type IndexArg struct {
	n   int
	arg Arg
}

// Index 返回包含下标的函数参数，idx 从 1 开始计算。
func Index(n int, arg Arg) IndexArg {
	return IndexArg{n: n, arg: arg}
}

// R1 返回下标为 1 的函数参数。
func R1(arg Arg) IndexArg { return Index(1, arg) }

// R2 返回下标为 2 的函数参数。
func R2(arg Arg) IndexArg { return Index(2, arg) }

// R3 返回下标为 3 的函数参数。
func R3(arg Arg) IndexArg { return Index(3, arg) }

// R4 返回下标为 4 的函数参数。
func R4(arg Arg) IndexArg { return Index(4, arg) }

// R5 返回下标为 5 的函数参数。
func R5(arg Arg) IndexArg { return Index(5, arg) }

// R6 返回下标为 6 的函数参数。
func R6(arg Arg) IndexArg { return Index(6, arg) }

// R7 返回下标为 7 的函数参数。
func R7(arg Arg) IndexArg { return Index(7, arg) }

// ValueArg 包含具体值的函数参数。
type ValueArg struct{ arg interface{} }

// Value 返回包含具体值的函数参数。
func Value(arg interface{}) ValueArg {
	return ValueArg{arg: arg}
}

// argList 函数参数列表。
type argList struct {

	// fnType 函数的类型。
	fnType reflect.Type

	// args 函数参数列表。
	args []Arg
}

// newArgList 返回新创建的函数参数的列表。
func newArgList(fnType reflect.Type, args []Arg) *argList {

	// 计算函数不可变参数的数量，需要排除接收者。
	fixedArgCount := fnType.NumIn()
	if fnType.IsVariadic() {
		fixedArgCount--
	}

	// 函数的参数要么都有下标，要么都没有下标。
	shouldIndex := false

	// 分配不可变参数数量的空间，可变参数加在后面。
	fnArgs := make([]Arg, fixedArgCount)

	if len(args) > 0 {
		switch arg := args[0].(type) {
		case *optionArg:
			fnArgs = append(fnArgs, arg)
		case IndexArg:
			shouldIndex = true
			if n := arg.n - 1; n >= 0 && n < fixedArgCount {
				fnArgs[n] = arg.arg
			} else {
				panic(errors.New("参数索引超出函数入参的个数"))
			}
		default:
			shouldIndex = false
			if fixedArgCount > 0 {
				fnArgs[0] = arg
			} else if fnType.IsVariadic() {
				fnArgs = append(fnArgs, arg)
			} else {
				panic(errors.New("函数没有定义参数但却传入了"))
			}
		}
	}

	for i := 1; i < len(args); i++ {
		switch arg := args[i].(type) {
		case *optionArg:
			fnArgs = append(fnArgs, arg)
		case IndexArg:
			if !shouldIndex {
				panic(errors.New("所有非可选参数必须都有或者都没有索引"))
			}
			if n := arg.n - 1; n < 0 || n >= fixedArgCount {
				panic(errors.New("参数索引超出函数入参的个数"))
			} else if fnArgs[n] != nil {
				panic(fmt.Errorf("发现相同索引<%d>的参数", arg.n))
			} else {
				fnArgs[n] = arg.arg
			}
		default:
			if shouldIndex {
				panic(errors.New("所有非可选参数必须都有或者都没有索引"))
			}
			if i < fixedArgCount {
				fnArgs[i] = arg
			} else if fnType.IsVariadic() {
				fnArgs = append(fnArgs, arg)
			} else {
				panic(errors.New("传入的参数个数超出了函数入参的数量"))
			}
		}
	}

	// 其他没有传入的函数参数默认为空字符串。
	for i := 0; i < fixedArgCount; i++ {
		if fnArgs[i] == nil {
			fnArgs[i] = ""
		}
	}

	return &argList{fnType: fnType, args: fnArgs}
}

// get 返回函数所有参数的真实值，fileLine 是函数定义所在的文件及其行号，供打印日志时使用。
func (r *argList) get(ctx Context, fileLine string) ([]reflect.Value, error) {

	fnType := r.fnType
	numIn := fnType.NumIn()

	variadic := fnType.IsVariadic()
	result := make([]reflect.Value, 0)

	for idx, arg := range r.args {
		var t reflect.Type

		if variadic && idx >= numIn-1 {
			t = fnType.In(numIn - 1).Elem()
		} else {
			t = fnType.In(idx)
		}

		v, err := r.getArg(ctx, arg, t, fileLine)
		if err != nil {
			return nil, err
		}

		if v.IsValid() {
			result = append(result, v)
		}
	}

	return result, nil
}

func (r *argList) getArg(ctx Context, arg Arg, t reflect.Type, fileLine string) (v reflect.Value, err error) {

	description := fmt.Sprintf("arg:\"%v\" %s", arg, fileLine)
	log.Tracef("get value %s", description)
	defer func() {
		if err == nil {
			log.Tracef("get value success %s", description)
		} else {
			log.Tracef("get value err:%s %s", err.Error(), description)
		}
	}()

	tag := ""

	switch g := arg.(type) {
	case ValueArg:
		return reflect.ValueOf(g.arg), nil
	case *optionArg:
		return g.call(ctx)
	case bean.Definition:
		tag = g.ID()
	case string:
		tag = g
	default:
		tag = util.TypeName(g) + ":"
	}

	v = reflect.New(t).Elem()

	// 处理 bean 类型
	if util.IsBeanReceiver(t) {
		if err = ctx.Wire(v, tag); err != nil {
			return reflect.Value{}, err
		}
		return v, nil
	}

	// 处理 value 类型
	if tag == "" {
		tag = "${}"
	}
	if err = ctx.Bind(v, tag); err != nil {
		return reflect.Value{}, err
	}
	return v, nil
}

// optionArg Option 形式的函数参数
type optionArg struct {
	r *Callable
	c cond.Condition
}

// Option 封装 Option 形式的函数参数。
func Option(fn interface{}, args ...Arg) *optionArg {

	fnType := reflect.TypeOf(fn)

	// 判断是否是正确的 Option 函数定义，必要条件之一是只有一个返回值。
	if fnType.Kind() != reflect.Func || fnType.NumOut() != 1 {
		panic(errors.New("error option func"))
	}

	return &optionArg{r: Bind(fn, args, 1)}
}

// WithCond 为 Option 设置一个 cond.Condition
func (arg *optionArg) WithCond(c cond.Condition) *optionArg {
	arg.c = c
	return arg
}

func (arg *optionArg) call(ctx Context) (v reflect.Value, err error) {

	fileLine := fmt.Sprintf("%s:%d", arg.r.file, arg.r.line)
	log.Tracef("call option func %s", fileLine)

	defer func() {
		if err == nil {
			log.Tracef("call option func success %s", fileLine)
		} else {
			log.Tracef("call option func err:%s %s", err.Error(), fileLine)
		}
	}()

	if arg.c != nil {
		if ok, err := ctx.Matches(arg.c); err != nil {
			return reflect.Value{}, err
		} else if !ok {
			return reflect.Value{}, nil
		}
	}

	out, err := arg.r.Call(ctx)
	if err != nil {
		return reflect.Value{}, err
	}
	return out[0], nil
}

// Callable 绑定函数及其参数。
type Callable struct {
	fn      interface{}
	argList *argList
	file    string // 注册点所在文件
	line    int    // 注册点所在行数
}

// Bind 绑定函数及其参数。
func Bind(fn interface{}, args []Arg, skip int) *Callable {
	fnType := reflect.TypeOf(fn)
	argList := newArgList(fnType, args)
	_, file, line, _ := runtime.Caller(skip + 1)
	return &Callable{fn: fn, argList: argList, file: file, line: line}
}

func (r *Callable) Call(ctx Context) ([]reflect.Value, error) {

	fileLine := fmt.Sprintf("%s:%d", r.file, r.line)
	in, err := r.argList.get(ctx, fileLine)
	if err != nil {
		return nil, err
	}

	out := reflect.ValueOf(r.fn).Call(in)
	if n := len(out); n > 0 {
		if o := out[n-1]; util.IsErrorType(o.Type()) {
			if i := o.Interface(); i == nil {
				return out[:n-1], nil
			} else {
				return out[:n-1], i.(error)
			}
		}
	}
	return out, nil
}
