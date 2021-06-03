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

package http

import (
	"path"

	"github.com/go-spring/spring-core/conf/fs"
	"github.com/go-spring/spring-core/util"
)

// FS 支持通过 HTTP 协议读取文件内容的 fs.FS 实现。
type FS struct{}

func New() fs.FS {
	return &FS{}
}

// Join joins any number of path elements into a single path。
func (_ *FS) Join(elem ...string) string {
	return path.Join(elem...)
}

// Split splits path immediately following the final slash。
func (_ *FS) Split(fileURL string) (dir, file string) {
	return path.Split(fileURL)
}

// ReadFile 从文件所在路径读取完成的文件内容。
func (_ *FS) ReadFile(filename string) (b []byte, ext string, err error) {
	return nil, "", util.UnimplementedMethod
}
