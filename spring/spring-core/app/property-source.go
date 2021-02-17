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

package app

import (
	"os"
	"path/filepath"

	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/log"
	"github.com/go-spring/spring-core/util"
	"github.com/spf13/viper"
)

func init() {
	RegisterPropertySource(&configMapPropertySource{})
}

// PropertySource 属性源接口
type PropertySource interface {

	// Scheme 返回属性源的标识
	Scheme() string

	// Load 加载属性文件，profile 配置文件剖面，fileLocation 和属性源相关。
	Load(fileLocation string, profile string) map[string]interface{}
}

// propertySources 属性源集合
var propertySources = make(map[string]PropertySource)

// RegisterPropertySource 注册属性源
func RegisterPropertySource(ps PropertySource) {
	propertySources[ps.Scheme()] = ps
}

// defaultPropertySource 基于默认配置文件的属性源
type defaultPropertySource struct{}

// Scheme 返回属性源的标识
func (p *defaultPropertySource) Scheme() string {
	return ""
}

// Load 加载属性文件，profile 配置文件剖面，fileLocation 配置文件所在目录。
func (p *defaultPropertySource) Load(fileLocation string, profile string) map[string]interface{} {

	fileNamePrefix := "application"
	if profile != "" {
		fileNamePrefix += "-" + profile
	}

	result := make(map[string]interface{})

	// 从预定义的文件格式中加载属性值列表
	for _, reader := range conf.Readers {
		var filename string

		for _, ext := range reader.FileExt() {
			s := filepath.Join(fileLocation, fileNamePrefix+ext)
			if _, err := os.Stat(s); err != nil {
				continue // 这里不需要警告
			}
			filename = s
			break
		}

		if filename == "" {
			continue
		}

		log.Info("load properties from file ", filename)
		err := reader.ReadFile(filename, result)
		util.Panic(err).When(err != nil && err != os.ErrNotExist)
	}

	return result
}

// configMapPropertySource 基于 k8s ConfigMap 的属性源
type configMapPropertySource struct{}

// Scheme 返回属性源的标识
func (p *configMapPropertySource) Scheme() string {
	return "k8s"
}

// Load 加载属性文件，profile 配置文件剖面，fileLocation 配置文件名称。
func (p *configMapPropertySource) Load(fileLocation string, profile string) map[string]interface{} {

	v := viper.New()
	v.SetConfigFile(fileLocation)

	err := v.ReadInConfig()
	util.Panic(err).When(err != nil)

	d := v.Sub("data")
	if d == nil {
		return nil
	}

	profileFileName := "application"
	if profile != "" {
		profileFileName += "-" + profile
	}

	result := make(map[string]interface{})

	// 从预定义的文件格式中加载属性值列表
	for _, reader := range conf.Readers {
		for _, ext := range reader.FileExt() {

			key := profileFileName + ext
			if !d.IsSet(key) {
				continue
			}

			log.Infof("load properties from config-map %s:%s", fileLocation, key)
			if val := d.GetString(key); val != "" {
				err = reader.ReadBuffer([]byte(val), result)
				util.Panic(err).When(err != nil)
			}
		}
	}

	return result
}
