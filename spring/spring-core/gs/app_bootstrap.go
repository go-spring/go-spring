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
	"reflect"

	"github.com/go-spring/spring-boost/errors"
	"github.com/go-spring/spring-boost/util"
	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/gs/arg"
)

type bootstrap struct {

	// 应用上下文
	c *container

	// 属性列表解析完成后的回调
	mapOfOnProperty  map[string]interface{}
	resourceLocators []ResourceLocator `autowire:""`
}

func newBootstrap() *bootstrap {
	return &bootstrap{
		c:               New().(*container),
		mapOfOnProperty: make(map[string]interface{}),
	}
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

// OnProperty 参考 App.OnProperty 的解释。
func (boot *bootstrap) OnProperty(key string, fn interface{}) {
	err := validOnProperty(fn)
	util.Panic(err).When(err != nil)
	boot.mapOfOnProperty[key] = fn
}

// Property 参考 Container.Property 的解释。
func (boot *bootstrap) Property(key string, value interface{}) {
	boot.c.Property(key, value)
}

// Object 参考 Container.Object 的解释。
func (boot *bootstrap) Object(i interface{}) *BeanDefinition {
	return boot.c.register(NewBean(reflect.ValueOf(i)))
}

// Provide 参考 Container.Provide 的解释。
func (boot *bootstrap) Provide(ctor interface{}, args ...arg.Arg) *BeanDefinition {
	return boot.c.register(NewBean(ctor, args...))
}

func (boot *bootstrap) start(e *configuration) error {

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

func (boot *bootstrap) loadBootstrap(e *configuration) error {
	if err := boot.loadConfigFile(e, "bootstrap"); err != nil {
		return err
	}
	for _, profile := range e.ActiveProfiles {
		if err := boot.loadConfigFile(e, "bootstrap-"+profile); err != nil {
			return err
		}
	}
	return nil
}

func (boot *bootstrap) loadConfigFile(e *configuration, filename string) error {
	for _, ext := range e.ConfigExtensions {
		resources, err := e.resourceLocator.Locate(filename + ext)
		if err != nil {
			return err
		}
		p := conf.New()
		for _, file := range resources {
			if err = p.Load(file.Name()); err != nil {
				return err
			}
		}
		for _, key := range p.Keys() {
			boot.c.p.Set(key, p.Get(key))
		}
	}
	return nil
}
