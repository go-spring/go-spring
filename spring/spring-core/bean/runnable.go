package bean

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/go-spring/spring-core/util"
)

// Runnable 执行器，不能返回 error 以外的其他值
type Runnable struct {
	Fn        interface{}
	StringArg *fnStringBindingArg // 一般参数绑定
	OptionArg *FnOptionBindingArg // Option 绑定

	withReceiver bool          // 函数是否包含接收者，也可以假装第一个参数是接收者
	receiver     reflect.Value // 接收者的值
}

// Run 运行执行器
func (r *Runnable) Run(assembly beanAssembly) error {

	// 获取函数定义所在的文件及其行号信息
	file, line, _ := util.FileLine(r.Fn)
	fileLine := fmt.Sprintf("%s:%d", file, line)

	// 组装 fn 调用所需的参数列表
	var in []reflect.Value

	if r.withReceiver {
		in = append(in, r.receiver)
	}

	if r.StringArg != nil {
		if v := r.StringArg.Get(assembly, fileLine); len(v) > 0 {
			in = append(in, v...)
		}
	}

	if r.OptionArg != nil {
		if v := r.OptionArg.Get(assembly, fileLine); len(v) > 0 {
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

	panic(errors.New("error func type"))
}
