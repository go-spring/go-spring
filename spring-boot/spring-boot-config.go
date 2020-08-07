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
	"os"
	"path/filepath"

	"github.com/go-spring/go-spring-parent/spring-logger"
	"github.com/go-spring/go-spring-parent/spring-utils"
	"github.com/spf13/viper"
)

// propertySource 属性源
type propertySource interface {
	// Name 返回属性源的名称
	Name() string

	// Load 加载属性文件，profile 配置文件剖面。
	Load(profile string) map[string]interface{}
}

// defaultPropertySource 基于默认配置文件的属性源
type defaultPropertySource struct {
	fileLocation string // 配置文件所在目录
}

// NewDefaultPropertySource defaultPropertySource 的构造函数
func NewDefaultPropertySource(fileLocation string) *defaultPropertySource {
	return &defaultPropertySource{
		fileLocation: fileLocation,
	}
}

// Name 返回属性源的名称
func (p *defaultPropertySource) Name() string {
	return ""
}

// Load 加载属性文件，profile 配置文件剖面。
func (p *defaultPropertySource) Load(profile string) map[string]interface{} {

	fileNamePrefix := "application"
	if profile != "" {
		fileNamePrefix += "-" + profile
	}

	result := make(map[string]interface{})

	// 从预定义的文件格式中加载属性值列表
	for _, reader := range configReaders {

		filename := filepath.Join(p.fileLocation, fileNamePrefix+reader.FileExt())
		if _, err := os.Stat(filename); err != nil {
			continue // 这里不需要警告
		}

		SpringLogger.Info("load properties from file ", filename)
		reader.ReadFile(filename, result)
	}

	return result
}

// configMapPropertySource 基于 k8s ConfigMap 的属性源
type configMapPropertySource struct {
	filename string // 配置文件名称
}

// NewConfigMapPropertySource configMapPropertySource 的构造函数
func NewConfigMapPropertySource(filename string) *configMapPropertySource {
	return &configMapPropertySource{
		filename: filename,
	}
}

// Name 返回属性源的名称
func (p *configMapPropertySource) Name() string {
	return "k8s"
}

// Load 加载属性文件，profile 配置文件剖面。
func (p *configMapPropertySource) Load(profile string) map[string]interface{} {

	v := viper.New()
	v.SetConfigFile(p.filename)

	err := v.ReadInConfig()
	SpringUtils.Panic(err).When(err != nil)

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
	for _, reader := range configReaders {
		if key := profileFileName + reader.FileExt(); d.IsSet(key) {
			SpringLogger.Infof("load properties from config-map %s:%s", p.filename, key)

			if val := d.GetString(key); val != "" {
				reader.ReadBuffer([]byte(val), result)
			}
		}
	}

	return result
}
