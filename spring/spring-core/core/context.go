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

// 实现了一个功能完善的运行时 IoC 容器。
package core

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
	"github.com/go-spring/spring-core/core/internal/sort"
	"github.com/go-spring/spring-core/log"
	"github.com/go-spring/spring-core/util"
)

// ApplicationContext 定义了 IoC 容器接口。
//
// 它的工作过程可以分为三个大的阶段：注册 Bean 列表、加载属性配置
// 文件、自动绑定。其中自动绑定又分为两个小阶段：解析（决议）和绑定。
//
// 一条需要谨记的注册规则是: AutoWireBeans 调用后就不能再注册新
// 的 Bean 了，这样做是因为实现起来更简单而且性能更高。
type ApplicationContext interface {

	// Context 返回上下文接口
	Context() context.Context

	// GetProfile 返回运行环境。
	GetProfile() string

	// HasProperty 查询属性值是否存在，属性名称统一转成小写。
	HasProperty(key string) bool

	// GetProperty 返回 key 转为小写后精确匹配的属性值，不存在返回 nil。
	GetProperty(key string) interface{}

	// PrefixProperties 返回 key 转为小写后作为前缀的所有符合条件的属性集合。
	PrefixProperties(key string) map[string]interface{}

	// WireBean 对对象或者构造函数的结果进行依赖注入和属性绑定，返回处理后的对象
	WireBean(objOrCtor interface{}, ctorArgs ...arg.Arg) (interface{}, error)

	// GetBean 获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
	// 它和 FindBean 的区别是它在调用后能够保证返回的 Bean 已经完成了注入和绑定过程。
	GetBean(i interface{}, selector ...bean.Selector) error

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

	// Range 遍历所有 Bean 的定义，不能保证解析和注入，请谨慎使用!
	Range(fn func(bd bean.Definition))

	// Go 安全地启动一个 goroutine
	Go(fn interface{}, args ...arg.Arg)

	// Invoke 立即执行一个一次性的任务
	Invoke(fn interface{}, args ...arg.Arg) error
}

// ConfigurableApplicationContext ApplicationContext 不允许内容被修改，这个可以。
type ConfigurableApplicationContext interface {

	// ApplicationContext 不能修改内容的接口
	ApplicationContext

	// Profile 设置运行环境
	Profile(profile string)

	// Properties 获取 Properties 对象
	Properties() conf.Properties

	// LoadProperties 加载属性配置，支持 properties、yaml 和 toml 三种文件格式。
	LoadProperties(filename string) error

	// Property 设置属性值，属性名称统一转成小写。
	Property(key string, value interface{})

	// Object 注册对象形式的 Bean。
	Object(i interface{}) *BeanDefinition

	// Factory 注册构造函数形式的 Bean。
	Factory(fn interface{}, args ...arg.Arg) *BeanDefinition

	// Config 注册一个配置函数
	Config(fn interface{}, args ...arg.Arg) *Configer

	// AddBean 添加 BeanDefinition 定义的 Bean。
	AddBean(bd *BeanDefinition) *BeanDefinition

	// Refresh 对所有 Bean 进行依赖注入和属性绑定
	Refresh()

	// Close 关闭容器上下文，用于通知 Bean 销毁等。
	// 该函数可以确保 Bean 的销毁顺序和注入顺序相反。
	Close(beforeDestroy ...func())
}

// applicationContext ApplicationContext 的默认实现
type applicationContext struct {

	// 上下文接口
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	profile string // 运行环境
	state   int    // 0 初始化，1 正在刷新，2 刷新完毕

	allBeans *util.Array // 所有注册点

	cacheById   map[string]*BeanDefinition
	cacheByName map[string]*util.Array
	cacheByType map[reflect.Type]*util.Array

	configers    *list.List // 配置方法集合
	destroyers   *list.List // 销毁函数集合
	destroyerMap map[string]*destroyer

	properties conf.Properties // 属性值列表接口
}

// NewApplicationContext applicationContext 的构造函数
func NewApplicationContext() *applicationContext {
	ctx, cancel := context.WithCancel(context.Background())
	return &applicationContext{
		ctx:          ctx,
		cancel:       cancel,
		properties:   conf.New(),
		allBeans:     util.NewArray(),
		cacheById:    make(map[string]*BeanDefinition),
		cacheByName:  make(map[string]*util.Array),
		cacheByType:  make(map[reflect.Type]*util.Array),
		configers:    list.New(),
		destroyers:   list.New(),
		destroyerMap: make(map[string]*destroyer),
	}
}

// Properties 获取 Properties 对象
func (ctx *applicationContext) Properties() conf.Properties {
	return ctx.properties
}

// LoadProperties 加载属性配置，支持 properties、yaml 和 toml 三种文件格式。
func (ctx *applicationContext) LoadProperties(filename string) error {
	return ctx.properties.Load(filename)
}

// HasProperty 查询属性值是否存在，属性名称统一转成小写。
func (ctx *applicationContext) HasProperty(key string) bool {
	return ctx.properties.Has(key)
}

// GetProperty 返回 key 转为小写后精确匹配的属性值，不存在返回 nil。
func (ctx *applicationContext) GetProperty(key string) interface{} {
	return ctx.properties.Get(key)
}

// Property 设置属性值，属性名称统一转成小写。
func (ctx *applicationContext) Property(key string, value interface{}) {
	ctx.properties.Set(key, value)
}

// PrefixProperties 返回 key 转为小写后作为前缀的所有符合条件的属性集合。
func (ctx *applicationContext) PrefixProperties(key string) map[string]interface{} {
	return ctx.properties.Prefix(key)
}

// Context 返回上下文接口
func (ctx *applicationContext) Context() context.Context {
	return ctx.ctx
}

// GetProfile 返回运行环境。
func (ctx *applicationContext) GetProfile() string {
	return ctx.profile
}

// Profile 设置运行环境
func (ctx *applicationContext) Profile(profile string) {
	ctx.profile = profile
}

// checkAutoWired 检查是否已调用 Refresh 方法
func (ctx *applicationContext) checkAutoWired() {
	if ctx.state == 0 {
		panic(errors.New("should call after Refresh"))
	}
}

// checkRegistration 检查注册是否已被冻结
func (ctx *applicationContext) checkRegistration() {
	if ctx.state > 0 {
		panic(errors.New("bean registration have been frozen"))
	}
}

func (ctx *applicationContext) deleteBeanDefinition(bd *BeanDefinition) {
	bd.setStatus(Deleted)
	delete(ctx.cacheById, bd.BeanId())
}

func (ctx *applicationContext) registerBeanDefinition(bd *BeanDefinition) {
	if _, ok := ctx.cacheById[bd.BeanId()]; ok {
		panic(fmt.Errorf("duplicate registration, bean:%q", bd.BeanId()))
	}
	ctx.cacheById[bd.BeanId()] = bd
}

// Object 注册对象形式的 Bean。
func (ctx *applicationContext) Object(i interface{}) *BeanDefinition {
	return ctx.AddBean(NewBean(reflect.ValueOf(i)))
}

// Factory 注册构造函数形式的 Bean。
func (ctx *applicationContext) Factory(fn interface{}, args ...arg.Arg) *BeanDefinition {
	return ctx.AddBean(NewBean(fn, args...))
}

// AddBean 添加 BeanDefinition 定义的 Bean。
func (ctx *applicationContext) AddBean(bd *BeanDefinition) *BeanDefinition {
	ctx.checkRegistration()
	ctx.allBeans.Append(bd)
	return bd
}

// GetBean 获取单例 Bean，它和 FindBean 的区别是它在调用后能够保证返回的 Bean 已经完成了注入和绑定过程。
func (ctx *applicationContext) GetBean(i interface{}, selector ...bean.Selector) error {

	if i == nil {
		return errors.New("i can't be nil")
	}

	ctx.checkAutoWired()

	// 使用指针才能够对外赋值
	if reflect.TypeOf(i).Kind() != reflect.Ptr {
		return errors.New("i must be pointer")
	}

	s := bean.Selector("")
	if len(selector) > 0 {
		s = selector[0]
	}

	w := toAssembly(ctx)
	v := reflect.ValueOf(i)
	tag := ToSingletonTag(s)
	return w.getBeanValue(v.Elem(), tag, reflect.Value{}, "")
}

// FindBean 返回符合条件的 Bean 集合，不保证返回的 Bean 已经完成注入和绑定过程。
func (ctx *applicationContext) FindBean(selector bean.Selector) ([]bean.Definition, error) {
	ctx.checkAutoWired()

	finder := func(fn func(*BeanDefinition) bool) (result []bean.Definition, err error) {
		for _, b := range ctx.cacheById {
			if b.getStatus() != Resolving && fn(b) {
				// 避免 Bean 未被解析
				if err = ctx.resolveBean(b); err != nil {
					return nil, err
				}
				if b.getStatus() != Deleted {
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
			return b.Match(tag.TypeName, tag.BeanName)
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
func (ctx *applicationContext) CollectBeans(i interface{}, selectors ...bean.Selector) error {
	ctx.checkAutoWired()

	if t := reflect.TypeOf(i); t.Kind() != reflect.Ptr || t.Elem().Kind() != reflect.Slice {
		return errors.New("i must be slice ptr")
	}

	tag := collectionTag{Nullable: true}

	for _, selector := range selectors {
		tag.Items = append(tag.Items, ToSingletonTag(selector))
	}

	return toAssembly(ctx).collectBeans(reflect.ValueOf(i).Elem(), tag, "")
}

func (ctx *applicationContext) getCacheByType(typ reflect.Type) *util.Array {
	i, ok := ctx.cacheByType[typ]
	if !ok {
		i = util.NewArray()
		ctx.cacheByType[typ] = i
	}
	return i
}

func (ctx *applicationContext) setCacheByType(t reflect.Type, b *BeanDefinition) {
	log.Debugf("register %s type:%q id:%q %s", b.getClass(), t.String(), b.BeanId(), b.FileLine())
	ctx.getCacheByType(t).Append(b)
}

func (ctx *applicationContext) getCacheByName(name string) *util.Array {
	i, ok := ctx.cacheByName[name]
	if !ok {
		i = util.NewArray()
		ctx.cacheByName[name] = i
	}
	return i
}

func (ctx *applicationContext) setCacheByName(name string, bd *BeanDefinition) {
	ctx.getCacheByName(name).Append(bd)
}

// autoExport 自动导出 Bean 实现的接口
func (ctx *applicationContext) autoExport(t reflect.Type, bd *BeanDefinition) error {

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
				if err := ctx.autoExport(typ, bd); err != nil {
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
		bd.Export(f.Type)
	}
	return nil
}

// resolveBean 对 Bean 进行决议是否能够创建 Bean 的实例
func (ctx *applicationContext) resolveBean(bd *BeanDefinition) error {

	// 正在进行或者已经完成决议过程
	if bd.getStatus() >= Resolving {
		return nil
	}

	bd.setStatus(Resolving)

	// 不满足判断条件的则标记为删除状态并删除其注册
	if bd.cond != nil && !bd.cond.Matches(ctx) {
		ctx.deleteBeanDefinition(bd)
		return nil
	}

	// 将符合注册条件的 Bean 放入到缓存里面
	ctx.setCacheByType(bd.Type(), bd)

	// 自动导出接口，这种情况仅对于结构体才会有效
	if typ := util.Indirect(bd.Type()); typ.Kind() == reflect.Struct {
		if err := ctx.autoExport(typ, bd); err != nil {
			return err
		}
	}

	// 按照导出类型放入缓存
	for t := range bd.exports {
		if bd.Type().Implements(t) {
			ctx.setCacheByType(t, bd)
		} else {
			return fmt.Errorf("%s not implement %s interface", bd.Description(), t)
		}
	}

	// 按照 Bean 的名字进行缓存
	ctx.setCacheByName(bd.BeanName(), bd)

	bd.setStatus(Resolved)
	return nil
}

func (ctx *applicationContext) registerAllBeans() {
	for i := 0; i < ctx.allBeans.Len(); i++ {
		bd := ctx.allBeans.Get(i).(*BeanDefinition)
		ctx.registerBeanDefinition(bd)
	}
}

// resolveConfigers 对 Config 函数进行决议是否能够保留它
func (ctx *applicationContext) resolveConfigers() {

	// 对 config 函数进行决议
	for e := ctx.configers.Front(); e != nil; {
		next := e.Next()
		configer := e.Value.(*Configer)
		if configer.cond != nil && !configer.cond.Matches(ctx) {
			ctx.configers.Remove(e)
		}
		e = next
	}

	// 对 config 函数进行排序
	ctx.configers = sort.TripleSorting(ctx.configers, getBeforeConfigers)
}

// resolveBeans 对 Bean 进行决议是否能够创建 Bean 的实例
func (ctx *applicationContext) resolveBeans() error {
	for _, bd := range ctx.cacheById {
		if err := ctx.resolveBean(bd); err != nil {
			return err
		}
	}
	return nil
}

// runConfigers 执行 Config 函数
func (ctx *applicationContext) runConfigers(assembly *beanAssembly) error {
	for e := ctx.configers.Front(); e != nil; e = e.Next() {
		configer := e.Value.(*Configer)
		if err := configer.r.Run(assembly); err != nil {
			return err
		}
	}
	return nil
}

// destroyer 某个 Bean 可能会被多个 Bean 依赖，因此需要排重处理。
func (ctx *applicationContext) destroyer(bd *BeanDefinition) *destroyer {
	d, ok := ctx.destroyerMap[bd.BeanId()]
	if !ok {
		d = &destroyer{current: bd}
		ctx.destroyerMap[bd.BeanId()] = d
	}
	return d
}

// sortDestroyers 对销毁函数进行排序
func (ctx *applicationContext) sortDestroyers() {
	for _, d := range ctx.destroyerMap {
		ctx.destroyers.PushBack(d)
	}
	ctx.destroyers = sort.TripleSorting(ctx.destroyers, getBeforeDestroyers)
}

// wireBeans 对 Bean 执行自动注入
func (ctx *applicationContext) wireBeans(assembly *beanAssembly) error {
	for _, bd := range ctx.cacheById {
		if err := assembly.wireBeanDefinition(bd, false); err != nil {
			return err
		}
	}
	return nil
}

// Refresh 对所有 Bean 进行依赖注入和属性绑定
func (ctx *applicationContext) Refresh() {

	if ctx.state > 0 {
		panic(errors.New("already refreshed"))
	}

	var (
		err      error
		assembly *beanAssembly
	)

	defer func() {
		if err != nil {
			if assembly != nil {
				log.Errorf("%v ↩\n%s", err, assembly.stack.path())
			}
			panic(err)
		}
	}()

	// 处理 Method Bean 等
	ctx.registerAllBeans()

	ctx.state = 1

	ctx.resolveConfigers()
	if err = ctx.resolveBeans(); err != nil {
		return
	}

	assembly = toAssembly(ctx)

	if err = ctx.runConfigers(assembly); err != nil {
		return
	}

	if err = ctx.wireBeans(assembly); err != nil {
		return
	}

	ctx.sortDestroyers()

	ctx.state = 2
}

// WireBean 对对象或者构造函数的结果进行依赖注入和属性绑定，返回处理后的对象
func (ctx *applicationContext) WireBean(objOrCtor interface{}, ctorArgs ...arg.Arg) (interface{}, error) {
	ctx.checkAutoWired()
	assembly := toAssembly(ctx)
	bd := NewBean(objOrCtor, ctorArgs...)
	err := assembly.wireBeanDefinition(bd, false)
	if err != nil {
		return nil, err
	}
	return bd.Interface(), nil
}

// Range 遍历所有 Bean 的定义，不能保证解析和注入，请谨慎使用!
func (ctx *applicationContext) Range(fn func(bd bean.Definition)) {
	for _, v := range ctx.cacheById {
		fn(v)
	}
}

// Close 关闭容器上下文，用于通知 Bean 销毁等，该函数可以确保 Bean 的销毁顺序和注入顺序相反。
func (ctx *applicationContext) Close(beforeDestroy ...func()) {

	// 上下文结束
	ctx.cancel()

	// 调用 destroy 之前的钩子函数
	for _, f := range beforeDestroy {
		f()
	}

	// 等待 safe goroutines 全部退出
	ctx.wg.Wait()

	log.Info("safe goroutines exited")

	assembly := toAssembly(ctx)

	// 按照顺序执行销毁函数
	for i := ctx.destroyers.Front(); i != nil; i = i.Next() {
		d := i.Value.(*destroyer)
		if err := d.run(assembly); err != nil {
			log.Error(err)
		}
	}
}

// Invoke 立即执行一个一次性的任务
func (ctx *applicationContext) Invoke(fn interface{}, args ...arg.Arg) error {
	ctx.checkAutoWired()
	if fnType := reflect.TypeOf(fn); util.FuncType(fnType) {
		if util.ReturnNothing(fnType) || util.ReturnOnlyError(fnType) {
			return arg.Runner(fn, false, args).Run(toAssembly(ctx))
		}
	}
	return errors.New("fn should be func() or func()error")
}

// Config 注册一个配置函数
func (ctx *applicationContext) Config(fn interface{}, args ...arg.Arg) *Configer {
	configer := Config(fn, args)
	ctx.configers.PushBack(configer)
	return configer
}

// Go 安全地启动一个 goroutine
func (ctx *applicationContext) Go(fn interface{}, args ...arg.Arg) {

	ctx.checkAutoWired()
	fnType := reflect.TypeOf(fn)
	if util.FuncType(fnType) && util.ReturnNothing(fnType) {

		ctx.wg.Add(1)
		go func() {
			defer ctx.wg.Done()

			defer func() {
				if err := recover(); err != nil {
					log.Error(err)
				}
			}()

			_ = arg.Runner(fn, false, args).Run(toAssembly(ctx))
		}()
	}
	panic(errors.New("fn should be func()"))
}
