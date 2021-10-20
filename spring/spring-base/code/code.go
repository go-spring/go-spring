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

package code

import (
	"fmt"
	"path/filepath"
	"runtime"
)

// Line 获取当前调用点的文件信息，希望未来可以实现编译期计算。
func Line() string {
	_, file, line, _ := runtime.Caller(1)
	_, file = filepath.Split(file)
	return fmt.Sprintf("%s:%d", file, line)
}
