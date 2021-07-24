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

package errors

import (
	"errors"
	"fmt"
	"runtime"
)

var (
	New    = errors.New
	Is     = errors.Is
	As     = errors.As
	Unwrap = errors.Unwrap
)

// ToString 返回 error 的字符串。
func ToString(err error) string {
	if err == nil {
		return "<nil>"
	}
	return err.Error()
}

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

// Cause 获取封装的异常源。
func Cause(err error) interface{} {
	if c, ok := err.(*withCause); ok {
		return c.cause
	}
	return err
}

// WithFileLine 返回错误发生的文件行号，skip 是相对于当前函数的深度。
func WithFileLine(err error, skip int) error {
	_, file, line, _ := runtime.Caller(skip + 1)
	str := fmt.Sprintf("%s:%d", file, line)
	if err != nil {
		str += ": " + err.Error()
	}
	return New(str)
}
