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

// Package gs 实现了一个功能完善的运行时 IoC 容器。
package gs

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/go-spring/spring-core/arg"
	"github.com/go-spring/spring-core/bean"
	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/log"
	"github.com/go-spring/spring-core/sort"
	"github.com/go-spring/spring-core/util"
)

type GetBeanArg struct {
	Selector bean.Selector
}

type GetBeanOption func(arg *GetBeanArg)

func Use(s bean.Selector) GetBeanOption {
	return func(arg *GetBeanArg) {
		arg.Selector = s
	}
}

// Context 定义了 IoC 容器接口。
//
// 它的工作过程可以分为三个大的阶段：注册 Bean 列表、加载属性配置
// 文件、自动绑定。其中自动绑定又分为两个小阶段：解析（决议）和绑定。
//
// 一条需要谨记的注册规则是: AutoWireBeans 调用后就不能再注册新
// 的 Bean 了，这样做是因为实现起来更简单而且性能更高。
type Context interface {

	// Context 返回上下文接口
	Context() context.Context

	// Profile 返回运行环境。
	Profile() string

	// Properties 返回所有属性。
	Properties() map[string]interface{}

	// GetProperty 返回 key 转为小写后精确匹配的属性值，不存在返回 nil。
	GetProperty(key string, opts ...conf.GetOption) interface{}

	// GetBean 获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
	// 它和 FindBean 的区别是它在调用后能够保证返回的 Bean 已经完成了注入和绑定过程。
	GetBean(i interface{}, opts ...GetBeanOption) error

	// FindBean 返回符合条件的 Bean 集合，不保证返回的 Bean 已经完成注入和绑定过程。
	FindBean(selector bean.Selector) ([]bean.Definition, error)

	// CollectBeans 收集数组或指针定义的所有符合条件的 Bean，收集到返回 true，否则返
	// 回 false。该函数有两种模式:自动模式和指定模式。自动模式是指 selectors 参数为空，
	// 这时候不仅会收集符合条件的单例 Bean，还会收集符合条件的数组 Bean (是指数组的元素
	// 符合条件，然后把数组元素拆开一个个放到收集结果里面)。指定模式是指 selectors 参数
	// 不为空，这时候只会收集单例 Bean，而且要求这些单例 Bean 不仅需要满足收集条件，而且
	// 必须满足 selector 条件。另外，自动模式下不对收集结果进行排序，指定模式下根据
	// selectors 列表的顺序对收集结果进行排序。
	CollectBeans(i interface{}, selectors ...bean.Selector) error

	// Bind 绑定结构体属性。
	Bind(i interface{}, opts ...conf.BindOption) error

	// Wire 对对象或者构造函数的结果进行依赖注入和属性绑定，返回处理后的对象
	Wire(objOrCtor interface{}, ctorArgs ...arg.Arg) (interface{}, error)

	// Invoke 立即执行一个一次性的任务
	Invoke(fn interface{}, args ...arg.Arg) error

	// Go 安全地启动一个 goroutine
	Go(fn interface{}, args ...arg.Arg)
}

// ApplicationContext Context 不允许内容被修改，这个可以。
type ApplicationContext interface {

	// Context 不能修改内容的接口
	Context

	// SetProfile 设置运行环境
	SetProfile(profile string)

	// SetProperty 设置属性值，属性名称统一转成小写。
	SetProperty(key string, value interface{})

	// RegisterBean 注册对象形式的 Bean。
	RegisterBean(i interface{}) *BeanDefinition

	// ProvideBean 普通函数注册需要使用 reflect.ValueOf(fn) 这种方式避免和构造函数发生冲突。
	ProvideBean(objOrCtor interface{}, ctorArgs ...arg.Arg) *BeanDefinition

	// Config 注册一个配置函数
	Config(fn interface{}, args ...arg.Arg) *Configer

	// Refresh 对所有 Bean 进行依赖注入和属性绑定
	Refresh()

	// Close 关闭容器上下文，用于通知 Bean 销毁等。
	// 该函数可以确保 Bean 的销毁顺序和注入顺序相反。
	Close()
}

type refreshState int

const (
	Unrefreshed = refreshState(0) // 未刷新
	Refreshing  = refreshState(1) // 正在刷新
	Refreshed   = refreshState(2) // 已刷新
)

// applicationContext ApplicationContext 的默认实现
type applicationContext struct {

	// 属性列表
	p conf.Properties

	// 上下文
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	profile string
	state   refreshState

	allBeans   []*BeanDefinition // 所有注册点
	configers  *list.List        // 配置方法集合
	destroyers *list.List        // 销毁函数集合

	cacheById   map[string]*BeanDefinition
	cacheByName map[string][]*BeanDefinition
	cacheByType map[reflect.Type][]*BeanDefinition
}

// New applicationContext 的构造函数
func New(filename ...string) *applicationContext {

	p := conf.New()
	for _, s := range filename {
		if err := p.Load(s); err != nil {
			panic(err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &applicationContext{
		p:           p,
		ctx:         ctx,
		cancel:      cancel,
		cacheById:   make(map[string]*BeanDefinition),
		cacheByName: make(map[string][]*BeanDefinition),
		cacheByType: make(map[reflect.Type][]*BeanDefinition),
		configers:   list.New(),
		destroyers:  list.New(),
	}
}

// Properties 返回所有属性值的集合，底层调用 conf.Properties 的 Map 方法。
func (c *applicationContext) Properties() map[string]interface{} {
	return c.p.Map()
}

// Bind 对任意值对象进行绑定，底层调用 conf.Properties 的 Bind 方法。
func (c *applicationContext) Bind(i interface{}, opts ...conf.BindOption) error {
	return c.p.Bind(i, opts...)
}

// GetProperty 获取 key 对应的属性值，底层调用 conf.Properties 的 Get 方法。
func (c *applicationContext) GetProperty(key string, opts ...conf.GetOption) interface{} {
	return c.p.Get(key, opts...)
}

// SetProperty 设置 key 对应的属性值，底层调用 conf.Properties 的 Set 方法。
func (c *applicationContext) SetProperty(key string, value interface{}) {
	c.p.Set(key, value)
}

// Context 返回上下文接口
func (c *applicationContext) Context() context.Context {
	return c.ctx
}

// Profile 返回运行环境。
func (c *applicationContext) Profile() string {
	return c.profile
}

// SetProfile 设置运行环境
func (c *applicationContext) SetProfile(profile string) {
	c.profile = profile
}

// callAfterRefreshing 有些方法必须在刷新开始后才能调用，比如 GetBean、Wire、Go 等。
func (c *applicationContext) callAfterRefreshing() {
	if c.state == Unrefreshed {
		panic(errors.New("should call after Refreshing"))
	}
}

// callBeforeRefreshing 有些方法在刷新开始后不能再调用，比如 ProvideBean、Config 等。
func (c *applicationContext) callBeforeRefreshing() {
	if c.state != Unrefreshed {
		panic(errors.New("should call before Refreshing"))
	}
}

// RegisterBean 注册对象形式的 Bean。
func (c *applicationContext) RegisterBean(i interface{}) *BeanDefinition {
	return c.ProvideBean(reflect.ValueOf(i))
}

// ProvideBean 注册构造函数形式的 Bean。
func (c *applicationContext) ProvideBean(objOrCtor interface{}, ctorArgs ...arg.Arg) *BeanDefinition {
	c.callBeforeRefreshing()
	b := NewBean(objOrCtor, ctorArgs...)
	c.allBeans = append(c.allBeans, b)
	return b
}

// Config 注册一个配置函数
func (c *applicationContext) Config(fn interface{}, args ...arg.Arg) *Configer {
	c.callBeforeRefreshing()
	configer := config(fn, args, 1)
	c.configers.PushBack(configer)
	return configer
}

// GetBean 获取单例 Bean，它和 FindBean 的区别是它在调用后能够保证返回的 Bean 已经完成了注入和绑定过程。
func (c *applicationContext) GetBean(i interface{}, opts ...GetBeanOption) error {

	if i == nil {
		return errors.New("i can't be nil")
	}

	c.callAfterRefreshing()

	// 使用指针才能够对外赋值
	if reflect.TypeOf(i).Kind() != reflect.Ptr {
		return errors.New("i must be pointer")
	}

	a := GetBeanArg{Selector: bean.Selector("")}
	for _, opt := range opts {
		opt(&a)
	}

	w := toAssembly(c)
	v := reflect.ValueOf(i).Elem()
	return w.getBean(toSingletonTag(a.Selector), v)
}

// FindBean 返回符合条件的 Bean 集合，不保证返回的 Bean 已经完成注入和绑定过程。
func (c *applicationContext) FindBean(selector bean.Selector) ([]bean.Definition, error) {
	c.callAfterRefreshing()

	finder := func(fn func(*BeanDefinition) bool) (result []bean.Definition, err error) {
		for _, b := range c.cacheById {
			if b.status != Resolving && fn(b) {
				// 避免 Bean 未被解析
				if err = c.resolveBean(b); err != nil {
					return nil, err
				}
				if b.status != Deleted {
					result = append(result, b)
				}
			}
		}
		return
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
			t = e // 接口类型去掉指针
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

// CollectBeans 收集数组或指针定义的所有符合条件的 Bean，收集到返回 true，否则返
// 回 false。该函数有两种模式:自动模式和指定模式。自动模式是指 selectors 参数为空，
// 这时候不仅会收集符合条件的单例 Bean，还会收集符合条件的数组 Bean (是指数组的元素
// 符合条件，然后把数组元素拆开一个个放到收集结果里面)。指定模式是指 selectors 参数
// 不为空，这时候只会收集单例 Bean，而且要求这些单例 Bean 不仅需要满足收集条件，而且
// 必须满足 selector 条件。另外，自动模式下不对收集结果进行排序，指定模式下根据
// selectors 列表的顺序对收集结果进行排序。
func (c *applicationContext) CollectBeans(i interface{}, selectors ...bean.Selector) error {
	c.callAfterRefreshing()

	v := reflect.ValueOf(i)
	if v.Kind() != reflect.Ptr {
		return errors.New("i must be slice ptr")
	}

	var tag collectionTag
	for _, selector := range selectors {
		s := toSingletonTag(selector)
		tag.beanTags = append(tag.beanTags, s)
	}
	return toAssembly(c).collectBeans(tag, v.Elem())
}

// Wire 对对象或者构造函数的结果进行依赖注入和属性绑定，返回处理后的对象
func (c *applicationContext) Wire(objOrCtor interface{}, ctorArgs ...arg.Arg) (interface{}, error) {
	c.callAfterRefreshing()
	assembly := toAssembly(c)
	b := NewBean(objOrCtor, ctorArgs...)
	// 这里使用 wireBean 是为了追踪 bean 的注入路径。
	if err := assembly.wireBean(b); err != nil {
		return nil, err
	}
	return b.Interface(), nil
}

// Invoke 立即执行一个一次性的任务
func (c *applicationContext) Invoke(fn interface{}, args ...arg.Arg) error {
	c.callAfterRefreshing()
	if fnType := reflect.TypeOf(fn); util.IsFuncType(fnType) {
		if util.ReturnNothing(fnType) || util.ReturnOnlyError(fnType) {
			r := arg.Bind(fn, args, arg.Skip(1))
			_, err := r.Call(toAssembly(c))
			return err
		}
	}
	return errors.New("fn should be func() or func()error")
}

// Go 安全地启动一个 goroutine
func (c *applicationContext) Go(fn interface{}, args ...arg.Arg) {
	c.callAfterRefreshing()

	fnType := reflect.TypeOf(fn)
	if !util.IsFuncType(fnType) || !util.ReturnNothing(fnType) {
		panic(errors.New("fn should be func()"))
	}

	r := arg.Bind(fn, args, arg.Skip(1))

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		defer func() {
			if r := recover(); r != nil {
				log.Error(r)
			}
		}()

		_, err := r.Call(toAssembly(c))
		if err != nil {
			log.Error(err.Error())
		}
	}()
}

// Refresh 对所有 Bean 进行依赖注入和属性绑定
func (c *applicationContext) Refresh() {

	if c.state != Unrefreshed {
		panic(errors.New("already refreshed"))
	}

	// 处理 Method Bean 等
	c.registerBeans()

	c.state = Refreshing

	c.resolveConfigers()

	err := c.resolveBeans()
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

	c.destroyers = assembly.sortDestroyers()
	c.state = Refreshed
}

func (c *applicationContext) registerBeans() {
	for i := 0; i < len(c.allBeans); i++ {
		b := c.allBeans[i]
		if _, ok := c.cacheById[b.ID()]; ok {
			panic(fmt.Errorf("duplicate registration, bean:%q", b.ID()))
		}
		c.cacheById[b.ID()] = b
	}
}

// resolveConfigers 对 Config 函数进行决议是否能够保留它
func (c *applicationContext) resolveConfigers() {

	// 对 config 函数进行决议
	for e := c.configers.Front(); e != nil; {
		next := e.Next()
		configer := e.Value.(*Configer)
		if configer.cond != nil && !configer.cond.Matches(c) {
			c.configers.Remove(e)
		}
		e = next
	}

	// 对 config 函数进行排序
	c.configers = sort.Triple(c.configers, getBeforeConfigers)
}

// resolveBeans 对 Bean 进行决议是否能够创建 Bean 的实例
func (c *applicationContext) resolveBeans() error {
	for _, b := range c.cacheById {
		if err := c.resolveBean(b); err != nil {
			return err
		}
	}
	return nil
}

// resolveBean 对 Bean 进行决议是否能够创建 Bean 的实例
func (c *applicationContext) resolveBean(b *BeanDefinition) error {

	// 正在进行或者已经完成决议过程
	if b.status >= Resolving {
		return nil
	}
	b.status = Resolving

	// 不满足判断条件的则标记为删除状态并删除其注册
	if b.cond != nil && !b.cond.Matches(c) {
		delete(c.cacheById, b.ID())
		b.status = Deleted
		return nil
	}

	// 将符合注册条件的 Bean 放入到缓存里面
	log.Debugf("register %s name:%q type:%q %s", b.getClass(), b.Name(), b.Type().String(), b.FileLine())
	c.cacheByType[b.Type()] = append(c.cacheByType[b.Type()], b)

	// 自动导出接口，这种情况仅对于结构体才会有效
	if typ := util.Indirect(b.Type()); typ.Kind() == reflect.Struct {
		if err := c.autoExport(typ, b); err != nil {
			return err
		}
	}

	// 按照导出类型放入缓存
	for t := range b.exports {
		if b.Type().Implements(t) {
			log.Debugf("register %s name:%q type:%q %s", b.getClass(), b.Name(), t.String(), b.FileLine())
			c.cacheByType[t] = append(c.cacheByType[t], b)
		} else {
			return fmt.Errorf("%s not implement %s interface", b.Description(), t)
		}
	}

	// 按照 Bean 的名字进行缓存
	c.cacheByName[b.name] = append(c.cacheByName[b.name], b)

	b.status = Resolved
	return nil
}

// autoExport 自动导出 Bean 实现的接口
func (c *applicationContext) autoExport(t reflect.Type, b *BeanDefinition) error {

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)

		_, export := f.Tag.Lookup("export")
		_, inject := f.Tag.Lookup("inject")
		_, autowire := f.Tag.Lookup("autowire")

		if !export { // 没有 export 标签

			// 嵌套才能递归；有注入标记的也不进行递归
			if !f.Anonymous || inject || autowire {
				continue
			}

			// 只处理结构体情况的递归，暂时不考虑接口的情况
			typ := util.Indirect(f.Type)
			if typ.Kind() == reflect.Struct {
				if err := c.autoExport(typ, b); err != nil {
					return err
				}
			}

			continue
		}

		// 有 export 标签的必须是接口类型
		if f.Type.Kind() != reflect.Interface {
			return errors.New("export can only use on interface")
		}

		// 不能导出需要注入的接口，因为会重复注册
		if export && (inject || autowire) {
			return errors.New("inject or autowire can't use with export")
		}

		// 不限定导出接口字段必须是空白标识符，但建议使用空白标识符
		b.Export(f.Type)
	}
	return nil
}

// runConfigers 执行 Config 函数
func (c *applicationContext) runConfigers(assembly *beanAssembly) error {
	for e := c.configers.Front(); e != nil; e = e.Next() {
		c := e.Value.(*Configer)
		_, err := c.fn.Call(assembly)
		if err != nil {
			return err
		}
	}
	return nil
}

// wireBeans 对 Bean 执行自动注入
func (c *applicationContext) wireBeans(assembly *beanAssembly) error {
	for _, b := range c.cacheById {
		if err := assembly.wireBean(b); err != nil {
			return err
		}
	}
	return nil
}

// Close 关闭容器上下文，用于通知 Bean 销毁等，该函数可以确保 Bean 的销毁顺序和注入顺序相反。
func (c *applicationContext) Close() {
	c.callAfterRefreshing()

	c.cancel()
	c.wg.Wait()

	log.Info("safe goroutines exited")

	assembly := toAssembly(c)
	for i := c.destroyers.Front(); i != nil; i = i.Next() {
		err := i.Value.(*destroyer).run(assembly)
		if err != nil {
			log.Error(err)
		}
	}
}
