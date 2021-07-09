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

// Package environ 提供了 go-spring 所有内置属性的定义，以及一种允许使用环境变量
// 进行属性覆盖的机制。
package environ

const (
	Version = "go-spring@v1.0.5"
	Website = "https://go-spring.com/"
)

// propNames go-spring 提供了一种机制，允许通过环境变量覆盖命令行或者配置文件中的
// 配置项，但是必须通过 NewProp 进行声明(注册)。假设我们通过 NewProp 定义了一个名
// 为 example.name 的属性，那么可以通过环境变量 EXAMPLE_NAME 进行配置覆盖。
var propNames = make(map[string]struct{})

// NewProp 注册可以通过环境变量进行覆盖的属性。
func NewProp(name string) {
	propNames[name] = struct{}{}
}

// ValidProp 返回是否允许通过环境变量进行覆盖。
func ValidProp(name string) bool {
	_, ok := propNames[name]
	return ok
}

// EnablePandora 是否允许 gs.Pandora 接口，不允许环境覆盖。
const EnablePandora = "enable-pandora"

// SpringActiveProfile 当前应用的 profile 配置，允许环境覆盖。
const SpringActiveProfile = "spring.active.profile"

// SpringApplicationName 当前应用的名称，不允许环境覆盖。
const SpringApplicationName = "spring.application.name"

// SpringBannerVisible 是否显示 banner，允许环境覆盖。
const SpringBannerVisible = "spring.banner.visible"

// SpringConfigLocation 配置文件的位置，允许环境覆盖。
const SpringConfigLocation = "spring.config.location"

// SpringPidFile 保存进程 ID 的文件，不允许环境覆盖。
const SpringPidFile = "spring.pid.file"

func init() {
	NewProp(SpringActiveProfile)
	NewProp(SpringBannerVisible)
	NewProp(SpringConfigLocation)
}
