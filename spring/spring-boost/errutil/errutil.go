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

package errutil

import (
	"fmt"
	"runtime"
)

// ToString 返回 error 的字符串。
func ToString(err error) string {
	if err == nil {
		return "<nil>"
	}
	return err.Error()
}

// WithFileLine 返回错误发生的文件行号，skip 是相对于当前函数的深度。
func WithFileLine(err error, skip int) error {
	_, file, line, _ := runtime.Caller(skip + 1)
	return fmt.Errorf("%s:%d: %w", file, line, err)
}
