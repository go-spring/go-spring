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

package boot

import (
	"bytes"
	"io/ioutil"
	"os"
	"sort"

	"github.com/go-spring/spring-core/util"
	"github.com/magiconair/properties"
	"github.com/spf13/viper"
)

func init() {
	RegisterFileConfigReader(".properties", propertiesReadBuffer)
	RegisterFileConfigReader(".yaml", viperReadBuffer("yaml"))
	RegisterFileConfigReader(".toml", viperReadBuffer("toml"))
}

// ConfigReader 配置读取器接口
type ConfigReader interface {
	FileExt() string // 文件扩展名
	ReadFile(filename string, out map[string]interface{})
	ReadBuffer(buffer []byte, out map[string]interface{})
}

// configReaders 配置读取器集合
var configReaders []ConfigReader

// RegisterConfigReader 注册配置读取器
func RegisterConfigReader(reader ConfigReader) {
	configReaders = append(configReaders, reader)
}

// RegisterFileConfigReader 注册基于文件的配置读取器
func RegisterFileConfigReader(fileExt string, fn FnReadBuffer) {
	RegisterConfigReader(NewFileConfigReader(fileExt, fn))
}

type FnReadBuffer func(buffer []byte, out map[string]interface{})

// FileConfigReader 基于文件的配置读取器
type FileConfigReader struct {
	ext string
	fn  FnReadBuffer
}

// NewFileConfigReader FileConfigReader 的构造函数
func NewFileConfigReader(fileExt string, fn FnReadBuffer) *FileConfigReader {
	return &FileConfigReader{ext: fileExt, fn: fn}
}

func (r *FileConfigReader) FileExt() string {
	return r.ext
}

func (r *FileConfigReader) ReadFile(filename string, out map[string]interface{}) {
	file, err := ioutil.ReadFile(filename)
	util.Panic(err).When(err != nil && err != os.ErrNotExist)
	r.ReadBuffer(file, out)
}

func (r *FileConfigReader) ReadBuffer(buffer []byte, out map[string]interface{}) {
	r.fn(buffer, out)
}

// propertiesReadBuffer 使用 properties 读取配置文件
func propertiesReadBuffer(buffer []byte, out map[string]interface{}) {

	p := properties.NewProperties()
	p.DisableExpansion = true

	err := p.Load(buffer, properties.UTF8)
	util.Panic(err).When(err != nil)

	for _, key := range p.Keys() {
		value, _ := p.Get(key)
		out[key] = value
	}
}

// viperReadBuffer 使用 viper 读取配置文件
func viperReadBuffer(fileType string) FnReadBuffer {
	return func(buffer []byte, out map[string]interface{}) {

		v := viper.New()
		v.SetConfigType(fileType)

		err := v.ReadConfig(bytes.NewBuffer(buffer))
		util.Panic(err).When(err != nil)

		keys := v.AllKeys()
		sort.Strings(keys)

		for _, key := range keys {
			val := v.Get(key)
			out[key] = val
		}
	}
}
