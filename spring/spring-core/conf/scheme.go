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
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Scheme 属性源接口
type Scheme interface {

	// Load 加载符合条件的属性文件，fileLocation 是配置文件所在的目录或者数据文件，
	// fileName 是配置文件的名称，但不包含扩展名。
	Load(p *Properties, fileLocation string, fileName string, configTypes []string) error
}

func init() {
	NewScheme(defaultScheme, "")
	NewScheme(configMapScheme, "k8s")
}

var schemes = make(map[string]Scheme)

// NewScheme 注册属性源
func NewScheme(ps Scheme, name string) {
	schemes[name] = ps
}

func FindScheme(name string) (Scheme, bool) {
	ps, ok := schemes[name]
	return ps, ok
}

type FuncScheme func(p *Properties, fileLocation string, fileName string, configTypes []string) error

func (f FuncScheme) Load(p *Properties, fileLocation string, fileName string, configTypes []string) error {
	return f(p, fileLocation, fileName, configTypes)
}

// defaultScheme 整个文件都是属性
var defaultScheme = FuncScheme(func(p *Properties, fileLocation string, fileName string, configTypes []string) error {
	for _, configType := range configTypes {
		file := filepath.Join(fileLocation, fileName+"."+configType)
		if _, err := os.Stat(file); err != nil {
			continue
		}
		if err := p.Load(file); err != nil {
			return err
		}
	}
	return nil
})

// configMapScheme 基于 k8s ConfigMap 的属性源
var configMapScheme = FuncScheme(func(p *Properties, fileLocation string, fileName string, configTypes []string) error {

	v := viper.New()
	v.SetConfigFile(fileLocation)
	if err := v.ReadInConfig(); err != nil {
		return err
	}

	d := v.Sub("data")
	if d == nil {
		return fmt.Errorf("data not found in config-map %s", fileLocation)
	}

	for _, configType := range configTypes {
		key := fileName + "." + configType
		if !d.IsSet(key) {
			continue
		}
		val := d.GetString(key)
		if len(val) == 0 {
			continue
		}
		err := p.Read([]byte(val), configType)
		if err != nil {
			return err
		}
	}
	return nil
})
