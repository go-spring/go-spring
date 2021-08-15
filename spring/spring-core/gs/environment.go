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

package gs

import (
	"os"
	"regexp"
	"strings"

	"github.com/go-spring/spring-boost/cast"
	"github.com/go-spring/spring-core/conf"
)

// EnvPrefix 属性覆盖的环境变量需要携带该前缀。
const EnvPrefix = "GS_"

// IncludeEnvPatterns 只加载符合条件的环境变量。
const IncludeEnvPatterns = "INCLUDE_ENV_PATTERNS"

// ExcludeEnvPatterns 排除符合条件的环境变量。
const ExcludeEnvPatterns = "EXCLUDE_ENV_PATTERNS"

// SpringProfilesActive 当前应用的 profile 配置。
const SpringProfilesActive = "spring.profiles.active"

// SpringConfigLocations 配置文件的位置，支持逗号分隔。
const SpringConfigLocations = "spring.config.locations"

// SpringConfigExtensions 配置文件的扩展名，支持逗号分隔。
const SpringConfigExtensions = "spring.config.extensions"

// Environment 提供获取环境变量和命令行参数的方法，命令行参数优先级更高。
type Environment interface {
	ActiveProfile() string
	ConfigLocations() []string
	ConfigExtensions() []string
	Get(key string, opts ...conf.GetOption) interface{}
}

type environment struct {
	p *conf.Properties

	activeProfile    string
	configLocations  []string
	configExtensions []string
}

func (e *environment) ActiveProfile() string {
	return e.activeProfile
}

func (e *environment) ConfigLocations() []string {
	return e.configLocations
}

func (e *environment) ConfigExtensions() []string {
	return e.configExtensions
}

func (e *environment) Get(key string, opts ...conf.GetOption) interface{} {
	return e.p.Get(key, opts...)
}

// loadCmdArgs 加载 -name value 形式的命令行参数。
func loadCmdArgs(p *conf.Properties) error {
	for i := 0; i < len(os.Args); i++ {

		s := os.Args[i]
		if !strings.HasPrefix(s, "-") {
			continue
		}

		k, v := s[1:], ""
		if i >= len(os.Args)-1 {
			p.Set(k, v)
			break
		}

		if !strings.HasPrefix(os.Args[i+1], "-") {
			v = os.Args[i+1]
			i++
		}
		p.Set(k, v)
	}
	return nil
}

// loadSystemEnv 添加符合 includes 条件的环境变量，排除符合 excludes 条件的
// 环境变量。如果发现存在允许通过环境变量覆盖的属性名，那么保存时转换成真正的属性名。
func loadSystemEnv(p *conf.Properties) error {

	toRex := func(patterns []string) ([]*regexp.Regexp, error) {
		var rex []*regexp.Regexp
		for _, v := range patterns {
			exp, err := regexp.Compile(v)
			if err != nil {
				return nil, err
			}
			rex = append(rex, exp)
		}
		return rex, nil
	}

	includes := []string{".*"}
	if s, ok := os.LookupEnv(IncludeEnvPatterns); ok {
		includes = strings.Split(s, ",")
	}
	includeRex, err := toRex(includes)
	if err != nil {
		return err
	}

	var excludes []string
	if s, ok := os.LookupEnv(ExcludeEnvPatterns); ok {
		excludes = strings.Split(s, ",")
	}
	excludeRex, err := toRex(excludes)
	if err != nil {
		return err
	}

	matches := func(rex []*regexp.Regexp, s string) bool {
		for _, r := range rex {
			if r.MatchString(s) {
				return true
			}
		}
		return false
	}

	for _, env := range os.Environ() {

		kv := strings.SplitN(env, "=", 2)
		if len(kv) == 1 {
			continue
		}

		k, v := kv[0], kv[1]
		if k == "" || v == "" {
			continue
		}

		if strings.HasPrefix(k, EnvPrefix) {
			propKey := strings.TrimPrefix(k, EnvPrefix)
			propKey = strings.ReplaceAll(propKey, "_", ".")
			p.Set(strings.ToLower(propKey), v)
			continue
		}

		if matches(excludeRex, k) || !matches(includeRex, k) {
			continue
		}
		p.Set(k, v)
	}
	return nil
}

func (e *environment) prepare() error {

	if err := loadSystemEnv(e.p); err != nil {
		return err
	}

	if err := loadCmdArgs(e.p); err != nil {
		return err
	}

	s := e.p.Get(SpringConfigLocations, conf.Def("config/"))
	e.configLocations = strings.Split(cast.ToString(s), ",")

	extensions := ".properties,.prop,.yaml,.yml,.toml,.tml"
	s = e.p.Get(SpringConfigExtensions, conf.Def(extensions))
	e.configExtensions = strings.Split(cast.ToString(s), ",")

	e.activeProfile = cast.ToString(e.p.Get(SpringProfilesActive))
	return nil
}
