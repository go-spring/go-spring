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

	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/gs/arg"
)

type tempBootstrap struct {
	resourceLocators []ResourceLocator `autowire:""`
}

type bootstrap struct {
	*tempBootstrap
	c *container
}

func newBootstrap() *bootstrap {
	return &bootstrap{
		c: New().(*container),
	}
}

func (b *bootstrap) clear() {
	b.tempBootstrap = nil
}

// OnProperty 参考 App.OnProperty 的解释。
func (b *bootstrap) OnProperty(key string, fn interface{}) {
	b.c.OnProperty(key, fn)
}

// Property 参考 Container.Property 的解释。
func (b *bootstrap) Property(key string, value interface{}) {
	b.c.Property(key, value)
}

// Object 参考 Container.Object 的解释。
func (b *bootstrap) Object(i interface{}) *BeanDefinition {
	return b.c.Accept(NewBean(reflect.ValueOf(i)))
}

// Provide 参考 Container.Provide 的解释。
func (b *bootstrap) Provide(ctor interface{}, args ...arg.Arg) *BeanDefinition {
	return b.c.Accept(NewBean(ctor, args...))
}

// ResourceLocator 参考 Container.Object 的解释。
func (b *bootstrap) ResourceLocator(i interface{}) *BeanDefinition {
	return b.c.Accept(NewBean(reflect.ValueOf(i))).Export((*ResourceLocator)(nil))
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
