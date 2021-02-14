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

package core

import (
	"errors"
	"reflect"

	"github.com/go-spring/spring-core/log"
)

// Runner 立即执行器
type Runner struct {
	r   Runnable
	ctx *applicationContext
}

// newRunner Runner 的构造函数，fn 不能返回 error 以外的其他值
func newRunner(ctx *applicationContext, fn interface{}, args []Arg) *Runner {

	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func {
		panic(errors.New("fn must be a func"))
	}

	return &Runner{
		ctx: ctx,
		r: Runnable{
			Fn:      fn,
			argList: NewArgList(fnType, false, args),
		},
	}
}

func (r *Runner) run() error {
	assembly := newDefaultBeanAssembly(r.ctx)

	defer func() { // 捕获自动注入过程中的异常，打印错误日志然后重新抛出
		if err := recover(); err != nil {
			log.Errorf("%v ↩\n%s", err, assembly.wiringStack.path())
			panic(err)
		}
	}()

	return r.r.Run(assembly)
}
