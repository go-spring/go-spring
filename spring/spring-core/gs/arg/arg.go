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

//go:generate mockgen -build_flags="-mod=mod" -package=arg -source=arg.go -destination=arg_mock.go

// Package arg 用于实现函数参数绑定。
package arg

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"

	"github.com/go-spring/spring-base/code"
	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/gs/cond"
	"github.com/go-spring/spring-core/gs/gsutil"
)

var (
	logger = log.GetLogger()
)

// Context defines some methods of IoC container that Callable use.
type Context interface {
	// Matches returns true when the Condition returns true,
	// and returns false when the Condition returns false.
	Matches(c cond.Condition) (bool, error)
	// Bind binds properties value by the "value" tag.
	Bind(v reflect.Value, tag string) error
	// Wire wires dependent beans by the "autowire" tag.
	Wire(v reflect.Value, tag string) error
}

// Arg 用于为函数参数提供绑定值。可以是 bean.Selector 类型，表示注入 bean ；
// 可以是 ${X:=Y} 形式的字符串，表示属性绑定或者注入 bean ；可以是 ValueArg
// 类型，表示不从 IoC 容器获取而是用户传入的普通值；可以是 IndexArg 类型，表示
// 带有下标的参数绑定；可以是 *optionArg 类型，用于为 Option 方法提供参数绑定。
type Arg interface{}

// IndexArg is an Arg that has an index.
type IndexArg struct {
	n   int
	arg Arg
}

// Index returns an IndexArg.
func Index(n int, arg Arg) IndexArg {
	return IndexArg{n: n, arg: arg}
}

// R0 returns an IndexArg with index 0.
func R0(arg Arg) IndexArg { return Index(0, arg) }

// R1 returns an IndexArg with index 1.
func R1(arg Arg) IndexArg { return Index(1, arg) }

// R2 returns an IndexArg with index 2.
func R2(arg Arg) IndexArg { return Index(2, arg) }

// R3 returns an IndexArg with index 3.
func R3(arg Arg) IndexArg { return Index(3, arg) }

// R4 returns an IndexArg with index 4.
func R4(arg Arg) IndexArg { return Index(4, arg) }

// R5 returns an IndexArg with index 5.
func R5(arg Arg) IndexArg { return Index(5, arg) }

// R6 returns an IndexArg with index 6.
func R6(arg Arg) IndexArg { return Index(6, arg) }

// ValueArg is an Arg that has a value.
type ValueArg struct {
	v interface{}
}

// Nil return a ValueArg with a value of nil.
func Nil() ValueArg {
	return ValueArg{v: nil}
}

// Value return a ValueArg with a value of v.
func Value(v interface{}) ValueArg {
	return ValueArg{v: v}
}

// argList stores the arguments of a function.
type argList struct {
	fnType reflect.Type
	args   []Arg
}

// newArgList returns a new argList.
func newArgList(fnType reflect.Type, args []Arg) (*argList, error) {

	fixedArgCount := fnType.NumIn()
	if fnType.IsVariadic() {
		fixedArgCount--
	}

	shouldIndex := func() bool {
		if len(args) == 0 {
			return false
		}
		_, ok := args[0].(IndexArg)
		return ok
	}()

	fnArgs := make([]Arg, fixedArgCount)

	if len(args) > 0 {
		switch arg := args[0].(type) {
		case *optionArg:
			fnArgs = append(fnArgs, arg)
		case IndexArg:
			if arg.n < 0 || arg.n >= fixedArgCount {
				return nil, util.Errorf(code.FileLine(), "arg index %d exceeds max index %d", arg.n, fixedArgCount)
			} else {
				fnArgs[arg.n] = arg.arg
			}
		default:
			if fixedArgCount > 0 {
				fnArgs[0] = arg
			} else if fnType.IsVariadic() {
				fnArgs = append(fnArgs, arg)
			} else {
				return nil, util.Errorf(code.FileLine(), "function has no args but given %d", len(args))
			}
		}
	}

	for i := 1; i < len(args); i++ {
		switch arg := args[i].(type) {
		case *optionArg:
			fnArgs = append(fnArgs, arg)
		case IndexArg:
			if !shouldIndex {
				return nil, util.Errorf(code.FileLine(), "the Args must have or have no index")
			}
			if arg.n < 0 || arg.n >= fixedArgCount {
				return nil, util.Errorf(code.FileLine(), "arg index %d exceeds max index %d", arg.n, fixedArgCount)
			} else if fnArgs[arg.n] != nil {
				return nil, util.Errorf(code.FileLine(), "found same index %d", arg.n)
			} else {
				fnArgs[arg.n] = arg.arg
			}
		default:
			if shouldIndex {
				return nil, util.Errorf(code.FileLine(), "the Args must have or have no index")
			}
			if i < fixedArgCount {
				fnArgs[i] = arg
			} else if fnType.IsVariadic() {
				fnArgs = append(fnArgs, arg)
			} else {
				return nil, util.Errorf(code.FileLine(), "the count %d of Args exceeds max index %d", len(args), fixedArgCount)
			}
		}
	}

	for i := 0; i < fixedArgCount; i++ {
		if fnArgs[i] == nil {
			fnArgs[i] = ""
		}
	}

	return &argList{fnType: fnType, args: fnArgs}, nil
}

// get returns all processed Args value. fileLine is the binding position of Callable.
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

		// option arg may not return a value when the condition is not met.
		v, err := r.getArg(ctx, arg, t, fileLine)
		if err != nil {
			return nil, util.Wrapf(err, code.FileLine(), "returns error when getting %d arg", idx)
		}
		if v.IsValid() {
			result = append(result, v)
		}
	}

	return result, nil
}

func (r *argList) getArg(ctx Context, arg Arg, t reflect.Type, fileLine string) (reflect.Value, error) {

	var (
		err error
		tag string
	)

	description := fmt.Sprintf("arg:\"%v\" %s", arg, fileLine)
	logger.Tracef("get value %s", description)
	defer func() {
		if err == nil {
			logger.Tracef("get value success %s", description)
		} else {
			logger.Tracef("get value error %s %s", err.Error(), description)
		}
	}()

	switch g := arg.(type) {
	case *Callable:
		if results, err := g.Call(ctx); err != nil {
			return reflect.Value{}, util.Wrapf(err, code.FileLine(), "")
		} else if len(results) < 1 {
			return reflect.Value{}, errors.New("")
		} else {
			return results[0], nil
		}
	case ValueArg:
		if g.v == nil {
			return reflect.Zero(t), nil
		}
		return reflect.ValueOf(g.v), nil
	case *optionArg:
		return g.call(ctx)
	case gsutil.BeanDefinition:
		tag = g.ID()
	case string:
		tag = g
	default:
		tag = gsutil.TypeName(g) + ":"
	}

	// binds properties value by the "value" tag.
	if conf.IsValueType(t) {
		if tag == "" {
			tag = "${}"
		}
		v := reflect.New(t).Elem()
		if err = ctx.Bind(v, tag); err != nil {
			return reflect.Value{}, err
		}
		return v, nil
	}

	// wires dependent beans by the "autowire" tag.
	if gsutil.IsBeanReceiver(t) {
		v := reflect.New(t).Elem()
		if err = ctx.Wire(v, tag); err != nil {
			return reflect.Value{}, err
		}
		return v, nil
	}

	return reflect.Value{}, util.Errorf(code.FileLine(), "error type %s", t.String())
}

// optionArg Option 函数的参数绑定。
type optionArg struct {
	r *Callable
	c cond.Condition
}

// Provide 为 Option 方法绑定运行时参数。
func Provide(fn interface{}, args ...Arg) *Callable {
	r, err := Bind(fn, args, 1)
	util.Panic(err).When(err != nil)
	return r
}

// Option 返回 Option 函数的参数绑定。
func Option(fn interface{}, args ...Arg) *optionArg {

	t := reflect.TypeOf(fn)
	if t.Kind() != reflect.Func || t.NumOut() != 1 {
		panic(errors.New("invalid option func"))
	}

	r, err := Bind(fn, args, 1)
	util.Panic(err).When(err != nil)
	return &optionArg{r: r}
}

// On 设置一个 cond.Condition 对象。
func (arg *optionArg) On(c cond.Condition) *optionArg {
	arg.c = c
	return arg
}

func (arg *optionArg) call(ctx Context) (reflect.Value, error) {

	var (
		ok  bool
		err error
	)

	logger.Tracef("call option func %s", arg.r.fileLine)
	defer func() {
		if err == nil {
			logger.Tracef("call option func success %s", arg.r.fileLine)
		} else {
			logger.Tracef("call option func error %s %s", err.Error(), arg.r.fileLine)
		}
	}()

	if arg.c != nil {
		ok, err = ctx.Matches(arg.c)
		if err != nil {
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

// Callable wrappers a function and its binding arguments, then you can invoke
// the Call method of Callable to get the function's result.
type Callable struct {
	fn       interface{}
	argList  *argList
	fileLine string
}

// Bind returns a Callable that wrappers a function and its binding arguments.
// The argument skip is the number of frames to skip over.
func Bind(fn interface{}, args []Arg, skip int) (*Callable, error) {

	fnType := reflect.TypeOf(fn)
	argList, err := newArgList(fnType, args)
	if err != nil {
		return nil, err
	}

	_, file, line, _ := runtime.Caller(skip + 1)
	r := &Callable{
		fn:       fn,
		argList:  argList,
		fileLine: fmt.Sprintf("%s:%d", file, line),
	}
	return r, nil
}

// Arg returns the ith binding argument.
func (r *Callable) Arg(i int) (Arg, bool) {
	if i >= len(r.argList.args) {
		return nil, false
	}
	return r.argList.args[i], true
}

// Call invokes the function with its binding arguments processed in the IoC
// container. If the function returns an error, then the Call returns it.
func (r *Callable) Call(ctx Context) ([]reflect.Value, error) {

	in, err := r.argList.get(ctx, r.fileLine)
	if err != nil {
		return nil, err
	}

	out := reflect.ValueOf(r.fn).Call(in)
	n := len(out)
	if n == 0 {
		return out, nil
	}

	o := out[n-1]
	if util.IsErrorType(o.Type()) {
		if i := o.Interface(); i != nil {
			return out[:n-1], i.(error)
		}
		return out[:n-1], nil
	}
	return out, nil
}
