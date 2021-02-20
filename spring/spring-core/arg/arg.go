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

type Arg interface{}

type ArgList struct {
	fnType       reflect.Type
	WithReceiver bool
	args         []Arg
}

func NewArgList(fnType reflect.Type, withReceiver bool, args []Arg) *ArgList {
	return &ArgList{fnType: fnType, WithReceiver: withReceiver, args: args}
}

// Get 获取函数参数的绑定值，fileLine 是函数所在文件及其行号，日志使用
func (argList *ArgList) Get(assembly bean.Assembly, fileLine string) []reflect.Value {

	fnType := argList.fnType
	numIn := fnType.NumIn()

	// 第一个参数是接收者
	if argList.WithReceiver {
		numIn -= 1
	}

	variadic := fnType.IsVariadic()
	result := make([]reflect.Value, 0)

	for i, arg := range argList.args {
		var it reflect.Type

		if variadic && i >= numIn-1 {
			if argList.WithReceiver {
				it = fnType.In(numIn)
			} else {
				it = fnType.In(numIn - 1)
			}
		} else {
			if argList.WithReceiver {
				it = fnType.In(i + 1)
			} else {
				it = fnType.In(i)
			}
		}

		if variadic && i >= numIn-1 { // 可变参数
			ev := argList.getArgValue(it.Elem(), arg, assembly, fileLine)
			if ev.IsValid() {
				result = append(result, ev)
			}
		} else {
			iv := argList.getArgValue(it, arg, assembly, fileLine)
			result = append(result, iv)
		}
	}

	return result
}

// getArgValue 获取绑定参数值
func (argList *ArgList) getArgValue(t reflect.Type, arg Arg, assembly bean.Assembly, fileLine string) reflect.Value {

	// TODO 检查有些 defer 像这里这样是不正确的，panic 也会打印 success 日志
	description := fmt.Sprintf("arg:\"%v\" %s", arg, fileLine)
	defer log.Tracef("get value success %s", description)
	log.Tracef("get value %s", description)

	if arg == nil {
		panic(errors.New("selector can't be nil or empty"))
	}

	selector := ""
	switch tArg := arg.(type) {
	//case *BeanDefinition: TODO 怎么支持呢？
	//	selector = tArg.BeanId()
	case *option:
		return tArg.call(assembly)
	case string:
		selector = tArg
	default:
		selector = bean.TypeName(tArg) + ":"
	}

	v := reflect.New(t).Elem()
	if util.IsValueType(v.Kind()) { // 值类型，采用属性绑定语法
		if selector == "" {
			selector = "${}"
		}
		err := assembly.BindValue(v, selector)
		util.Panic(err).When(err != nil)
	} else { // 引用类型，采用对象注入语法
		err := assembly.WireValue(v, selector)
		util.Panic(err).When(err != nil)
	}
	return v
}

type value struct{ v interface{} }

func Value(v interface{}) *value { return &value{v: v} }

// option Option 函数的绑定参数
type option struct {
	cond cond.Condition // 判断条件

	fn      interface{}
	argList *ArgList

	file string // 注册点所在文件
	line int    // 注册点所在行数
}

// 判断是否是合法的 Option 函数，只能有一个返回值
func validOptionFunc(fnType reflect.Type) bool {
	return fnType.Kind() == reflect.Func && fnType.NumOut() == 1
}

// Option Option 的构造函数，args 是 Option 函数的一般参数绑定
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
	if ok := validOptionFunc(fnType); !ok {
		panic(errors.New("option func must be func(...)option"))
	}

	fnArgs := make([]Arg, len(args))
	for i, arg := range args {
		fnArgs[i] = arg
	}

	return &option{
		fn:      fn,
		argList: NewArgList(fnType, false, fnArgs),
		file:    file,
		line:    line,
	}
}

func (arg *option) FileLine() string {
	return fmt.Sprintf("%s:%d", arg.file, arg.line)
}

// Cond 为 Option 设置一个 cond.Condition
func (arg *option) Cond(cond cond.Condition) *option {
	arg.cond = cond
	return arg
}

// call 获取 Option 的运算值
func (arg *option) call(assembly bean.Assembly) reflect.Value {

	defer log.Tracef("call option func success %s", arg.FileLine())
	log.Tracef("call option func %s", arg.FileLine())

	ctx := assembly.ConditionContext().(cond.ConditionContext)
	if arg.cond == nil || arg.cond.Matches(ctx) {
		fnValue := reflect.ValueOf(arg.fn)
		in := arg.argList.Get(assembly, arg.FileLine())
		out := fnValue.Call(in)
		return out[0]
	}

	return reflect.Value{}
}

type Map struct{}

// 返回带索引的参数
func R1(arg Arg) {

}

func R2(arg Arg) {

}

func R3(arg Arg) {

}

func R4(arg Arg) {

}

func R5(arg Arg) {

}

func R6(arg Arg) {

}
