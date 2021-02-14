package core

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/go-spring/spring-core/util"
)

// Runnable 执行器，不能返回 error 以外的其他值
type Runnable struct {
	Fn       interface{}
	argList  *ArgList
	receiver reflect.Value // 接收者的值
}

// newRunnable Runnable 的构造函数
func newRunnable(fn interface{}, fnType reflect.Type, receiver reflect.Value, args []Arg) *Runnable {
	return &Runnable{Fn: fn, receiver: receiver, argList: NewArgList(fnType, receiver.IsValid(), args)}
}

// Run 运行执行器
func (r *Runnable) run(assembly beanAssembly) error {

	// 获取函数定义所在的文件及其行号信息
	file, line, _ := util.FileLine(r.Fn)
	fileLine := fmt.Sprintf("%s:%d", file, line)

	// 组装 fn 调用所需的参数列表
	var in []reflect.Value

	if r.argList.withReceiver {
		in = append(in, r.receiver)
	}

	if r.argList != nil {
		if v := r.argList.Get(assembly, fileLine); len(v) > 0 {
			in = append(in, v...)
		}
	}

	// 调用 fn 函数
	out := reflect.ValueOf(r.Fn).Call(in)

	// 获取 error 返回值
	if n := len(out); n == 0 {
		return nil
	} else if n == 1 {
		if o := out[0]; o.Type() == errorType {
			if i := o.Interface(); i == nil {
				return nil
			} else {
				return i.(error)
			}
		}
	}

	return errors.New("error func type")
}
