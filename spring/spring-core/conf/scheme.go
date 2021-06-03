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

package conf

import (
	"github.com/go-spring/spring-core/conf/scheme"
	"github.com/go-spring/spring-core/conf/scheme/file"
	"github.com/go-spring/spring-core/conf/scheme/k8s"
)

const MaxSchemeNameLength = 16

func init() {
	NewScheme(file.New, "")
	NewScheme(k8s.New, "k8s")
}

var schemeMap = make(map[string]scheme.Factory)

// NewScheme 注册读取属性列表文件内容的方案，name 最长不超过 16 个字符。
func NewScheme(s scheme.Factory, name string) {
	schemeMap[name] = s
}
