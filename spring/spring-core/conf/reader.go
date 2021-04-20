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
	// TODO 不使用 viper 实现 properties 因为支持引用
	RegisterReader(Viper("prop"), ".properties")
	RegisterReader(Viper("yaml"), ".yaml", ".yml")
	RegisterReader(Viper("toml"), ".toml")
}

// Reader 属性读取器接口
type Reader interface {
	FileExt() []string // 属性读取器支持的文件扩展名的列表
	ReadFile(p Properties, filename string) error
	ReadBuffer(p Properties, b []byte) error
}

var readers []Reader

// EachReader 遍历属性读取器列表
func EachReader(fn func(r Reader) error) error {
	for _, r := range readers {
		if err := fn(r); err != nil {
			return err
		}
	}
	return nil
}

// RegisterReader 注册属性读取器
func RegisterReader(fn ReaderFunc, fileExt ...string) {
	readers = append(readers, &reader{ext: fileExt, fn: fn})
}

type ReaderFunc func(p Properties, b []byte) error

// reader 属性读取接口的默认实现。
type reader struct {
	ext []string
	fn  ReaderFunc
}

// FileExt 返回属性读取器对应的文件扩展名的列表
func (r *reader) FileExt() []string { return r.ext }

// ReadBuffer 从内存中读取当前属性读取器支持的格式。
func (r *reader) ReadBuffer(p Properties, b []byte) error {
	return r.fn(p, b)
}

// ReadFile 从文件中读取当前属性读取器支持的格式。
func (r *reader) ReadFile(p Properties, filename string) error {
	file, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	return r.ReadBuffer(p, file)
}

// Viper 使用 viper 读取 fileType 类型的属性文件。
func Viper(fileType string) ReaderFunc {
	return func(p Properties, b []byte) error {

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
