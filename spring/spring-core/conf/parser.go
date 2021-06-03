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
	"strings"

	"github.com/go-spring/spring-core/conf/parser"
	"github.com/go-spring/spring-core/conf/parser/prop"
	"github.com/go-spring/spring-core/conf/parser/toml"
	"github.com/go-spring/spring-core/conf/parser/yaml"
)

func init() {
	NewParser(prop.New(), ".properties", ".prop")
	NewParser(yaml.New(), ".yaml", ".yml")
	NewParser(toml.New(), ".toml")
}

var parserMap = make(map[string]parser.Parser)

// NewParser 注册属性列表解析器，ext 是解析器支持的文件扩展名。
func NewParser(p parser.Parser, ext ...string) {
	for _, s := range ext {
		parserMap[strings.ToLower(s)] = p
	}
}
