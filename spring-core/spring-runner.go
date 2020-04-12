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

package SpringCore

import (
	"fmt"
	"reflect"
	"runtime"
)

// runner 执行器
type runner struct {
	ctx       *defaultSpringContext
	fn        interface{}
	stringArg *fnStringBindingArg // 普通参数绑定
	optionArg *fnOptionBindingArg // Option 绑定
}

// Options 设置 Option 模式函数的参数绑定
func (r *runner) Options(options ...*optionArg) *runner {
	r.optionArg = &fnOptionBindingArg{options}
	return r
}

// When 参数为 true 时执行器运行
func (r *runner) When(ok bool) {

	if !ok {
		return
	}

	fnValue := reflect.ValueOf(r.fn)
	fnPtr := fnValue.Pointer()
	fnInfo := runtime.FuncForPC(fnPtr)
	file, line := fnInfo.FileLine(fnPtr)
	strCaller := fmt.Sprintf("%s:%d", file, line)

	a := &defaultBeanAssembly{springCtx: r.ctx}
	c := &defaultCaller{caller: strCaller}

	var in []reflect.Value

	if r.stringArg != nil {
		if v := r.stringArg.Get(a, c); len(v) > 0 {
			in = append(in, v...)
		}
	}

	if r.optionArg != nil {
		if v := r.optionArg.Get(a, c); len(v) > 0 {
			in = append(in, v...)
		}
	}

	reflect.ValueOf(r.fn).Call(in)
}
