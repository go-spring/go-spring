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

package local

import (
	"io/ioutil"
	"net/url"
	"path/filepath"

	"github.com/go-spring/spring-core/util"
)

// Scheme 从扩展后的 URL 读取文件数据，同时返回文件的扩展名，读取失败时返回错误。
func Scheme(u *url.URL) (_ []byte, ext string, _ error) {

	if u.Host != "" {
		return nil, "", util.UnimplementedMethod
	}

	b, err := ioutil.ReadFile(u.Path)
	if err != nil {
		return nil, "", err
	}

	return b, filepath.Ext(u.Path), nil
}
