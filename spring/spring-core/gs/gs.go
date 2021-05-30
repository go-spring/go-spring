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

// Package gs 实现了 go-spring 框架的基础骨架，包含 IoC 容器、基于 IoC 容器
// 的 App 以及全局 App 对象封装三个部分。
package gs

import (
	"bytes"
	"container/list"
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"sync"

	"github.com/go-spring/spring-core/arg"
	"github.com/go-spring/spring-core/bean"
	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/log"
	"github.com/go-spring/spring-core/sort"
	"github.com/go-spring/spring-core/util"
)

type refreshState int

const (
	Unrefreshed = refreshState(0) // 未刷新
	Refreshing  = refreshState(1) // 正在刷新
	Refreshed   = refreshState(2) // 已刷新
)

// Container 是 go-spring 框架的基石，实现了 Martin Fowler 在 << Inversion
// of Control Containers and the Dependency Injection pattern >> 一文滥
// 觞的依赖注入的概念。原文的依赖注入仅仅是指对象(在 Java 中是指结构体实例)之间的
// 依赖关系处理，而有些 IoC 容器在实现时比如 Spring 还引入了对属性 property 的处
// 理，通常大家会用依赖注入统述上面两种概念，但实际上使用属性绑定来描述对 property
// 的处理更合适，因此 go-spring 在描述对 bean 的处理时要么单独使用依赖注入或属性
// 绑定，要么同时使用依赖注入和属性绑定。
type Container struct {
	p *conf.Properties

	state refreshState

	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	beans       []*BeanDefinition
	beansById   map[string]*BeanDefinition
	beansByName map[string][]*BeanDefinition
	beansByType map[reflect.Type][]*BeanDefinition

	configers    []*Configer
	configerList *list.List

	destroyerList *list.List
}

type newArg struct {
	openPandora bool
}

type NewOption func(arg *newArg)

// OpenPandora 使用此选项时 Container 会注册一个 Pandora 实例。
func OpenPandora() NewOption {
	return func(arg *newArg) {
		arg.openPandora = true
	}
}

// New 创建 IoC 容器，使用 OpenPandora 选项可以注册一个 Pandora 实例。
func New(opts ...NewOption) *Container {

	a := newArg{}
	for _, opt := range opts {
		opt(&a)
	}

	ctx, cancel := context.WithCancel(context.Background())
	c := &Container{
		p:             conf.New(),
		ctx:           ctx,
		cancel:        cancel,
		beansById:     make(map[string]*BeanDefinition),
		beansByName:   make(map[string][]*BeanDefinition),
		beansByType:   make(map[reflect.Type][]*BeanDefinition),
		configerList:  list.New(),
		destroyerList: list.New(),
	}

	if a.openPandora {
		c.Object(&pandora{c}).Export((*Pandora)(nil))
	}
	return c
}

// callBeforeRefreshing 有些方法只能在 Refresh 开始前调用，比如 Object 。
func (c *Container) callBeforeRefreshing() {
	if c.state != Unrefreshed {
		panic(errors.New("should call before Refreshing"))
	}
}

// callAfterRefreshing 有些方法必须在 Refresh 开始后才能调用，比如 Wire 。
func (c *Container) callAfterRefreshing() {
	if c.state == Unrefreshed {
		panic(errors.New("should call after Refreshing"))
	}
}

// Load 从文件读取属性列表。
func (c *Container) Load(filename string) error {
	c.callBeforeRefreshing()
	return c.p.Load(filename)
}

// Property 设置 key 对应的属性值。
func (c *Container) Property(key string, value interface{}) {
	c.callBeforeRefreshing()
	c.p.Set(key, value)
}

// Object 注册对象形式的 bean ，包括接口、指针、channel、function 及其集合。
func (c *Container) Object(i interface{}) *BeanDefinition {
	return c.register(NewBean(reflect.ValueOf(i)))
}

// Provide 注册构造函数形式的 bean ，这里的构造函数泛指所有可以返回 bean 对象
// 的函数或方法( golang 里面方法是指带有接收者的函数)，当使用方法定义的构造函数
// 时，接收者被当做函数的第一个参数。另外，ctor 只能返回一个 bean 对象，当然也
// 支持返回一个额外的 error 对象。
func (c *Container) Provide(ctor interface{}, args ...arg.Arg) *BeanDefinition {
	return c.register(NewBean(ctor, args...))
}

func (c *Container) register(b *BeanDefinition) *BeanDefinition {
	c.callBeforeRefreshing()
	c.beans = append(c.beans, b)
	return b
}

// Config 注册配置函数，配置函数是指可以接受一些 bean 作为入参且没有返回值的函
// 数，使用场景大多是在 bean 初始化之后对 bean 进行二次配置，该机制可以作为框架
// 配置能力的有效补充，但是一定要慎用！
func (c *Container) Config(fn interface{}, args ...arg.Arg) *Configer {
	return c.config(NewConfiger(fn, args...))
}

func (c *Container) config(configer *Configer) *Configer {
	c.callBeforeRefreshing()
	c.configers = append(c.configers, configer)
	return configer
}

// find 查找符合条件的单例 bean，考虑到该方法可能的使用场景，因此在找不到符合条件
// 的 bean 时返回 nil，在找到多于 1 个符合条件的 bean 时返回 error，而且该函数
// 只保证返回的 bean 是有效(未被标记为删除)的，而不保证已经完成属性绑定和依赖注入。
func (c *Container) find(selector bean.Selector) (*BeanDefinition, error) {
	c.callAfterRefreshing()

	finder := func(fn func(*BeanDefinition) bool) (*BeanDefinition, error) {
		var result []*BeanDefinition

		for _, b := range c.beansById {
			if b.status == Resolving || !fn(b) {
				continue
			}
			if err := c.resolveBean(b); err != nil {
				return nil, err
			}
			if b.status == Deleted {
				continue
			}
			result = append(result, b)
		}

		if result == nil {
			return nil, nil
		}

		if n := len(result); n > 1 {
			buf := bytes.Buffer{}
			buf.WriteString(fmt.Sprintf("found %d beans", n))
			for _, d := range result {
				buf.WriteString(fmt.Sprintf(" %q", d))
			}
			return nil, errors.New(buf.String())
		}

		return result[0], nil
	}

	t := reflect.TypeOf(selector)

	if t.Kind() == reflect.String {
		tag := parseSingletonTag(selector.(string))
		return finder(func(b *BeanDefinition) bool {
			return b.Match(tag.typeName, tag.beanName)
		})
	}

	if t.Kind() == reflect.Ptr {
		if e := t.Elem(); e.Kind() == reflect.Interface {
			t = e // 指 (*error)(nil) 形式的 bean 选择器
		}
	}

	return finder(func(b *BeanDefinition) bool {
		if !b.Type().AssignableTo(t) {
			return false
		}
		if t.Kind() != reflect.Interface {
			return true
		}
		_, ok := b.exports[t]
		return ok
	})
}

// Refresh 决议和组装所有的 configer 与 bean 。
func (c *Container) Refresh() {

	if c.state != Unrefreshed {
		panic(errors.New("container already refreshed"))
	}

	c.state = Refreshing

	err := c.registerBeans()
	util.Panic(err).When(err != nil)

	err = c.resolveConfigers()
	util.Panic(err).When(err != nil)

	err = c.resolveBeans()
	util.Panic(err).When(err != nil)

	assembly := toAssembly(c)

	defer func() {
		if len(assembly.stack) > 0 {
			log.Infof("wiring path %s", assembly.stack.path())
		}
	}()

	err = c.runConfigers(assembly)
	util.Panic(err).When(err != nil)

	err = c.wireBeans(assembly)
	util.Panic(err).When(err != nil)

	c.destroyerList = assembly.sortDestroyers()
	c.state = Refreshed

	log.Info("container refreshed successfully")
}

func (c *Container) registerBeans() error {
	for _, b := range c.beans {
		if d, ok := c.beansById[b.ID()]; ok {
			return fmt.Errorf("found duplicate beans [%s] [%s]", b, d)
		}
		c.beansById[b.ID()] = b
	}
	return nil
}

func (c *Container) resolveConfigers() error {
	for _, g := range c.configers {
		if g.cond != nil {
			if ok, err := g.cond.Matches(&pandora{c}); err != nil {
				return err
			} else if !ok {
				continue
			}
		}
		c.configerList.PushBack(g)
	}
	c.configerList = sort.Triple(c.configerList, getBeforeConfigers)
	return nil
}

func (c *Container) resolveBeans() error {
	for _, b := range c.beansById {
		if err := c.resolveBean(b); err != nil {
			return err
		}
	}
	return nil
}

// resolveBean 决议 bean 是否是有效的，如果 bean 是无效的则呗标记为已删除。
func (c *Container) resolveBean(b *BeanDefinition) error {

	if b.status >= Resolving {
		return nil
	}

	b.status = Resolving

	if b.cond != nil {
		if ok, err := b.cond.Matches(&pandora{c}); err != nil {
			return err
		} else if !ok {
			delete(c.beansById, b.ID())
			b.status = Deleted
			return nil
		}
	}

	log.Debugf("register %s name:%q type:%q %s", b.getClass(), b.Name(), b.Type(), b.FileLine())
	c.beansByType[b.Type()] = append(c.beansByType[b.Type()], b)
	c.beansByName[b.name] = append(c.beansByName[b.name], b)

	if err := c.export(b.Type(), b); err != nil {
		return err
	}

	for t := range b.exports {
		if !b.Type().Implements(t) {
			return fmt.Errorf("%s doesn't implement %s interface", b, t)
		}
		log.Debugf("register %s name:%q type:%q %s", b.getClass(), b.Name(), t, b.FileLine())
		c.beansByType[t] = append(c.beansByType[t], b)
	}

	b.status = Resolved
	return nil
}

// export 导出结构体指针类型 bean 使用 export 语法导出的接口。
func (c *Container) export(t reflect.Type, b *BeanDefinition) error {

	t = util.Indirect(t)
	if t.Kind() != reflect.Struct {
		return nil
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)

		_, export := f.Tag.Lookup("export")
		_, inject := f.Tag.Lookup("inject")
		_, autowire := f.Tag.Lookup("autowire")

		if !export {
			if f.Anonymous && !inject && !autowire {
				if err := c.export(f.Type, b); err != nil {
					return err
				}
			}
			continue
		}

		// 有 export 标签的必须是接口类型。
		if f.Type.Kind() != reflect.Interface {
			return errors.New("export can only use on interface")
		}

		// 不能导出需要注入的接口。
		if inject || autowire {
			return errors.New("inject or autowire can't use with export")
		}

		if err := b.export(f.Type); err != nil {
			return err
		}
	}
	return nil
}

func (c *Container) runConfigers(assembly *beanAssembly) error {
	for e := c.configerList.Front(); e != nil; e = e.Next() {
		g := e.Value.(*Configer)
		if _, err := g.fn.Call(assembly); err != nil {
			return err
		}
	}
	return nil
}

func (c *Container) wireBeans(assembly *beanAssembly) error {
	for _, b := range c.beansById {
		if err := assembly.wireBean(b); err != nil {
			return err
		}
	}
	return nil
}

// Close 关闭容器，此方法必须在 Refresh 之后调用。该方法会触发 ctx 的 Done 信
// 号，然后等待所有 goroutine 结束，最后按照被依赖先销毁的原则执行所有的销毁函数。
func (c *Container) Close() {
	c.callAfterRefreshing()

	c.cancel()
	c.wg.Wait()

	log.Info("goroutines exited")

	assembly := toAssembly(c)
	c.runDestroyers(assembly)

	log.Info("container closed")
}

func (c *Container) runDestroyers(assembly *beanAssembly) {
	for e := c.destroyerList.Front(); e != nil; e = e.Next() {
		d := e.Value.(*destroyer)
		if err := d.run(assembly); err != nil {
			log.Error(err)
		}
	}
}

// Go 创建安全可等待的 goroutine，fn 要求的 ctx 对象由 Container 提供，当
// Container 关闭时 ctx 发出 Done 信号， fn 在接收到此信号后应当立即退出。
func (c *Container) Go(fn func(ctx context.Context)) {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		defer func() {
			if r := recover(); r != nil {
				log.Errorf("%v, %s", r, debug.Stack())
			}
		}()

		fn(c.ctx)
	}()
}
