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

package scheme

import "github.com/go-spring/spring-core/conf/fs"

// Scheme 定义读取属性列表文件内容的方案，可以是读取完整的文件，也可以是读取文件
// 的某一部分。通过与 fs.FS 对象配合，既可以从本地读，也可以从远程读。
type Scheme interface {

	// Open 打开文件读取器。
	Open(location string) (Reader, error)

	// Split splits path immediately following the final Separator。
	Split(path string) (dir, file string)
}

// Factory Scheme 的工厂方法，fs.FS 可能用于 Split 文件路径。
type Factory func(fs fs.FS) Scheme

// Reader 文件读取器，filename 对应的文件不存在时必须返回 os.ErrNotExist 。
type Reader interface {
	ReadFile(filename string) (b []byte, ext string, err error)
}
