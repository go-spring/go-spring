package core

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/go-spring/spring-core/arg"
	"github.com/go-spring/spring-core/bean"
	"github.com/go-spring/spring-core/util"
)

// runnable 执行器，不能返回 error 以外的其他值
type runnable struct {
	fn  interface{}
	arg *arg.ArgList
}

// newRunnable Runnable 的构造函数
func newRunnable(fn interface{}, arg *arg.ArgList) *runnable {
	return &runnable{fn: fn, arg: arg}
}

// run 运行执行器
func (r *runnable) Run(assembly bean.Assembly, receiver ...reflect.Value) error {

	// 获取函数定义所在的文件及其行号信息
	file, line, _ := util.FileLine(r.fn)
	fileLine := fmt.Sprintf("%s:%d", file, line)

	// 组装 fn 调用所需的参数列表
	var in []reflect.Value

	if r.arg.WithReceiver {
		in = append(in, receiver[0])
	}

	if r.arg != nil {
		if v := r.arg.Get(assembly, fileLine); len(v) > 0 {
			in = append(in, v...)
		}
	}

	// 调用 fn 函数
	out := reflect.ValueOf(r.fn).Call(in)

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
