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
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/viper"
)

//
// 属性源
//
type PropertySource interface {
	// 加载属性文件，path 配置文件目录，profile 配置文件剖面。
	Load(path string, profile string) map[string]interface{}
}

//
// 基于普通 map 的属性源
//
type MapPropertySource struct {
	profile    string
	properties map[string]interface{}
}

//
// 构造函数
//
func NewMapPropertySource(profile string, properties map[string]interface{}) *MapPropertySource {
	return &MapPropertySource{
		profile:    profile,
		properties: properties,
	}
}

func (p *MapPropertySource) Load(path string, profile string) map[string]interface{} {
	if profile == p.profile {
		return p.properties
	}
	return nil
}

//
// 基于 k8s ConfigMap 的属性源
//
type ConfigMapPropertySource struct {
	filename string
}

//
// 构造函数
//
func NewConfigMapPropertySource(filename string) *ConfigMapPropertySource {
	return &ConfigMapPropertySource{
		filename: filename,
	}
}

func (p *ConfigMapPropertySource) Load(path string, profile string) map[string]interface{} {

	var configMapFileName string
	if filepath.IsAbs(p.filename) {
		configMapFileName = p.filename
	} else {
		configMapFileName = filepath.Join(path, p.filename)
	}

	if _, err := os.Stat(configMapFileName); err != nil {
		panic(err)
	}

	v := viper.New()
	v.SetConfigFile(configMapFileName)
	v.ReadInConfig()

	d := v.Sub("data")
	if d == nil {
		return nil
	}

	profileFileName := "application"
	if profile != "" {
		profileFileName += "-" + profile
	}

	result := make(map[string]interface{})

	for _, ext := range []string{".properties", ".yaml", ".toml"} {
		if key := profileFileName + ext; d.IsSet(key) {
			fmt.Printf(">>> load properties from config-map %s:%s\n", configMapFileName, key)

			val := d.GetString(key)
			if val == "" {
				continue
			}

			v0 := viper.New()
			v0.SetConfigType(ext[1:])
			v0.ReadConfig(strings.NewReader(val))

			v0Keys := v0.AllKeys()
			sort.Strings(v0Keys)

			for _, v0Key := range v0Keys {
				v0Val := v0.Get(v0Key)
				result[v0Key] = v0Val
				fmt.Printf("%s=%v\n", v0Key, v0Val)
			}
		}
	}

	return result
}
