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
	"github.com/go-spring/spring-core/arg"
	"github.com/go-spring/spring-core/bean"
	"github.com/go-spring/spring-core/conf"
)

// PandoraBox 请谨慎使用该接口提供的方法。
type PandoraBox interface {
	Prop(key string, opts ...conf.GetOption) interface{}
	Get(i interface{}, opts ...GetOption) error
	Find(selector bean.Selector) ([]bean.Definition, error)
	Collect(i interface{}, selectors ...bean.Selector) error
	Bind(i interface{}, opts ...conf.BindOption) error
	Wire(objOrCtor interface{}, ctorArgs ...arg.Arg) (interface{}, error)
	Go(fn interface{}, args ...arg.Arg)
	Invoke(fn interface{}, args ...arg.Arg) ([]interface{}, error)
}

type pandora struct {
	c *Container
}

func (ctx *pandora) Prop(key string, opts ...conf.GetOption) interface{} {
	return ctx.c.prop(key, opts...)
}

func (ctx *pandora) Get(i interface{}, opts ...GetOption) error {
	return ctx.c.get(i, opts...)
}

func (ctx *pandora) Find(selector bean.Selector) ([]bean.Definition, error) {
	return ctx.c.find(selector)
}

func (ctx *pandora) Collect(i interface{}, selectors ...bean.Selector) error {
	return ctx.c.collect(i, selectors...)
}

func (ctx *pandora) Bind(i interface{}, opts ...conf.BindOption) error {
	return ctx.c.bind(i, opts...)
}

func (ctx *pandora) Wire(objOrCtor interface{}, ctorArgs ...arg.Arg) (interface{}, error) {
	return ctx.c.wire(objOrCtor, ctorArgs...)
}

func (ctx *pandora) Go(fn interface{}, args ...arg.Arg) {
	ctx.c.goroutine(fn, args...)
}

func (ctx *pandora) Invoke(fn interface{}, args ...arg.Arg) ([]interface{}, error) {
	return ctx.c.invoke(fn, args...)
}
