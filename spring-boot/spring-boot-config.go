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

package SpringBoot

import (
	"github.com/spf13/viper"
)

//
// 配置文件解析器
//
type ConfigParser interface {
	// 文件扩展名
	FileExt() []string

	// 解析配置文件
	Parse(ctx ApplicationContext, filename string) error
}

//
// 使用 spf13/viper 实现的解析器
//
type ConfigParserViper struct {
}

//
// 文件扩展名
//
func (_ *ConfigParserViper) FileExt() []string {
	return []string{".properties", ".yaml", ".toml"}
}

//
// 解析配置文件
//
func (parser *ConfigParserViper) Parse(ctx ApplicationContext, filename string) error {

	v := viper.New()
	v.SetConfigFile(filename)
	v.ReadInConfig()

	for _, key := range v.AllKeys() {
		val := v.Get(key)
		ctx.SetProperties(key, val)
	}
	return nil
}
