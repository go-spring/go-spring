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
	"bytes"
	"strings"

	"github.com/go-spring/spring-core/java"
	"github.com/spf13/viper"
)

func init() {
	NewReader(Java(), "properties", "prop")
	NewReader(Viper("yaml"), "yaml", "yml")
	NewReader(Viper("toml"), "toml")
}

type ReaderFunc func(p *Properties, b []byte) error

// reader 属性读取器接口
type reader interface {
	Read(p *Properties, b []byte) error
}

var readers = make(map[string]reader)

// NewReader 注册属性读取器
func NewReader(f ReaderFunc, configTypes ...string) {
	for _, configType := range configTypes {
		configType = strings.ToLower(configType)
		readers[configType] = funcReader(f)
	}
}

type funcReader ReaderFunc

func (r funcReader) Read(p *Properties, b []byte) error {
	return r(p, b)
}

// Java 读取 java properties 属性文件。
func Java() ReaderFunc {
	return func(p *Properties, b []byte) error {
		m, err := java.ReadProperties(b)
		if err != nil {
			return err
		}
		for k, v := range m {
			p.Set(k, v)
		}
		return nil
	}
}

// Viper 读取 fileType 类型的属性文件。
func Viper(fileType string) ReaderFunc {
	return func(p *Properties, b []byte) error {

		v := viper.New()
		v.SetConfigType(fileType)
		if err := v.ReadConfig(bytes.NewBuffer(b)); err != nil {
			return err
		}

		for _, key := range v.AllKeys() {
			val := v.Get(key)
			p.Set(key, val)
		}
		return nil
	}
}
