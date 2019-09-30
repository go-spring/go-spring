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

package SpringCheck

import "errors"

//
// 检查错误是否为空，如果不为空，则触发 panic 。
//
func CheckError(err error) {
	if err != nil {
		panic(err)
	}
}

//
// 检查变量是否不为空，如果为空，则触发 panic 。TODO 请仔细验证该函数的效果，
// 参见 https://github.com/didi/go-spring/issues/9 的讨论。
//
func CheckNotNull(i interface{}) {
	if i == nil {
		panic(errors.New("should not be nil"))
	}
}

//
// 检查字符串是否不为空，如果为空，则触发 panic 。
//
func CheckNotEmpty(str string) {
	if str == "" {
		panic(errors.New("should not be empty"))
	}
}
