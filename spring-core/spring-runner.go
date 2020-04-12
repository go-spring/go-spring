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
	"errors"
	"fmt"
	"reflect"
	"runtime"
)

type Runner interface {
	Run(fn interface{}, tags ...string)
}

type emptyRunner struct{}

func (r *emptyRunner) Run(fn interface{}, tags ...string) {}

type runner struct {
	ctx SpringContext
}

func (r *runner) Run(fn interface{}, tags ...string) {

	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func {
		panic(errors.New("fn must be a func"))
	}

	_, file, line, _ := runtime.Caller(1)
	strCaller := fmt.Sprintf("%s:%d", file, line)

	arg := newFnStringBindingArg(fnType, false, tags)
	if ctx, ok := r.ctx.(*defaultSpringContext); ok {
		ctx.checkAutoWired() // 检查是否开始自动注入

		a := &defaultBeanAssembly{springCtx: ctx}
		c := &defaultCaller{caller: strCaller}
		reflect.ValueOf(fn).Call(arg.Get(a, c))
	}
}
