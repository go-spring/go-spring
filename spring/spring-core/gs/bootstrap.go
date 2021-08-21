/*
 * Copyright 2012-2019 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by bootlicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package gs

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-spring/spring-boost/cast"
	"github.com/go-spring/spring-boost/util"
	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/gs/arg"
	"github.com/go-spring/spring-core/gs/cond"
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

func (e *environment) Properties() cond.Properties {
	return e.p
}

type Environment interface {
	ActiveProfile() string
	ConfigLocations() []string
	ConfigExtensions() []string
	Properties() cond.Properties
}

type PropertySource interface {
	Load(e Environment) (map[string]*conf.Properties, error)
}

type bootstrap struct {

	// 应用上下文
	c *Container

	// 属性列表解析完成后的回调
	mapOfOnProperty map[string]interface{}

	PropertySources []PropertySource `autowire:""`
}

func validOnProperty(fn interface{}) error {
	t := reflect.TypeOf(fn)
	if t.Kind() != reflect.Func {
		return errors.New("fn should be a func(value_type)")
	}
	if t.NumIn() != 1 || !util.IsValueType(t.In(0)) || t.NumOut() != 0 {
		return errors.New("fn should be a func(value_type)")
	}
	return nil
}

// OnProperty 当 key 对应的属性值准备好后发送一个通知。
func (boot *bootstrap) OnProperty(key string, fn interface{}) {
	err := validOnProperty(fn)
	util.Panic(err).When(err != nil)
	boot.mapOfOnProperty[key] = fn
}

// Property 设置 key 对应的属性值，如果 key 对应的属性值已经存在则 Set 方法会
// 覆盖旧值。Set 方法除了支持 string 类型的属性值，还支持 int、uint、bool 等
// 其他基础数据类型的属性值。特殊情况下，Set 方法也支持 slice 、map 与基础数据
// 类型组合构成的属性值，其处理方式是将组合结构层层展开，可以将组合结构看成一棵树，
// 那么叶子结点的路径就是属性的 key，叶子结点的值就是属性的值。
func (boot *bootstrap) Property(key string, value interface{}) {
	boot.c.Property(key, value)
}

// Object 注册对象形式的 bean ，需要注意的是该方法在注入开始后就不能再调用了。
func (boot *bootstrap) Object(i interface{}) *BeanDefinition {
	return boot.c.register(NewBean(reflect.ValueOf(i)))
}

// Provide 注册构造函数形式的 bean ，需要注意的是该方法在注入开始后就不能再调用了。
func (boot *bootstrap) Provide(ctor interface{}, args ...arg.Arg) *BeanDefinition {
	return boot.c.register(NewBean(ctor, args...))
}

func (boot *bootstrap) start(e *environment) error {

	boot.c.Object(boot)

	if err := boot.loadBootstrap(e); err != nil {
		return err
	}

	// 保存从环境变量和命令行解析的属性
	for _, k := range e.p.Keys() {
		boot.c.p.Set(k, e.p.Get(k))
	}

	for key, f := range boot.mapOfOnProperty {
		t := reflect.TypeOf(f)
		in := reflect.New(t.In(0)).Elem()
		err := boot.c.p.Bind(in, conf.Key(key))
		if err != nil {
			return err
		}
		reflect.ValueOf(f).Call([]reflect.Value{in})
	}

	return boot.c.Refresh()
}

func (boot *bootstrap) loadBootstrap(e *environment) error {
	if err := boot.loadConfigFile(e, "bootstrap"); err != nil {
		return err
	}
	if e.activeProfile == "" {
		return nil
	}
	return boot.loadConfigFile(e, "bootstrap-"+e.activeProfile)
}

func (boot *bootstrap) loadConfigFile(e *environment, filename string) error {
	for _, loc := range e.configLocations {
		for _, ext := range e.configExtensions {
			err := boot.c.Load(filepath.Join(loc, filename+ext))
			if err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

func (boot *bootstrap) sourceMap(e *environment) (map[string][]*conf.Properties, error) {
	sourceMap := make(map[string][]*conf.Properties)
	for _, ps := range boot.PropertySources {
		m, err := ps.Load(e)
		if err != nil {
			return nil, err
		}
		for k, p := range m {
			sourceMap[k] = append(sourceMap[k], p)
		}
	}
	return sourceMap, nil
}
