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
	"io/ioutil"

	"github.com/spf13/viper"
)

func init() {
	RegisterReader(Viper("prop"), ".properties")
	RegisterReader(Viper("yaml"), ".yaml", ".yml")
	RegisterReader(Viper("toml"), ".toml")
}

// Reader 配置读取器接口
type Reader interface {
	FileExt() []string // 读取器支持的文件扩展名的列表
	ReadFile(filename string, out map[string]interface{}) error
	ReadBuffer(buffer []byte, out map[string]interface{}) error
}

// Readers 配置读取器集合
var Readers []Reader

// RegisterReader 注册基于文件的配置读取器
func RegisterReader(fn ReaderFunc, fileExt ...string) {
	Readers = append(Readers, &reader{ext: fileExt, fn: fn})
}

type ReaderFunc func(b []byte, out map[string]interface{}) error

type reader struct {
	ext []string
	fn  ReaderFunc
}

func (r *reader) FileExt() []string { return r.ext }

func (r *reader) ReadBuffer(b []byte, out map[string]interface{}) error {
	return r.fn(b, out)
}

func (r *reader) ReadFile(filename string, out map[string]interface{}) error {
	file, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	return r.ReadBuffer(file, out)
}

// Viper 使用 viper 读取配置文件
func Viper(fileType string) ReaderFunc {
	return func(b []byte, out map[string]interface{}) error {

		v := viper.New()
		v.SetConfigType(fileType)
		if err := v.ReadConfig(bytes.NewBuffer(b)); err != nil {
			return err
		}

		for _, key := range v.AllKeys() {
			val := v.Get(key)
			out[key] = val
		}
		return nil
	}
}
