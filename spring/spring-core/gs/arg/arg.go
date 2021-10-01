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

	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-core/gs/cond"
	"github.com/go-spring/spring-core/gs/internal"
)

// Context IoC 容器对 arg 模块提供的最小功能集。
type Context interface {

	// Matches 条件成立返回 true，否则返回 false。
	Matches(c cond.Condition) (bool, error)

	// Bind 根据 tag 的内容对 v 进行属性绑定。
	Bind(v reflect.Value, tag string) error

	// Wire 根据 tag 的内容对 v 进行依赖注入。
	Wire(v reflect.Value, tag string) error
}

// Arg 用于为函数参数提供绑定值。可以是 bean.Selector 类型，表示注入 bean ；
// 可以是 ${X:=Y} 形式的字符串，表示属性绑定或者注入 bean ；可以是 ValueArg
// 类型，表示不从 IoC 容器获取而是用户传入的普通值；可以是 IndexArg 类型，表示
// 带有下标的参数绑定；可以是 *optionArg 类型，用于为 Option 方法提供参数绑定。
type Arg interface{}

// IndexArg 包含下标的参数绑定。
type IndexArg struct {
	n   int
	arg Arg
}

// Index 返回包含下标的参数绑定，下标从 1 开始。
func Index(n int, arg Arg) IndexArg {
	return IndexArg{n: n, arg: arg}
}

// R0 返回下标为 0 的参数绑定。
func R0(arg Arg) IndexArg { return Index(1, arg) }

// R1 返回下标为 1 的参数绑定。
func R1(arg Arg) IndexArg { return Index(2, arg) }

// R2 返回下标为 2 的参数绑定。
func R2(arg Arg) IndexArg { return Index(3, arg) }

// R3 返回下标为 3 的参数绑定。
func R3(arg Arg) IndexArg { return Index(4, arg) }

// R4 返回下标为 4 的参数绑定。
func R4(arg Arg) IndexArg { return Index(5, arg) }

// R5 返回下标为 5 的参数绑定。
func R5(arg Arg) IndexArg { return Index(6, arg) }

// R6 返回下标为 6 的参数绑定。
func R6(arg Arg) IndexArg { return Index(7, arg) }

// ValueArg 包含具体值的参数绑定。
type ValueArg struct {
	v interface{}
}

// Value 返回包含具体值的参数绑定。
func Value(v interface{}) ValueArg {
	return ValueArg{v: v}
}

// argList 函数参数绑定列表。
type argList struct {

	// args 参数绑定列表。
	args []Arg

	// fnType 函数的类型。
	fnType reflect.Type
}

func newArgList(fnType reflect.Type, args []Arg) (*argList, error) {

	// 计算函数类型中包含不可变参数的数量。
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
			if n := arg.n - 1; n < 0 || n >= fixedArgCount {
				return nil, errors.New("参数索引超出函数入参的个数")
			} else {
				fnArgs[n] = arg.arg
			}
		default:
			if fixedArgCount > 0 {
				fnArgs[0] = arg
			} else if fnType.IsVariadic() {
				fnArgs = append(fnArgs, arg)
			} else {
				return nil, errors.New("函数没有参数但却绑定了参数")
			}
		}
	}

	for i := 1; i < len(args); i++ {
		switch arg := args[i].(type) {
		case *optionArg:
			fnArgs = append(fnArgs, arg)
		case IndexArg:
			if !shouldIndex {
				return nil, errors.New("所有参数必须都有或者都没有索引")
			}
			if n := arg.n - 1; n < 0 || n >= fixedArgCount {
				return nil, errors.New("参数索引超出函数入参的个数")
			} else if fnArgs[n] != nil {
				return nil, fmt.Errorf("发现相同索引 %d 的参数", arg.n)
			} else {
				fnArgs[n] = arg.arg
			}
		default:
			if shouldIndex {
				return nil, errors.New("所有参数必须都有或者都没有索引")
			}
			if i < fixedArgCount {
				fnArgs[i] = arg
			} else if fnType.IsVariadic() {
				fnArgs = append(fnArgs, arg)
			} else {
				panic(errors.New("参数的数量超出了函数入参的数量"))
			}
		}
	}

	// 其他没有传入的参数绑定默认为空字符串。
	for i := 0; i < fixedArgCount; i++ {
		if fnArgs[i] == nil {
			fnArgs[i] = ""
		}
	}

	return &argList{fnType: fnType, args: fnArgs}, nil
}

// get 返回所有绑定参数的真实值，fileLine 是函数定义所在的文件信息。
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

		// Option 参数可能因为条件不满足而没有生成绑定值
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

func (r *argList) getArg(ctx Context, arg Arg, t reflect.Type, fileLine string) (reflect.Value, error) {

	var (
		err error
		tag string
	)

	description := fmt.Sprintf("arg:\"%v\" %s", arg, fileLine)
	log.Tracef("get value %s", description)
	defer func() {
		if err == nil {
			log.Tracef("get value success %s", description)
		} else {
			log.Tracef("get value error %s %s", err.Error(), description)
		}
	}()

	switch g := arg.(type) {
	case ValueArg:
		return reflect.ValueOf(g.v), nil
	case *optionArg:
		return g.call(ctx)
	case internal.BeanDefinition:
		tag = g.ID()
	case string:
		tag = g
	default:
		tag = util.TypeName(g) + ":"
	}

	v := reflect.New(t).Elem()

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

func (r *argList) Len() int {
	return len(r.args)
}

// optionArg Option 函数的参数绑定。
type optionArg struct {
	r *Callable
	c cond.Condition
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

	log.Tracef("call option func %s", arg.r.fileLine)
	defer func() {
		if err == nil {
			log.Tracef("call option func success %s", arg.r.fileLine)
		} else {
			log.Tracef("call option func error %s %s", err.Error(), arg.r.fileLine)
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

// Callable 绑定函数及其参数，然后通过 Call 方法获取绑定函数的执行结果。
type Callable struct {
	fn       interface{}
	argList  *argList
	fileLine string
}

// Bind 绑定函数及其参数，skip 是相对于当前方法需要跳过的调用栈层数。
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

// Call 通过反射机制获取函数的绑定参数并执行函数，最后返回函数的执行结果。
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

func (r *Callable) Arg(i int) (Arg, bool) {
	if i >= r.argList.Len() {
		return nil, false
	}
	return r.argList.args[i], true
}
