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
	"bytes"
	"github.com/go-spring/go-spring-parent/spring-utils"
	"github.com/magiconair/properties"
	"github.com/spf13/viper"
	"io/ioutil"
	"os"
	"sort"
)

// configReaders 各种格式配置文件的读取器集合
var configReaders = map[string]ConfigReader{
	".properties": new(PropertiesReader),
	".yaml":       &ViperReader{"yaml"},
	".toml":       &ViperReader{"toml"},
}

type ConfigReader interface {
	ReadFile(filename string, out map[string]interface{})
	ReadBuffer(buffer []byte, out map[string]interface{})
}

// PropertiesReader 读取 properties 格式的配置文件
type PropertiesReader struct{}

func (r *PropertiesReader) ReadFile(filename string, out map[string]interface{}) {
	file, err := ioutil.ReadFile(filename)
	SpringUtils.Panic(err).When(err != nil && err != os.ErrNotExist)
	r.ReadBuffer(file, out)
}

func (r *PropertiesReader) ReadBuffer(buffer []byte, out map[string]interface{}) {

	p := properties.NewProperties()
	p.DisableExpansion = true

	err := p.Load(buffer, properties.UTF8)
	SpringUtils.Panic(err).When(err != nil)

	for _, key := range p.Keys() {
		value, _ := p.Get(key)
		out[key] = value
	}
}

// ViperReader 读取 yaml、toml 等格式的配置文件
type ViperReader struct {
	fileType string // yaml、toml 等
}

func (r *ViperReader) readViper(v *viper.Viper, out map[string]interface{}) {

	keys := v.AllKeys()
	sort.Strings(keys)

	for _, key := range keys {
		val := v.Get(key)
		out[key] = val
	}
}

func (r *ViperReader) ReadFile(filename string, out map[string]interface{}) {

	v := viper.New()
	v.SetConfigFile(filename)

	err := v.ReadInConfig()
	SpringUtils.Panic(err).When(err != nil)

	r.readViper(v, out)
}

func (r *ViperReader) ReadBuffer(buffer []byte, out map[string]interface{}) {

	v := viper.New()
	v.SetConfigType(r.fileType)

	err := v.ReadConfig(bytes.NewBuffer(buffer))
	SpringUtils.Panic(err).When(err != nil)

	r.readViper(v, out)
}
