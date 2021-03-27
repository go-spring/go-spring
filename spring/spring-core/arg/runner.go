package arg

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/go-spring/spring-core/util"
)

// errorType error 的反射类型
var errorType = reflect.TypeOf((*error)(nil)).Elem()

// Runner 执行器，不能返回 error 以外的其他值
type Runner struct {
	fn  interface{}
	arg *ArgList
}

// NewRunner Runner 的构造函数
func NewRunner(fn interface{}, arg *ArgList) *Runner {
	return &Runner{fn: fn, arg: arg}
}

// Run 运行执行器
func (r *Runner) Run(ctx Context, receiver ...reflect.Value) error {

	// 获取函数定义所在的文件及其行号信息
	file, line, _ := util.FileLine(r.fn)
	fileLine := fmt.Sprintf("%s:%d", file, line)

	// 组装 fn 调用所需的参数列表
	var in []reflect.Value

	if r.arg.WithReceiver() {
		in = append(in, receiver[0])
	}

	if r.arg != nil {
		v, err := r.arg.Get(ctx, fileLine)
		if err != nil {
			return err
		}
		if len(v) > 0 {
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
