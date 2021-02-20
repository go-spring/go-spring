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

// IndexArg 包含下标的参数
type IndexArg struct {
	idx int
	arg Arg
}

// Index IndexArg 的构造函数
func Index(idx int, arg Arg) IndexArg {
	return IndexArg{idx: idx, arg: arg}
}

// R1 封装下标为 1 的参数
func R1(arg Arg) IndexArg { return Index(1, arg) }

// R2 封装下标为 2 的参数
func R2(arg Arg) IndexArg { return Index(2, arg) }

// R3 封装下标为 3 的参数
func R3(arg Arg) IndexArg { return Index(3, arg) }

// R4 封装下标为 4 的参数
func R4(arg Arg) IndexArg { return Index(4, arg) }

// R5 封装下标为 5 的参数
func R5(arg Arg) IndexArg { return Index(5, arg) }

// R6 封装下标为 6 的参数
func R6(arg Arg) IndexArg { return Index(6, arg) }

// R7 封装下标为 7 的参数
func R7(arg Arg) IndexArg { return Index(7, arg) }

// ValueArg 直接包含值的参数
type ValueArg struct{ v interface{} }

// Value ValueArg 的构造函数
func Value(v interface{}) ValueArg { return ValueArg{v: v} }

type ArgList struct {
	fnType       reflect.Type
	WithReceiver bool
	args         []Arg
}

func NewArgList(fnType reflect.Type, withReceiver bool, args []Arg) *ArgList {
	fnArgCount := fnType.NumIn()

	// 可选参数使用 append 所以减 1
	if fnType.IsVariadic() {
		fnArgCount--
	}

	// 接收者自动传入所以减 1
	if withReceiver {
		fnArgCount--
	}

	fnArgs := make([]Arg, fnArgCount)

	shouldIndex := false
	if len(args) > 0 {
		switch arg := args[0].(type) {
		case IndexArg:
			shouldIndex = true
			idx := arg.idx - 1
			if idx < 0 || idx >= fnArgCount {
				panic(errors.New("参数索引超出函数入参的个数"))
			}
			fnArgs[idx] = arg.arg
		case *option:
			fnArgs = append(fnArgs, arg)
		default:
			if fnArgCount == 0 {
				if fnType.IsVariadic() {
					fnArgs = append(fnArgs, arg)
				} else {
					panic(errors.New("参数索引超出函数入参的个数"))
				}
			} else {
				fnArgs[0] = arg
			}
		}
	}

	for i := 1; i < len(args); i++ {
		switch arg := args[i].(type) {
		case IndexArg:
			if !shouldIndex {
				panic(errors.New("所有非可选参数必须都有或者都没有索引"))
			}
			idx := arg.idx - 1
			if idx < 0 || idx >= fnArgCount {
				panic(errors.New("参数索引超出函数入参的个数"))
			}
			if fnArgs[idx] != nil {
				panic(fmt.Errorf("发现相同索引<%d>的参数", arg.idx))
			}
			fnArgs[idx] = arg.arg
		case *option:
			fnArgs = append(fnArgs, arg)
		default:
			if shouldIndex {
				panic(errors.New("所有非可选参数必须都有或者都没有索引"))
			}
			if i >= fnArgCount {
				if fnType.IsVariadic() {
					fnArgs = append(fnArgs, arg)
				} else {
					panic(errors.New("参数索引超出函数入参的个数"))
				}
			} else {
				fnArgs[i] = arg
			}
		}
	}

	for i := 0; i < fnArgCount; i++ {
		if fnArgs[i] == nil {
			fnArgs[i] = ""
		}
	}

	return &ArgList{fnType: fnType, WithReceiver: withReceiver, args: fnArgs}
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

		idx := i
		if argList.WithReceiver {
			idx++
		}

		if variadic && idx >= numIn-1 {
			it := fnType.In(numIn - 1).Elem()
			ev := argList.getArgValue(it, arg, assembly, fileLine)
			if ev.IsValid() { // 条件可能不满足所以没有对应的参数
				result = append(result, ev)
			}
		} else {
			it := fnType.In(idx)
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

	selector := ""
	switch tArg := arg.(type) {
	case ValueArg:
		return reflect.ValueOf(tArg.v)
	case *option:
		return tArg.call(assembly)
	case bean.Definition:
		selector = tArg.BeanId()
	case string:
		selector = tArg
	default:
		selector = util.TypeName(tArg) + ":"
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

	return &option{
		fn:      fn,
		argList: NewArgList(fnType, false, args),
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
