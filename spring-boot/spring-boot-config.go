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
	// 属性源的名称
	Name() string

	// 加载属性文件，profile 配置文件剖面。
	Load(profile string) map[string]interface{}
}

//
// 基于默认配置文件的属性源
//
type DefaultPropertySource struct {
	fileLocation string
}

//
// 构造函数
//
func NewDefaultPropertySource(fileLocation string) *DefaultPropertySource {
	return &DefaultPropertySource{
		fileLocation: fileLocation,
	}
}

func (p *DefaultPropertySource) Name() string {
	return ""
}

func (p *DefaultPropertySource) Load(profile string) map[string]interface{} {

	fileNamePrefix := "application"
	if profile != "" {
		fileNamePrefix += "-" + profile
	}

	result := make(map[string]interface{})

	// 从预定义的文件中加载属性列表
	for _, ext := range []string{".properties", ".yaml", ".toml"} {

		filename := filepath.Join(p.fileLocation, fileNamePrefix+ext)
		if _, err := os.Stat(filename); err != nil {
			continue // 这里不需要警告
		}

		fmt.Println(">>> load properties from", filename)

		v := viper.New()
		v.SetConfigFile(filename)
		if err := v.ReadInConfig(); err != nil {
			panic(err)
		}

		keys := v.AllKeys()
		sort.Strings(keys)

		for _, k := range keys {
			v := v.Get(k)
			result[k] = v
			fmt.Printf("%s=%v\n", k, v)
		}
	}

	return result
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

func (p *ConfigMapPropertySource) Name() string {
	return "k8s"
}

func (p *ConfigMapPropertySource) Load(profile string) map[string]interface{} {

	v := viper.New()
	v.SetConfigFile(p.filename)
	if err := v.ReadInConfig(); err != nil {
		panic(err)
	}

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
			fmt.Printf(">>> load properties from config-map %s:%s\n", p.filename, key)

			val := d.GetString(key)
			if val == "" {
				continue
			}

			v0 := viper.New()
			v0.SetConfigType(ext[1:])
			if err := v0.ReadConfig(strings.NewReader(val)); err != nil {
				panic(err)
			}

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
