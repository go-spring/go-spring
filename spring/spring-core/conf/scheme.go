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

	"github.com/go-spring/spring-core/log"
	"github.com/spf13/viper"
)

func init() {
	RegisterScheme(defaultScheme, "")
	RegisterScheme(configMapScheme, "k8s")
}

var schemeMap = make(map[string]Scheme)

func FindScheme(name string) (Scheme, bool) {
	ps, ok := schemeMap[name]
	return ps, ok
}

// RegisterScheme 注册属性源
func RegisterScheme(ps Scheme, name string) {
	schemeMap[name] = ps
}

// Scheme 属性源接口
type Scheme interface {

	// Load 加载符合条件的属性文件，fileLocation 是配置文件所在的目录或者数据文件，
	// fileName 是配置文件的名称，但不包含扩展名。通过遍历配置读取器获取存在的配置文件。
	Load(p Properties, fileLocation string, fileName string) error
}

type FuncScheme func(p Properties, fileLocation string, fileName string) error

func (fn FuncScheme) Load(p Properties, fileLocation string, fileName string) error {
	return fn(p, fileLocation, fileName)
}

// defaultScheme 整个文件都是属性
var defaultScheme = FuncScheme(func(p Properties, fileLocation string, fileName string) error {
	err := EachReader(func(r Reader) error {
		var file string

		for _, ext := range r.FileExt() {
			s := filepath.Join(fileLocation, fileName+ext)
			if _, err := os.Stat(s); err == nil {
				file = s
				break
			}
		}

		if file == "" {
			return nil
		}

		log.Info("load properties from file ", file)
		err := r.ReadFile(p, file)
		if err != nil && err != os.ErrNotExist {
			return err
		}
		return nil
	})

	if err != nil {
		return err
	}
	return nil
})

// configMapScheme 基于 k8s ConfigMap 的属性源
var configMapScheme = FuncScheme(func(p Properties, fileLocation string, fileName string) error {

	v := viper.New()
	v.SetConfigFile(fileLocation)
	if err := v.ReadInConfig(); err != nil {
		return err
	}

	d := v.Sub("data")
	if d == nil {
		return fmt.Errorf("data not found in config-map %s", fileLocation)
	}

	err := EachReader(func(r Reader) error {
		for _, ext := range r.FileExt() {

			key := fileName + ext
			if !d.IsSet(key) {
				continue
			}

			log.Infof("load properties from config-map %s:%s", fileLocation, key)
			if val := d.GetString(key); val != "" {
				if err := r.ReadBuffer(p, []byte(val)); err != nil {
					return err
				}
			}
		}
		return nil
	})

	if err != nil {
		return err
	}
	return nil
})
