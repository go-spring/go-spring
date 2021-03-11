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

	"github.com/go-spring/spring-core/bean"
	"github.com/go-spring/spring-core/cond"
	"github.com/go-spring/spring-core/log"
	"github.com/go-spring/spring-core/util"
)

type Context interface {

	// Matches 条件表达式成立返回 true
	Matches(cond cond.Condition) bool

	// BindValue 对结构体的字段进行属性绑定
	BindValue(v reflect.Value, tag string) error

	// WireValue 对结构体的字段进行绑定
	WireValue(v reflect.Value, tag string) error
}

// Arg 函数的绑定参数。
type Arg interface{}

// IndexArg 包含下标的函数绑定参数。
type IndexArg struct {
	idx int
	arg Arg
}

// Index 返回包含下标的函数绑定参数。
func Index(idx int, arg Arg) IndexArg {
	return IndexArg{idx: idx, arg: arg}
}

// R1 返回下标为 1 的函数绑定参数。
func R1(arg Arg) IndexArg { return Index(1, arg) }

// R2 返回下标为 2 的函数绑定参数。
func R2(arg Arg) IndexArg { return Index(2, arg) }

// R3 返回下标为 3 的函数绑定参数。
func R3(arg Arg) IndexArg { return Index(3, arg) }

// R4 返回下标为 4 的函数绑定参数。
func R4(arg Arg) IndexArg { return Index(4, arg) }

// R5 返回下标为 5 的函数绑定参数。
func R5(arg Arg) IndexArg { return Index(5, arg) }

// R6 返回下标为 6 的函数绑定参数。
func R6(arg Arg) IndexArg { return Index(6, arg) }

// R7 返回下标为 7 的函数绑定参数。
func R7(arg Arg) IndexArg { return Index(7, arg) }

// ValueArg 包含具体值的函数绑定参数。
type ValueArg struct {
	arg interface{}
}

// Value 返回包含具体值的函数绑定参数。
func Value(arg interface{}) ValueArg {
	return ValueArg{arg: arg}
}

// ArgList 函数绑定参数的列表。
type ArgList struct {

	// args 函数绑定参数。
	args []Arg

	// fnType 函数的类型。
	fnType reflect.Type

	// withReceiver 这里所谓的接收者是指函数的第一个参数，是否包含接收者的
	// 意思是接收者是否由 IoC 容器在执行函数时自动传入作为接收者的第一个参数。
	withReceiver bool
}

// NewArgList 返回一个新创建的函数绑定参数的列表。
func NewArgList(fnType reflect.Type, withReceiver bool, args []Arg) *ArgList {

	// 计算函数不可变参数的数量，需要排除接收者。
	fixedArgCount := fnType.NumIn()
	if fnType.IsVariadic() {
		fixedArgCount--
	}
	if withReceiver {
		fixedArgCount--
	}

	// 函数的绑定参数要么都有下标，要么都没有下标。
	shouldIndex := false

	// 分配不可变参数数量的空间，可变参数加在后面。
	fnArgs := make([]Arg, fixedArgCount)

	if len(args) > 0 {
		switch arg := args[0].(type) {
		case *option:
			fnArgs = append(fnArgs, arg)
		case IndexArg:
			shouldIndex = true
			if idx := arg.idx - 1; idx >= 0 && idx < fixedArgCount {
				fnArgs[idx] = arg.arg
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
				panic(errors.New("参数索引超出函数入参的个数"))
			}
		}
	}

	for i := 1; i < len(args); i++ {
		switch arg := args[i].(type) {
		case *option:
			fnArgs = append(fnArgs, arg)
		case IndexArg:
			if !shouldIndex {
				panic(errors.New("所有非可选参数必须都有或者都没有索引"))
			}
			if idx := arg.idx - 1; idx < 0 || idx >= fixedArgCount {
				panic(errors.New("参数索引超出函数入参的个数"))
			} else if fnArgs[idx] != nil {
				panic(fmt.Errorf("发现相同索引<%d>的参数", arg.idx))
			} else {
				fnArgs[idx] = arg.arg
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
				panic(errors.New("参数索引超出函数入参的个数"))
			}
		}
	}

	// 其他没有传入的函数绑定参数默认为空字符串。
	for i := 0; i < fixedArgCount; i++ {
		if fnArgs[i] == nil {
			fnArgs[i] = ""
		}
	}

	return &ArgList{fnType: fnType, withReceiver: withReceiver, args: fnArgs}
}

func (argList *ArgList) WithReceiver() bool { return argList.withReceiver }

// Get 返回函数所有绑定参数的真实值，fileLine 是函数定义所在的文件及其行号，供打印日志时使用。
func (argList *ArgList) Get(assembly Context, fileLine string) ([]reflect.Value, error) {

	fnType := argList.fnType
	numIn := fnType.NumIn()

	// 接收者不算作函数的绑定参数。
	if argList.withReceiver {
		numIn -= 1
	}

	variadic := fnType.IsVariadic()
	result := make([]reflect.Value, 0)

	for idx, arg := range argList.args {

		if argList.withReceiver {
			idx++
		}

		if variadic && idx >= numIn-1 {
			t := fnType.In(numIn - 1).Elem()
			v, err := argList.getArgValue(t, arg, assembly, fileLine)
			if err != nil {
				return nil, err
			}
			// 条件可能不满足所以没有对应的参数
			if v.IsValid() {
				result = append(result, v)
			}
		} else {
			t := fnType.In(idx)
			v, err := argList.getArgValue(t, arg, assembly, fileLine)
			if err != nil {
				return nil, err
			}
			result = append(result, v)
		}
	}

	return result, nil
}

func (argList *ArgList) getArgValue(t reflect.Type, arg Arg, assembly Context, fileLine string) (reflect.Value, error) {

	// TODO 检查有些 defer 像这里这样是不正确的，panic 也会打印 success 日志
	description := fmt.Sprintf("arg:\"%v\" %s", arg, fileLine)
	defer log.Tracef("get value success %s", description)
	log.Tracef("get value %s", description)

	tag := ""

	switch g := arg.(type) {
	case ValueArg:
		return reflect.ValueOf(g.arg), nil
	case *option:
		return g.call(assembly)
	case bean.Definition:
		tag = g.BeanId()
	case string:
		tag = g
	default:
		tag = util.TypeName(g) + ":"
	}

	v := reflect.New(t).Elem()

	// 处理引用类型
	if util.IsRefType(v.Kind()) {
		if err := assembly.WireValue(v, tag); err != nil {
			return reflect.Value{}, err
		}
		return v, nil
	}

	// 处理值类型
	if tag == "" {
		tag = "${}"
	}
	if err := assembly.BindValue(v, tag); err != nil {
		return reflect.Value{}, err
	}
	return v, nil
}

// option Option 函数的绑定参数
type option struct {
	fn      interface{}
	argList *ArgList

	file string // 注册点所在文件
	line int    // 注册点所在行数

	cond cond.Condition // 判断条件
}

// Option 封装 Option 函数的绑定参数。
func Option(fn interface{}, args ...Arg) *option {

	var (
		file string
		line int
	)

	// 获取注册点信息
	for i := 1; i < 10; i++ {
		_, file0, line0, _ := runtime.Caller(i)

		// 排除 spring-core 包下面所有的非 test 文件
		if strings.Contains(file0, "/spring-core/") {
			if !strings.HasSuffix(file0, "_test.go") {
				continue
			}
		}

		file = file0
		line = line0
		break
	}

	fnType := reflect.TypeOf(fn)

	// 判断是否为正确的 Option 函数定义，必要条件之一是只有一个返回值。
	if fnType.Kind() != reflect.Func || fnType.NumOut() != 1 {
		panic(errors.New("option func must be func(...)option"))
	}

	return &option{
		fn:      fn,
		argList: NewArgList(fnType, false, args),
		file:    file,
		line:    line,
	}
}

// Cond 为 Option 设置一个 cond.Condition
func (arg *option) Cond(cond cond.Condition) *option {
	arg.cond = cond
	return arg
}

func (arg *option) call(assembly Context) (ret reflect.Value, err error) {

	fileLine := fmt.Sprintf("%s:%d", arg.file, arg.line)
	log.Tracef("call option func %s", fileLine)

	defer func() {
		if err == nil {
			log.Tracef("call option func %s succeed", fileLine)
		} else {
			log.Tracef("call option func %s failed %s", fileLine, err.Error())
		}
	}()

	if arg.cond == nil || assembly.Matches(arg.cond) {
		in, err := arg.argList.Get(assembly, fileLine)
		if err != nil {
			return reflect.Value{}, err
		}
		fnValue := reflect.ValueOf(arg.fn)
		out := fnValue.Call(in)
		return out[0], nil
	}

	return reflect.Value{}, nil
}
