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

package util

import (
	"errors"
	"fmt"
	"runtime"
)

type withCause struct {
	cause interface{}
}

// WithCause 封装一个异常源。
func WithCause(r interface{}) error {
	return &withCause{cause: r}
}

func (c *withCause) Error() string {
	return fmt.Sprint(c.cause)
}

// Cause 获取封装的异常源
func Cause(err error) interface{} {
	if c, ok := err.(*withCause); ok {
		return c.cause
	}
	return err
}

// Error 返回 error 的字符串。
func Error(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// ErrorWithFileLine 返回错误发生的文件行号，skip 是相对于当前函数的深度
func ErrorWithFileLine(err error, skip ...int) error {
	var (
		file string
		line int
	)
	if len(skip) > 0 {
		_, file, line, _ = runtime.Caller(skip[0] + 1)
	} else {
		_, file, line, _ = runtime.Caller(1)
	}
	str := fmt.Sprintf("%s:%d", file, line)
	if err != nil {
		str += ": " + err.Error()
	}
	return errors.New(str)
}

/******************** PanicCond **********************/

// Panic 抛出一个异常值
func Panic(err error) *PanicCond {
	return NewPanicCond(func() interface{} { return err })
}

// Panicf 抛出一段需要格式化的错误字符串
func Panicf(format string, a ...interface{}) *PanicCond {
	return NewPanicCond(func() interface{} { return fmt.Errorf(format, a...) })
}

// PanicCond 封装触发 panic 的条件
type PanicCond struct {
	fn func() interface{}
}

// NewPanicCond PanicCond 的构造函数
func NewPanicCond(fn func() interface{}) *PanicCond {
	return &PanicCond{fn}
}

// When 满足给定条件时抛出一个 panic
func (p *PanicCond) When(isPanic bool) {
	if isPanic {
		panic(p.fn())
	}
}
