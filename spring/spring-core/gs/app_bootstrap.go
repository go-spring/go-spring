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
	"errors"
	"reflect"

	"github.com/go-spring/spring-boost/conf"
	"github.com/go-spring/spring-boost/util"
	"github.com/go-spring/spring-core/gs/arg"
)

type tempBootstrap struct {
	mapOfOnProperty  map[string]interface{}
	resourceLocators []ResourceLocator `autowire:""`
}

type bootstrap struct {
	*tempBootstrap
	c *container
}

func newBootstrap() *bootstrap {
	return &bootstrap{
		c: New().(*container),
		tempBootstrap: &tempBootstrap{
			mapOfOnProperty: make(map[string]interface{}),
		},
	}
}

func (b *bootstrap) clear() {
	b.tempBootstrap = nil
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
func (b *bootstrap) OnProperty(key string, fn interface{}) {
	err := validOnProperty(fn)
	util.Panic(err).When(err != nil)
	b.mapOfOnProperty[key] = fn
}

// Property 参考 Container.Property 的解释。
func (b *bootstrap) Property(key string, value interface{}) {
	b.c.Property(key, value)
}

// Object 参考 Container.Object 的解释。
func (b *bootstrap) Object(i interface{}) *BeanDefinition {
	return b.c.register(NewBean(reflect.ValueOf(i)))
}

// Provide 参考 Container.Provide 的解释。
func (b *bootstrap) Provide(ctor interface{}, args ...arg.Arg) *BeanDefinition {
	return b.c.register(NewBean(ctor, args...))
}

// ResourceLocator 参考 Container.Object 的解释。
func (b *bootstrap) ResourceLocator(i interface{}) *BeanDefinition {
	return b.c.register(NewBean(reflect.ValueOf(i))).Export((*ResourceLocator)(nil))
}

func (b *bootstrap) start(e *configuration) error {

	b.c.Object(b)

	if err := b.loadBootstrap(e); err != nil {
		return err
	}

	// 保存从环境变量和命令行解析的属性
	for _, k := range e.p.Keys() {
		b.c.p.Set(k, e.p.Get(k))
	}

	for key, f := range b.mapOfOnProperty {
		t := reflect.TypeOf(f)
		in := reflect.New(t.In(0)).Elem()
		err := b.c.p.Bind(in, conf.Key(key))
		if err != nil {
			return err
		}
		reflect.ValueOf(f).Call([]reflect.Value{in})
	}

	return b.c.Refresh()
}

func (b *bootstrap) loadBootstrap(e *configuration) error {
	if err := b.loadConfigFile(e, "bootstrap"); err != nil {
		return err
	}
	for _, profile := range e.ActiveProfiles {
		if err := b.loadConfigFile(e, "bootstrap-"+profile); err != nil {
			return err
		}
	}
	return nil
}

func (b *bootstrap) loadConfigFile(e *configuration, filename string) error {
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
			b.c.p.Set(key, p.Get(key))
		}
	}
	return nil
}
