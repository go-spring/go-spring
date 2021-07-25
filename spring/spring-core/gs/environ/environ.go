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

// IncludeEnvPatterns 只加载符合条件的环境变量。
const IncludeEnvPatterns = "INCLUDE_ENV_PATTERNS"

// ExcludeEnvPatterns 排除符合条件的环境变量。
const ExcludeEnvPatterns = "EXCLUDE_ENV_PATTERNS"

// EnablePandora 是否允许 gs.Pandora 接口。
const EnablePandora = "enable-pandora"

// SpringPidFile 保存进程 ID 的文件。
const SpringPidFile = "spring.pid.file"

// SpringConfigLocations 配置文件的位置，支持逗号分隔。
const SpringConfigLocations = "spring.config.locations"

// SpringConfigExtensions 配置文件的扩展名，支持逗号分隔。
const SpringConfigExtensions = "spring.config.extensions"

// SpringBannerVisible 是否显示 banner。
const SpringBannerVisible = "spring.banner.visible"

// SpringProfilesActive 当前应用的 profile 配置。
const SpringProfilesActive = "spring.profiles.active"

// SpringApplicationName 当前应用的名称。
const SpringApplicationName = "spring.application.name"
