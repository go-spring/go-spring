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

package SpringCore

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"

	"github.com/go-spring/go-spring-parent/spring-logger"
	"github.com/go-spring/go-spring-parent/spring-utils"
	"github.com/go-spring/go-spring/spring-core/sort"
)

// beanKey Bean's unique key, with type and name.
type beanKey struct {
	typ  reflect.Type
	name string
}

// newBeanKey beanKey 的构造函数
func newBeanKey(typ reflect.Type, name string) beanKey {
	return beanKey{typ: typ, name: name}
}

// beanCacheItem BeanCache's item, for type cache or name cache.
type beanCacheItem struct {
	beans []*BeanDefinition
}

// newBeanCacheItem beanCacheItem 的构造函数
func newBeanCacheItem() *beanCacheItem {
	return &beanCacheItem{
		beans: make([]*BeanDefinition, 0),
	}
}

func (item *beanCacheItem) store(bd *BeanDefinition) {
	item.beans = append(item.beans, bd)
}

// defaultSpringContext SpringContext 的默认实现
type defaultSpringContext struct {
	// 属性值列表接口
	Properties

	// 上下文接口
	ctx    context.Context
	cancel context.CancelFunc

	profile   string // 运行环境
	autoWired bool   // 是否开始自动绑定
	allAccess bool   // 是否允许注入私有字段

	beanMap         map[beanKey]*BeanDefinition // Bean 的集合
	methodBeans     []*BeanDefinition           // 方法 Beans
	beanCacheByName map[string]*beanCacheItem
	beanCacheByType map[reflect.Type]*beanCacheItem

	configers    *list.List // 配置方法集合
	destroyers   *list.List // 销毁函数集合
	destroyerMap map[beanKey]*destroyer
}

// NewDefaultSpringContext defaultSpringContext 的构造函数
func NewDefaultSpringContext() *defaultSpringContext {
	ctx, cancel := context.WithCancel(context.Background())
	return &defaultSpringContext{
		ctx:             ctx,
		cancel:          cancel,
		Properties:      NewDefaultProperties(),
		methodBeans:     make([]*BeanDefinition, 0),
		beanMap:         make(map[beanKey]*BeanDefinition),
		beanCacheByName: make(map[string]*beanCacheItem),
		beanCacheByType: make(map[reflect.Type]*beanCacheItem),
		configers:       list.New(),
		destroyers:      list.New(),
		destroyerMap:    make(map[beanKey]*destroyer),
	}
}

// Context 返回上下文接口
func (ctx *defaultSpringContext) Context() context.Context {
	return ctx.ctx
}

// GetProfile 返回运行环境
func (ctx *defaultSpringContext) GetProfile() string {
	return ctx.profile
}

// SetProfile 设置运行环境
func (ctx *defaultSpringContext) SetProfile(profile string) {
	ctx.profile = profile
}

// AllAccess 返回是否允许访问私有字段
func (ctx *defaultSpringContext) AllAccess() bool {
	return ctx.allAccess
}

// SetAllAccess 设置是否允许访问私有字段
func (ctx *defaultSpringContext) SetAllAccess(allAccess bool) {
	ctx.allAccess = allAccess
}

// checkAutoWired 检查是否已调用 AutoWireBeans 方法
func (ctx *defaultSpringContext) checkAutoWired() {
	if !ctx.autoWired {
		panic(errors.New("should call after AutoWireBeans"))
	}
}

// checkRegistration 检查注册是否已被冻结
func (ctx *defaultSpringContext) checkRegistration() {
	if ctx.autoWired {
		panic(errors.New("bean registration have been frozen"))
	}
}

// deleteBeanDefinition 删除 BeanDefinition。
func (ctx *defaultSpringContext) deleteBeanDefinition(bd *BeanDefinition) {
	key := newBeanKey(bd.Type(), bd.Name())
	bd.status = beanStatus_Deleted
	delete(ctx.beanMap, key)
}

// registerBeanDefinition 注册 BeanDefinition，重复注册会 panic。
func (ctx *defaultSpringContext) registerBeanDefinition(bd *BeanDefinition) {
	ctx.checkRegistration()

	key := newBeanKey(bd.Type(), bd.Name())
	if _, ok := ctx.beanMap[key]; ok {
		panic(fmt.Errorf("duplicate registration, bean: \"%s\"", bd.BeanId()))
	}

	ctx.beanMap[key] = bd
}

// RegisterBean 注册单例 Bean，不指定名称，重复注册会 panic。
func (ctx *defaultSpringContext) RegisterBean(bean interface{}) *BeanDefinition {
	return ctx.RegisterNameBean("", bean)
}

// RegisterNameBean 注册单例 Bean，需要指定名称，重复注册会 panic。
func (ctx *defaultSpringContext) RegisterNameBean(name string, bean interface{}) *BeanDefinition {
	bd := ToBeanDefinition(name, bean)
	ctx.registerBeanDefinition(bd)
	return bd
}

// RegisterBeanFn 注册单例构造函数 Bean，不指定名称，重复注册会 panic。
func (ctx *defaultSpringContext) RegisterBeanFn(fn interface{}, tags ...string) *BeanDefinition {
	return ctx.RegisterNameBeanFn("", fn, tags...)
}

// RegisterNameBeanFn 注册单例构造函数 Bean，需指定名称，重复注册会 panic。
func (ctx *defaultSpringContext) RegisterNameBeanFn(name string, fn interface{}, tags ...string) *BeanDefinition {
	bd := FnToBeanDefinition(name, fn, tags...)
	ctx.registerBeanDefinition(bd)
	return bd
}

// RegisterMethodBean 注册成员方法单例 Bean，不指定名称，重复注册会 panic。
// 必须给定方法名而不能通过遍历方法列表比较方法类型的方式获得函数名，因为不同方法的类型可能相同。
// 而且 interface 的方法类型不带 receiver 而成员方法的类型带有 receiver，两者类型也不好匹配。
func (ctx *defaultSpringContext) RegisterMethodBean(selector BeanSelector, method string, tags ...string) *BeanDefinition {
	return ctx.RegisterNameMethodBean("", selector, method, tags...)
}

// RegisterNameMethodBean 注册成员方法单例 Bean，需指定名称，重复注册会 panic。
// 必须给定方法名而不能通过遍历方法列表比较方法类型的方式获得函数名，因为不同方法的类型可能相同。
// 而且 interface 的方法类型不带 receiver 而成员方法的类型带有 receiver，两者类型也不好匹配。
func (ctx *defaultSpringContext) RegisterNameMethodBean(name string, selector BeanSelector, method string, tags ...string) *BeanDefinition {
	ctx.checkRegistration()

	if selector == nil || selector == "" {
		panic(errors.New("selector can't be nil or empty"))
	}

	bd := MethodToBeanDefinition(name, selector, method, tags...)
	ctx.methodBeans = append(ctx.methodBeans, bd)
	return bd
}

// @Incubate 注册成员方法单例 Bean，不指定名称，重复注册会 panic。
// method 形如 ServerInterface.Consumer (接口) 或 (*Server).Consumer (类型)。
func (ctx *defaultSpringContext) RegisterMethodBeanFn(method interface{}, tags ...string) *BeanDefinition {
	return ctx.RegisterNameMethodBeanFn("", method, tags...)
}

// @Incubate 注册成员方法单例 Bean，需指定名称，重复注册会 panic。
// method 形如 ServerInterface.Consumer (接口) 或 (*Server).Consumer (类型)。
func (ctx *defaultSpringContext) RegisterNameMethodBeanFn(name string, method interface{}, tags ...string) *BeanDefinition {

	var methodName string

	fnPtr := reflect.ValueOf(method).Pointer()
	fnInfo := runtime.FuncForPC(fnPtr)
	s := strings.Split(fnInfo.Name(), "/")
	ss := strings.Split(s[len(s)-1], ".")
	if len(ss) == 3 { // 包名.类型名.函数名
		methodName = ss[2]
	} else {
		panic(errors.New("error method func"))
	}

	parent := reflect.TypeOf(method).In(0)
	return ctx.RegisterNameMethodBean(name, parent, methodName, tags...)
}

// GetBean 获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
// 它和 FindBean 的区别是它在调用后能够保证返回的 Bean 已经完成了注入和绑定过程。
func (ctx *defaultSpringContext) GetBean(i interface{}, selector ...BeanSelector) bool {

	if i == nil {
		panic(errors.New("i can't be nil"))
	}

	ctx.checkAutoWired()

	// 使用指针才能够对外赋值
	if reflect.TypeOf(i).Kind() != reflect.Ptr {
		panic(errors.New("i must be pointer"))
	}

	s := BeanSelector("")
	if len(selector) > 0 {
		s = selector[0]
	}

	tag := ToSingletonTag(s)
	tag.Nullable = true

	v := reflect.ValueOf(i)
	w := newDefaultBeanAssembly(ctx)
	return w.getBeanValue(v.Elem(), tag, reflect.Value{}, "")
}

// FindBean 查询单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
// 它和 GetBean 的区别是它在调用后不能保证返回的 Bean 已经完成了注入和绑定过程。
func (ctx *defaultSpringContext) FindBean(selector BeanSelector) (*BeanDefinition, bool) {
	ctx.checkAutoWired()

	finder := func(fn func(*BeanDefinition) bool) (result []*BeanDefinition) {
		for _, bean := range ctx.beanMap {
			if bean.status != beanStatus_Resolving && fn(bean) {
				ctx.resolveBean(bean) // 避免 Bean 未被解析
				if bean.status != beanStatus_Deleted {
					result = append(result, bean)
				}
			}
		}
		return
	}

	var result []*BeanDefinition

	switch o := selector.(type) {
	case string:
		tag := ParseSingletonTag(o)
		result = finder(func(b *BeanDefinition) bool {
			return b.Match(tag.TypeName, tag.BeanName)
		})
	default:
		{
			t := reflect.TypeOf(o) // map、slice 等不是指针类型
			if t.Kind() == reflect.Ptr {
				if e := t.Elem(); e.Kind() == reflect.Interface {
					t = e // 接口类型去掉指针
				}
			}

			result = finder(func(b *BeanDefinition) bool {
				if beanType := b.Type(); beanType.AssignableTo(t) { // 必须类型兼容
					if beanType == t || t.Kind() != reflect.Interface {
						return true
					}
					if _, ok := b.exports[t]; ok {
						return true
					}
				}
				return false
			})
		}
	}

	count := len(result)

	// 没有找到
	if count == 0 {
		return nil, false
	}

	// 多于 1 个
	if count > 1 {
		msg := fmt.Sprintf("found %d beans, bean: \"%v\" [", len(result), selector)
		for _, b := range result {
			msg += "( " + b.Description() + " ), "
		}
		msg = msg[:len(msg)-2] + "]"
		panic(errors.New(msg))
	}

	// 恰好 1 个
	return result[0], true
}

// CollectBeans 收集数组或指针定义的所有符合条件的 Bean，收集到返回 true，否则返
// 回 false。该函数有两种模式:自动模式和指定模式。自动模式是指 selectors 参数为空，
// 这时候不仅会收集符合条件的单例 Bean，还会收集符合条件的数组 Bean (是指数组的元素
// 符合条件，然后把数组元素拆开一个个放到收集结果里面)。指定模式是指 selectors 参数
// 不为空，这时候只会收集单例 Bean，而且要求这些单例 Bean 不仅需要满足收集条件，而且
// 必须满足 selector 条件。另外，自动模式下不对收集结果进行排序，指定模式下根据
// selectors 列表的顺序对收集结果进行排序。
func (ctx *defaultSpringContext) CollectBeans(i interface{}, selectors ...BeanSelector) bool {
	ctx.checkAutoWired()

	if t := reflect.TypeOf(i); t.Kind() != reflect.Ptr || t.Elem().Kind() != reflect.Slice {
		panic(errors.New("i must be slice ptr"))
	}

	tag := CollectionTag{Nullable: true}

	for _, selector := range selectors {
		tag.Items = append(tag.Items, ToSingletonTag(selector))
	}

	w := newDefaultBeanAssembly(ctx)
	return w.collectBeans(reflect.ValueOf(i).Elem(), tag, "")
}

// getTypeCacheItem 查找指定类型的缓存项
func (ctx *defaultSpringContext) getTypeCacheItem(typ reflect.Type) *beanCacheItem {
	i, ok := ctx.beanCacheByType[typ]
	if !ok {
		i = newBeanCacheItem()
		ctx.beanCacheByType[typ] = i
	}
	return i
}

// getNameCacheItem 查找指定类型的缓存项
func (ctx *defaultSpringContext) getNameCacheItem(name string) *beanCacheItem {
	i, ok := ctx.beanCacheByName[name]
	if !ok {
		i = newBeanCacheItem()
		ctx.beanCacheByName[name] = i
	}
	return i
}

// autoExport 自动导出 Bean 实现的接口
func (ctx *defaultSpringContext) autoExport(t reflect.Type, bd *BeanDefinition) {

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
			typ := SpringUtils.Indirect(f.Type)
			if typ.Kind() == reflect.Struct {
				ctx.autoExport(typ, bd)
			}

			continue
		}

		// 有 export 标签的必须是接口类型
		if f.Type.Kind() != reflect.Interface {
			panic(errors.New("export can only use on interface"))
		}

		// 不能导出需要注入的接口，因为会重复注册
		if export && (inject || autowire) {
			panic(errors.New("inject or autowire can't use with export"))
		}

		// 不限定导出接口字段必须是空白标识符，但建议使用空白标识符
		bd.Export(f.Type)
	}
}

func (ctx *defaultSpringContext) typeCache(typ reflect.Type, bd *BeanDefinition) {
	SpringLogger.Debugf("register bean type:\"%s\" beanId:\"%s\" %s", typ.String(), bd.BeanId(), bd.FileLine())
	ctx.getTypeCacheItem(typ).store(bd)
}

func (ctx *defaultSpringContext) nameCache(name string, bd *BeanDefinition) {
	ctx.getNameCacheItem(name).store(bd)
}

// resolveBean 对 Bean 进行决议是否能够创建 Bean 的实例
func (ctx *defaultSpringContext) resolveBean(bd *BeanDefinition) {

	// 正在进行或者已经完成决议过程
	if bd.status >= beanStatus_Resolving {
		return
	}

	bd.status = beanStatus_Resolving

	// 如果是成员方法 Bean，需要首先决议它的父 Bean 是否能实例化
	if b, ok := bd.bean.(*methodBean); ok {
		ctx.resolveBean(b.parent)

		// 父 Bean 已经被删除了，子 Bean 也不应该存在
		if b.parent.status == beanStatus_Deleted {
			ctx.deleteBeanDefinition(bd)
			return
		}
	}

	// 不满足判断条件的则标记为删除状态并删除其注册
	if ok := bd.checkCondition(ctx); !ok {
		ctx.deleteBeanDefinition(bd)
		return
	}

	// 将符合注册条件的 Bean 放入到缓存里面
	ctx.typeCache(bd.Type(), bd)

	// 自动导出接口，这种情况仅对于结构体才会有效
	if typ := SpringUtils.Indirect(bd.Type()); typ.Kind() == reflect.Struct {
		ctx.autoExport(typ, bd)
	}

	// 按照导出类型放入缓存
	for t := range bd.exports {
		if bd.Type().Implements(t) {
			ctx.typeCache(t, bd)
		} else {
			panic(fmt.Errorf("%s not implement %s interface", bd.Description(), t.String()))
		}
	}

	// 按照 Bean 的名字进行缓存
	ctx.nameCache(bd.name, bd)

	bd.status = beanStatus_Resolved
}

// registerMethodBeans 注册方法 Bean
func (ctx *defaultSpringContext) registerMethodBeans() {

	var (
		selector string
		filter   func(*BeanDefinition) bool
	)

	for _, bd := range ctx.methodBeans {
		bean := bd.bean.(*fakeMethodBean)
		result := make([]*BeanDefinition, 0)

		switch e := bean.selector.(type) {
		case string:
			selector = e
			tag := ParseSingletonTag(e)
			filter = func(b *BeanDefinition) bool {
				return b.Match(tag.TypeName, tag.BeanName)
			}
		case *BeanDefinition:
			selector = e.BeanId()
			result = append(result, e)
		case reflect.Type:
			selector = e.String()
			filter = func(b *BeanDefinition) bool {
				return b.Type() == e
			}
		default:
			t := reflect.TypeOf(e)
			if t.Kind() == reflect.Ptr {
				if et := t.Elem(); et.Kind() == reflect.Interface {
					t = et // 接口类型去掉指针
				}
			}
			selector = t.String()
			filter = func(b *BeanDefinition) bool {
				return b.Type() == t
			}
		}

		if filter != nil {
			for _, b := range ctx.beanMap {
				if filter(b) {
					result = append(result, b)
				}
			}
		}

		if l := len(result); l == 0 {
			panic(fmt.Errorf("can't find parent bean: \"%s\"", selector))
		} else if l > 1 {
			panic(fmt.Errorf("found %d parent bean: \"%s\"", l, selector))
		}

		bd.bean = newMethodBean(result[0], bean.method, bean.tags)
		ctx.registerBeanDefinition(bd)
	}
}

// resolveConfigers 对 Config 函数进行决议是否能够保留它
func (ctx *defaultSpringContext) resolveConfigers() {

	// 对 config 函数进行决议
	for e := ctx.configers.Front(); e != nil; {
		next := e.Next()
		configer := e.Value.(*Configer)
		if ok := configer.checkCondition(ctx); !ok {
			ctx.configers.Remove(e)
		}
		e = next
	}

	// 对 config 函数进行排序
	ctx.configers = sort.TripleSorting(ctx.configers, getBeforeConfigers)
}

// resolveBeans 对 Bean 进行决议是否能够创建 Bean 的实例
func (ctx *defaultSpringContext) resolveBeans() {
	for _, bd := range ctx.beanMap {
		ctx.resolveBean(bd)
	}
}

// runConfigers 执行 Config 函数
func (ctx *defaultSpringContext) runConfigers(assembly *defaultBeanAssembly) {
	for e := ctx.configers.Front(); e != nil; e = e.Next() {
		configer := e.Value.(*Configer)
		if err := configer.run(assembly); err != nil {
			panic(err)
		}
	}
}

func (ctx *defaultSpringContext) destroyer(bd *BeanDefinition) *destroyer {
	k := newBeanKey(bd.Type(), bd.Name())
	d, ok := ctx.destroyerMap[k]
	if !ok {
		d = &destroyer{bean: bd}
		ctx.destroyerMap[k] = d
	}
	return d
}

// sortDestroyers 对销毁函数进行排序
func (ctx *defaultSpringContext) sortDestroyers() {
	for _, d := range ctx.destroyerMap {
		ctx.destroyers.PushBack(d)
	}
	ctx.destroyers = sort.TripleSorting(ctx.destroyers, getBeforeDestroyers)
}

// wireBeans 对 Bean 执行自动注入
func (ctx *defaultSpringContext) wireBeans(assembly *defaultBeanAssembly) {
	for _, bd := range ctx.beanMap {
		assembly.wireBeanDefinition(bd, false)
	}
}

// AutoWireBeans 对所有 Bean 进行依赖注入和属性绑定
func (ctx *defaultSpringContext) AutoWireBeans() {

	if ctx.autoWired {
		panic(errors.New("AutoWireBeans already called"))
	}

	// 注册所有的 Method Bean
	ctx.registerMethodBeans()

	ctx.autoWired = true

	ctx.resolveConfigers()
	ctx.resolveBeans()

	assembly := newDefaultBeanAssembly(ctx)

	defer func() { // 捕获自动注入过程中的异常，打印错误日志然后重新抛出
		if err := recover(); err != nil {
			SpringLogger.Errorf("%v ↩\n%s", err, assembly.wiringStack.path())
			panic(err)
		}
	}()

	ctx.runConfigers(assembly)
	ctx.wireBeans(assembly)

	ctx.sortDestroyers()
}

// WireBean 对外部的 Bean 进行依赖注入和属性绑定
func (ctx *defaultSpringContext) WireBean(i interface{}) {
	ctx.checkAutoWired()

	assembly := newDefaultBeanAssembly(ctx)

	defer func() { // 捕获自动注入过程中的异常，打印错误日志然后重新抛出
		if err := recover(); err != nil {
			SpringLogger.Errorf("%v ↩\n%s", err, assembly.wiringStack.path())
			panic(err)
		}
	}()

	bd := ToBeanDefinition("", i)
	assembly.wireBeanDefinition(bd, false)
}

// GetBeanDefinitions 获取所有 Bean 的定义，不能保证解析和注入，请谨慎使用该函数!
func (ctx *defaultSpringContext) GetBeanDefinitions() []*BeanDefinition {
	result := make([]*BeanDefinition, 0)
	for _, v := range ctx.beanMap {
		result = append(result, v)
	}
	return result
}

// Close 关闭容器上下文，用于通知 Bean 销毁等，该函数可以确保 Bean 的销毁顺序和注入顺序相反。
func (ctx *defaultSpringContext) Close(beforeDestroy ...func()) {

	// 上下文结束
	ctx.cancel()

	// 调用 destroy 之前的钩子函数
	for _, f := range beforeDestroy {
		f()
	}

	assembly := newDefaultBeanAssembly(ctx)

	// 按照顺序执行销毁函数
	for i := ctx.destroyers.Front(); i != nil; i = i.Next() {
		d := i.Value.(*destroyer)
		if err := d.bean.destroy.run(assembly); err != nil {
			SpringLogger.Error(err)
		}
	}
}

// Run 立即执行一个一次性的任务
func (ctx *defaultSpringContext) Run(fn interface{}, tags ...string) *Runner {
	ctx.checkAutoWired()
	return newRunner(ctx, fn, tags)
}

// Config 注册一个配置函数
func (ctx *defaultSpringContext) Config(fn interface{}, tags ...string) *Configer {
	return ctx.ConfigWithName("", fn, tags...)
}

// ConfigWithName 注册一个配置函数，名称的作用是对 Config 进行排重和排顺序。
func (ctx *defaultSpringContext) ConfigWithName(name string, fn interface{}, tags ...string) *Configer {
	configer := newConfiger(name, fn, tags)
	ctx.configers.PushBack(configer)
	return configer
}
