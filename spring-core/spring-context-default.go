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
	"sort"
	"sync"

	"github.com/go-spring/go-spring-parent/spring-logger"
	"github.com/go-spring/go-spring-parent/spring-utils"
)

// beanKey Bean's unique key, type+name.
type beanKey struct {
	name string
	typ  reflect.Type
}

// beanCacheItem BeanCache's item.
type beanCacheItem struct {
	beans []*BeanDefinition
	mark  int // 1 结果已锁定
}

// newBeanCacheItem beanCacheItem 的构造函数
func newBeanCacheItem() *beanCacheItem {
	return &beanCacheItem{
		beans: make([]*BeanDefinition, 0),
	}
}

// copyTo 把现有元素拷贝到新缓存里
func (item *beanCacheItem) copyTo(c *beanCacheItem) {
	c.beans = append(c.beans, item.beans...)
}

// find 顺序查找元素在数组中的位置
func (item *beanCacheItem) find(bd *BeanDefinition) int {
	for i, b := range item.beans {
		if b == bd {
			return i
		}
	}
	return -1
}

func (item *beanCacheItem) store(t reflect.Type, bd *BeanDefinition, check bool) bool {
	if check && item.find(bd) >= 0 { // 预期数据量较少，因此未使用 map 进行存储。
		return false
	}
	SpringLogger.Debugf("register bean type:\"%s\" beanId:\"%s\" %s", t.String(), bd.BeanId(), bd.Caller())
	item.beans = append(item.beans, bd)
	return true
}

// wiringStack 存储绑定中的 Bean
type wiringStack struct {
	stack    *list.List
	watchers []WiringWatcher
}

// newWiringStack wiringStack 的构造函数
func newWiringStack(watchers []WiringWatcher) *wiringStack {

	if len(watchers) == 0 { // 添加默认的注入监视器
		watchers = append(watchers, func(bd IBeanDefinition, event WiringEvent) {
			switch event {
			case WiringEvent_Push:
				SpringLogger.Tracef("wiring %s", bd.Description())
			case WiringEvent_Pop:
				SpringLogger.Tracef("wired %s", bd.Description())
			}
		})
	}

	return &wiringStack{
		stack:    list.New(),
		watchers: watchers,
	}
}

// pushBack 添加一个 Item 到尾部
func (s *wiringStack) pushBack(bd IBeanDefinition) {
	s.stack.PushBack(bd)

	for _, w := range s.watchers {
		w(bd, WiringEvent_Push)
	}
}

// popBack 删除尾部的 item
func (s *wiringStack) popBack() {
	e := s.stack.Remove(s.stack.Back())

	for _, w := range s.watchers {
		w(e.(IBeanDefinition), WiringEvent_Pop)
	}
}

// path 返回依赖注入的路径
func (s *wiringStack) path() (path string) {
	for e := s.stack.Front(); e != nil; e = e.Next() {
		w := e.Value.(IBeanDefinition)
		path += fmt.Sprintf("=> %s ↩\n", w.Description())
	}
	return path[:len(path)-1]
}

// beanAssembly Bean 组装车间
type beanAssembly interface {
	springContext() SpringContext
	collectBeans(v reflect.Value) bool
	getBeanValue(v reflect.Value, beanId string, parent reflect.Value, field string) bool
}

// defaultBeanAssembly beanAssembly 的默认版本
type defaultBeanAssembly struct {
	springCtx   *defaultSpringContext
	wiringStack *wiringStack
}

// newDefaultBeanAssembly defaultBeanAssembly 的构造函数
func newDefaultBeanAssembly(springContext *defaultSpringContext,
	watchers []WiringWatcher) *defaultBeanAssembly {

	return &defaultBeanAssembly{
		springCtx:   springContext,
		wiringStack: newWiringStack(watchers),
	}
}

func (beanAssembly *defaultBeanAssembly) springContext() SpringContext {
	return beanAssembly.springCtx
}

// getCacheItem 获取指定类型的缓存项，返回值不会为 nil。
func (beanAssembly *defaultBeanAssembly) getCacheItem(t reflect.Type) *beanCacheItem {
	beanCache := &beanAssembly.springCtx.beanCache

	// 严格模式下必须使用 AsInterface() 导出接口
	if beanAssembly.springCtx.Strict {
		if c, ok := beanCache.Load(t); ok {
			return c.(*beanCacheItem)
		}
		return newBeanCacheItem()
	}

	// 处理具体类型
	if k := t.Kind(); k != reflect.Interface {

		// 如果缓存已存在则直接返回
		if c, ok := beanCache.Load(t); ok {
			return c.(*beanCacheItem)
		}

		// 如果是数组类型，则需要处理其元素类型
		if k == reflect.Slice || k == reflect.Array {
			beanAssembly.getCacheItem(t.Elem())
		}

		result := newBeanCacheItem()
		beanCache.Store(t, result)
		return result
	}

	// 处理接口类型

	var (
		check bool
		cache *beanCacheItem
	)

	if c, ok := beanCache.Load(t); ok {
		item := c.(*beanCacheItem)
		if item.mark == 1 {
			return item
		} else {
			cache = newBeanCacheItem()
			item.copyTo(cache)
			check = true
		}
	} else {
		cache = newBeanCacheItem()
	}

	// 锁定搜索结果
	cache.mark = 1

	for _, bd := range beanAssembly.springCtx.beanMap {
		if bd.Type().AssignableTo(t) && cache.store(t, bd, check) && len(bd.exports) == 0 {
			SpringLogger.Warnf("you should call AsInterface() on %s", bd.Description())
		}
	}

	beanCache.Store(t, cache)
	return cache
}

func (_ *defaultBeanAssembly) getBeanType(v reflect.Value) (reflect.Type, bool) {
	if v.IsValid() {
		if beanType := v.Type(); IsRefType(beanType.Kind()) {
			return beanType, true
		}
	}
	return nil, false
}

// getBeanValue 根据 BeanId 查找 Bean 并返回 Bean 源的值
func (beanAssembly *defaultBeanAssembly) getBeanValue(v reflect.Value, beanId string, parent reflect.Value, field string) bool {

	var (
		ok       bool
		beanType reflect.Type
	)

	if beanType, ok = beanAssembly.getBeanType(v); !ok {
		panic(fmt.Errorf("receiver must be ref type, bean: \"%s\" field: %s", beanId, field))
	}

	result := make([]*BeanDefinition, 0)

	typeName, beanName, nullable := ParseBeanId(beanId)
	m := beanAssembly.getCacheItem(beanType)
	for _, bean := range m.beans {
		// 不能将自身赋给自身的字段 && 类型全限定名匹配
		if bean.Value() != parent && bean.Match(typeName, beanName) {
			result = append(result, bean)
		}
	}

	count := len(result)

	// 没有找到
	if count == 0 {
		if nullable {
			return false
		} else {
			panic(fmt.Errorf("can't find bean, bean: \"%s\" field: %s type: %s", beanId, field, beanType))
		}
	}

	var primaryBeans []*BeanDefinition

	for _, bean := range result {
		if bean.primary {
			primaryBeans = append(primaryBeans, bean)
		}
	}

	if len(primaryBeans) > 1 {
		msg := fmt.Sprintf("found %d primary beans, bean: \"%s\" field: %s type: %s [", len(primaryBeans), beanId, field, beanType)
		for _, b := range primaryBeans {
			msg += "( " + b.Description() + " ), "
		}
		msg = msg[:len(msg)-2] + "]"
		panic(errors.New(msg))
	}

	if len(primaryBeans) == 0 {
		if count > 1 {
			msg := fmt.Sprintf("found %d beans, bean: \"%s\" field: %s type: %s [", len(result), beanId, field, beanType)
			for _, b := range result {
				msg += "( " + b.Description() + " ), "
			}
			msg = msg[:len(msg)-2] + "]"
			panic(errors.New(msg))
		}
		primaryBeans = append(primaryBeans, result[0])
	}

	// 依赖注入
	beanAssembly.wireBeanDefinition(primaryBeans[0], false)

	// 恰好 1 个
	v0 := SpringUtils.ValuePatchIf(v, beanAssembly.springCtx.AllAccess())
	v0.Set(primaryBeans[0].Value())
	return true
}

// collectBeans 收集符合条件的 Bean 源
func (beanAssembly *defaultBeanAssembly) collectBeans(v reflect.Value) bool {

	t := v.Type()
	ev := reflect.New(t).Elem()

	// 查找数组类型
	{
		m := beanAssembly.getCacheItem(t)
		for _, d := range m.beans {
			for i := 0; i < d.Value().Len(); i++ {
				di := d.Value().Index(i)
				if di.Kind() == reflect.Struct {
					beanAssembly.wireSliceItem(di.Addr(), d)
				} else if di.Kind() == reflect.Ptr {
					if de := di.Elem(); de.Kind() == reflect.Struct {
						beanAssembly.wireSliceItem(di, d)
					}
				}
				ev = reflect.Append(ev, di)
			}
		}
	}

	// 查找单例类型
	{
		et := t.Elem()
		m := beanAssembly.getCacheItem(et)
		for _, d := range m.beans {
			beanAssembly.wireBeanDefinition(d, false)
			ev = reflect.Append(ev, d.Value())
		}
	}

	if ev.Len() > 0 {
		v = SpringUtils.ValuePatchIf(v, beanAssembly.springCtx.AllAccess())
		v.Set(ev)
		return true
	}
	return false
}

// fieldBeanDefinition 带字段名称的 BeanDefinition 实现
type fieldBeanDefinition struct {
	*BeanDefinition
	field string // 字段名称
}

// Description 返回 Bean 的详细描述
func (d *fieldBeanDefinition) Description() string {
	return fmt.Sprintf("%s field: %s %s", d.bean.beanClass(), d.field, d.Caller())
}

// delegateBeanDefinition 代理功能的 BeanDefinition 实现
type delegateBeanDefinition struct {
	*BeanDefinition
	delegate IBeanDefinition // 代理项
}

// Description 返回 Bean 的详细描述
func (d *delegateBeanDefinition) Description() string {
	return fmt.Sprintf("%s value %s", d.delegate.springBean().beanClass(), d.delegate.Caller())
}

// wireSliceItem 注入 slice 的元素值
func (beanAssembly *defaultBeanAssembly) wireSliceItem(v reflect.Value, d IBeanDefinition) {
	bd := ValueToBeanDefinition("", v)
	bd.file = d.getFile()
	bd.line = d.getLine()
	beanAssembly.wireBeanDefinition(bd, false)
}

// wireBeanDefinition 绑定 BeanDefinition 指定的 Bean
func (beanAssembly *defaultBeanAssembly) wireBeanDefinition(bd IBeanDefinition, onlyAutoWire bool) {

	// 是否已删除
	if bd.getStatus() == beanStatus_Deleted {
		panic(fmt.Errorf("bean: \"%s\" have been deleted", bd.BeanId()))
	}

	// 是否已绑定
	if bd.getStatus() == beanStatus_Wired {
		return
	}

	// 将当前 Bean 放入注入栈，以便检测循环依赖。
	beanAssembly.wiringStack.pushBack(bd)

	// 是否循环依赖
	if bd.getStatus() == beanStatus_Wiring {
		if _, ok := bd.springBean().(*objectBean); !ok {
			panic(errors.New("found circle autowire"))
		}
		return
	}

	bd.setStatus(beanStatus_Wiring)

	// 首先初始化当前 Bean 不直接依赖的那些 Bean
	for _, selector := range bd.getDependsOn() {
		if bean, ok := beanAssembly.springCtx.FindBean(selector); !ok {
			panic(fmt.Errorf("can't find bean: \"%v\"", selector))
		} else {
			beanAssembly.wireBeanDefinition(bean, false)
		}
	}

	// 如果是成员方法 Bean，需要首先初始化它的父 Bean
	if mBean, ok := bd.springBean().(*methodBean); ok {
		beanAssembly.wireBeanDefinition(mBean.parent, false)
	}

	switch bean := bd.springBean().(type) {
	case *objectBean: // 原始对象
		beanAssembly.wireObjectBean(bd, onlyAutoWire)
	case *constructorBean: // 构造函数
		fnValue := reflect.ValueOf(bean.fn)
		beanAssembly.wireFunctionBean(fnValue, &bean.functionBean, bd)
	case *methodBean: // 成员方法
		fnValue := bean.parent.Value().MethodByName(bean.method)
		beanAssembly.wireFunctionBean(fnValue, &bean.functionBean, bd)
	default:
		panic(errors.New("unknown spring bean type"))
	}

	// 如果有则执行用户设置的初始化函数
	if bd.getInit() != nil {
		fnValue := reflect.ValueOf(bd.getInit())
		fnValue.Call([]reflect.Value{bd.Value()})
	}

	bd.setStatus(beanStatus_Wired)

	// 删除保存的注入帧
	beanAssembly.wiringStack.popBack()
}

// wireObjectBean 对原始对象进行注入
func (beanAssembly *defaultBeanAssembly) wireObjectBean(bd IBeanDefinition, onlyAutoWire bool) {
	st := bd.Type()
	switch sk := st.Kind(); sk {
	case reflect.Slice:
		et := st.Elem()
		if ek := et.Kind(); ek == reflect.Struct { // 结构体数组
			v := bd.Value()
			for i := 0; i < v.Len(); i++ {
				iv := v.Index(i).Addr()
				beanAssembly.wireSliceItem(iv, bd)
			}
		} else if ek == reflect.Ptr { // 指针数组
			it := et.Elem()
			if ik := it.Kind(); ik == reflect.Struct { // 结构体指针数组
				v := bd.Value()
				for p := 0; p < v.Len(); p++ {
					pv := v.Index(p)
					beanAssembly.wireSliceItem(pv, bd)
				}
			}
		}
	case reflect.Ptr:
		if et := st.Elem(); et.Kind() == reflect.Struct { // 结构体指针

			var etName string
			if etName = et.Name(); etName == "" {
				etName = et.String()
			}

			sv := bd.Value()
			ev := sv.Elem()

			for i := 0; i < et.NumField(); i++ {
				// 字段包含 value 标签时嵌套处理只注入变量
				fieldOnlyAutoWire := false

				ft := et.Field(i)
				fv := ev.Field(i)

				fieldName := etName + ".$" + ft.Name

				// 处理 value 标签
				if !onlyAutoWire {
					// 避免父结构体有 value 标签重新解析导致失败的情况
					if tag, ok := ft.Tag.Lookup("value"); ok {
						fieldOnlyAutoWire = true
						bindStructField(beanAssembly.springCtx, fv, tag, bindOption{
							fieldName: fieldName,
							allAccess: beanAssembly.springCtx.AllAccess(),
						})
					}
				}

				// 处理 autowire 标签
				if beanId, ok := ft.Tag.Lookup("autowire"); ok {
					beanAssembly.wireStructField(fv, beanId, sv, fieldName)
				}

				// 处理结构体类型的字段，防止递归所以不支持指针结构体字段
				if ft.Type.Kind() == reflect.Struct {
					// 开放私有字段，但是不会更新原属性
					fv0 := SpringUtils.ValuePatchIf(fv, beanAssembly.springCtx.AllAccess())
					if fv0.CanSet() {

						b := ValueToBeanDefinition("", fv0.Addr())
						b.file = bd.getFile()
						b.line = bd.getLine()
						fbd := &fieldBeanDefinition{b, fieldName}
						beanAssembly.wireBeanDefinition(fbd, fieldOnlyAutoWire)
					}
				}
			}
		}
	}
}

// wireFunctionBean 对函数定义 Bean 进行注入
func (beanAssembly *defaultBeanAssembly) wireFunctionBean(fnValue reflect.Value, bean *functionBean, bd IBeanDefinition) {

	var in []reflect.Value

	if bean.stringArg != nil {
		if r := bean.stringArg.Get(beanAssembly, bd); len(r) > 0 {
			in = append(in, r...)
		}
	}

	if bean.optionArg != nil {
		if r := bean.optionArg.Get(beanAssembly, bd); len(r) > 0 {
			in = append(in, r...)
		}
	}

	out := fnValue.Call(in)

	// 获取第一个返回值
	val := out[0]

	// 检查是否有 error 返回
	if len(out) == 2 {
		if err := out[1].Interface(); err != nil {
			panic(fmt.Errorf("function bean: \"%s\" return error: %v", bd.Caller(), err))
		}
	}

	if IsRefType(val.Kind()) {
		// 如果实现接口的是值类型，那么需要转换成指针类型然后赋给接口
		if val.Kind() == reflect.Interface && IsValueType(val.Elem().Kind()) {
			ptrVal := reflect.New(val.Elem().Type())
			ptrVal.Elem().Set(val.Elem())
			bean.rValue.Set(ptrVal)
		} else {
			bean.rValue.Set(val)
		}
	} else {
		bean.rValue.Elem().Set(val)
	}

	if bean.Value().IsNil() {
		panic(fmt.Errorf("function bean: \"%s\" return nil", bd.Caller()))
	}

	// 对返回值进行依赖注入
	b := &BeanDefinition{
		name:   bd.Name(),
		status: beanStatus_Default,
		file:   bd.getFile(),
		line:   bd.getLine(),
	}

	if bean.Type().Kind() == reflect.Interface {
		b.bean = newObjectBean(bean.Value().Elem())
	} else {
		b.bean = newObjectBean(bean.Value())
	}

	beanAssembly.wireBeanDefinition(&delegateBeanDefinition{b, bd}, false)
}

// wireStructField 对结构体的字段进行绑定
func (beanAssembly *defaultBeanAssembly) wireStructField(v reflect.Value,
	beanId string, parent reflect.Value, field string) {

	_, beanName, nullable := ParseBeanId(beanId)
	if beanName == "[]" { // 收集模式，autowire:"[]"

		// 收集模式的绑定对象必须是数组
		if v.Type().Kind() != reflect.Slice {
			panic(fmt.Errorf("field: %s should be slice", field))
		}

		ok := beanAssembly.collectBeans(v)
		if !ok && !nullable { // 没找到且不能为空则 panic
			panic(fmt.Errorf("can't find bean: \"%s\" field: %s", beanId, field))
		}

	} else { // 匹配模式，autowire:"" or autowire:"name"
		beanAssembly.getBeanValue(v, beanId, parent, field)
	}
}

// defaultSpringContext SpringContext 的默认版本
type defaultSpringContext struct {
	// 属性值列表接口
	Properties

	// 上下文接口
	context.Context
	cancel context.CancelFunc

	profile   string // 运行环境
	autoWired bool   // 已经开始自动绑定
	allAccess bool   // 允许注入私有字段

	eventNotify func(event ContextEvent) // 事件通知函数

	beanMap     map[beanKey]*BeanDefinition // Bean 的集合
	methodBeans []*BeanDefinition           // 方法 Beans
	configers   *list.List                  // 配置方法集合

	// Bean 的缓存，使用线程安全的 map 是考虑到运行时可能有
	// 并发操作，另外 resolveBeans 的时候一步步的创建缓存。
	beanCache sync.Map

	Sort   bool // 自动注入期间是否按照 BeanId 进行排序并依次进行注入
	Strict bool // 严格模式，true 必须使用 AsInterface() 导出接口
}

// NewDefaultSpringContext defaultSpringContext 的构造函数
func NewDefaultSpringContext() *defaultSpringContext {
	ctx, cancel := context.WithCancel(context.Background())
	return &defaultSpringContext{
		Context:     ctx,
		Strict:      true,
		cancel:      cancel,
		Properties:  NewDefaultProperties(),
		methodBeans: make([]*BeanDefinition, 0),
		beanMap:     make(map[beanKey]*BeanDefinition),
		configers:   list.New(),
	}
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

// SetEventNotify 设置 Context 事件通知函数
func (ctx *defaultSpringContext) SetEventNotify(notify func(event ContextEvent)) {
	ctx.eventNotify = notify
}

// checkAutoWired 检查是否已调用 AutoWireBeans 方法
func (ctx *defaultSpringContext) checkAutoWired() {
	if !ctx.autoWired {
		panic(errors.New("should call after ctx.AutoWireBeans()"))
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
	key := beanKey{bd.name, bd.Type()}
	bd.status = beanStatus_Deleted
	delete(ctx.beanMap, key)
}

// registerBeanDefinition 注册 BeanDefinition，重复注册会 panic。
func (ctx *defaultSpringContext) registerBeanDefinition(d *BeanDefinition) {
	ctx.checkRegistration()

	k := beanKey{d.name, d.Type()}
	if _, ok := ctx.beanMap[k]; ok {
		panic(fmt.Errorf("duplicate registration, bean: \"%s\"", d.BeanId()))
	}

	ctx.beanMap[k] = d
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
// selector 可以是 *BeanDefinition，可以是 BeanId，还可以是 (Type)(nil) 变量。
// 必须给定方法名而不能通过遍历方法列表比较方法类型的方式获得函数名，因为不同方法的类型可能相同。
// 而且 interface 的方法类型不带 receiver 而成员方法的类型带有 receiver，两者类型不好匹配。
func (ctx *defaultSpringContext) RegisterMethodBean(selector interface{}, method string, tags ...string) *BeanDefinition {
	return ctx.RegisterNameMethodBean("", selector, method, tags...)
}

// RegisterNameMethodBean 注册成员方法单例 Bean，需指定名称，重复注册会 panic。
// selector 可以是 *BeanDefinition，可以是 BeanId，还可以是 (Type)(nil) 变量。
// 必须给定方法名而不能通过遍历方法列表比较方法类型的方式获得函数名，因为不同方法的类型可能相同。
// 而且 interface 的方法类型不带 receiver 而成员方法的类型带有 receiver，两者类型不好匹配。
func (ctx *defaultSpringContext) RegisterNameMethodBean(name string, selector interface{}, method string, tags ...string) *BeanDefinition {
	ctx.checkRegistration()

	if selector == nil || selector == "" {
		panic(errors.New("selector can't be nil or empty"))
	}

	bd := MethodToBeanDefinition(name, selector, method, tags...)
	ctx.methodBeans = append(ctx.methodBeans, bd)
	return bd
}

// GetBean 根据类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
func (ctx *defaultSpringContext) GetBean(i interface{}, watchers ...WiringWatcher) bool {
	return ctx.GetBeanByName("?", i, watchers...)
}

// GetBeanByName 根据名称和类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
func (ctx *defaultSpringContext) GetBeanByName(beanId string, i interface{}, watchers ...WiringWatcher) bool {
	SpringUtils.Panic(errors.New("i can't be nil")).When(i == nil)

	ctx.checkAutoWired()

	// 确保存在可空标记，抑制 panic 效果。
	if beanId == "" || beanId[len(beanId)-1] != '?' {
		beanId += "?"
	}

	t := reflect.TypeOf(i)

	// 使用指针才能够对外赋值
	if t.Kind() != reflect.Ptr {
		panic(errors.New("i must be pointer"))
	}

	v := reflect.ValueOf(i)
	w := newDefaultBeanAssembly(ctx, watchers)
	return w.getBeanValue(v.Elem(), beanId, reflect.Value{}, "")
}

// FindBean 获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
// selector 可以是 BeanId，还可以是 (Type)(nil) 变量，Type 为接口类型时带指针。
func (ctx *defaultSpringContext) FindBean(selector interface{}) (*BeanDefinition, bool) {
	ctx.checkAutoWired()

	finder := func(fn func(*BeanDefinition) bool) (result []*BeanDefinition) {
		for _, bean := range ctx.beanMap {
			if fn(bean) {

				// 如果 Bean 正在解析则跳过
				if bean.status == beanStatus_Resolving {
					continue
				}

				// 避免 Bean 还未解析
				ctx.resolveBean(bean)

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
		typeName, beanName, _ := ParseBeanId(o)
		result = finder(func(b *BeanDefinition) bool {
			return b.Match(typeName, beanName)
		})
	default:
		{
			t := reflect.TypeOf(o) // map、slice 等不是指针类型
			if t.Kind() == reflect.Ptr {
				e := t.Elem()
				if e.Kind() == reflect.Interface {
					t = e // 接口类型去掉指针
				}
			}

			result = finder(func(b *BeanDefinition) bool {
				return b.Type().AssignableTo(t)
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

	// 恰好 1 个 & 仅供查询无需绑定
	return result[0], true
}

// FindBeanByName 根据名称和类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
func (ctx *defaultSpringContext) FindBeanByName(beanId string) (*BeanDefinition, bool) {
	return ctx.FindBean(beanId)
}

// CollectBeans 收集数组或指针定义的所有符合条件的 Bean 对象，收集到返回 true，否则返回 false。
func (ctx *defaultSpringContext) CollectBeans(i interface{}, watchers ...WiringWatcher) bool {
	ctx.checkAutoWired()

	t := reflect.TypeOf(i)

	if t.Kind() != reflect.Ptr {
		panic(errors.New("i must be slice ptr"))
	}

	et := t.Elem()

	if et.Kind() != reflect.Slice {
		panic(errors.New("i must be slice ptr"))
	}

	w := newDefaultBeanAssembly(ctx, watchers)
	return w.collectBeans(reflect.ValueOf(i).Elem())
}

// findCacheItem 查找指定类型的缓存项
func (ctx *defaultSpringContext) findCacheItem(t reflect.Type) *beanCacheItem {
	c, _ := ctx.beanCache.LoadOrStore(t, newBeanCacheItem())
	return c.(*beanCacheItem)
}

// autoExport 自动导出 Bean 实现的接口
func (ctx *defaultSpringContext) autoExport(bd *BeanDefinition, t reflect.Type) {
	for i := 0; i < t.NumField(); i++ {
		if f := t.Field(i); f.Anonymous && f.Type.Kind() == reflect.Interface {
			m := ctx.findCacheItem(f.Type)
			m.store(f.Type, bd, false)
		}
	}
}

// resolveBean 对 Bean 进行决议是否能够创建 Bean 的实例
func (ctx *defaultSpringContext) resolveBean(bd *BeanDefinition) {

	if bd.status > beanStatus_Default {
		return
	}

	bd.status = beanStatus_Resolving

	// 如果是成员方法 Bean，需要首先决议它的父 Bean 是否能实例化
	if mBean, ok := bd.bean.(*methodBean); ok {
		ctx.resolveBean(mBean.parent)

		// 父 Bean 已经被删除了，子 Bean 也不应该存在
		if mBean.parent.status == beanStatus_Deleted {
			ctx.deleteBeanDefinition(bd)
			return
		}
	}

	if ok := bd.Matches(ctx); !ok { // 不满足则删除注册
		ctx.deleteBeanDefinition(bd)
		return
	}

	// 将符合注册条件的 Bean 放入到缓存里面
	item := ctx.findCacheItem(bd.Type())
	item.store(bd.Type(), bd, false)

	// 自动导出接口，这种情况下应该只对于结构体才会有效
	if bd.autoExport {
		t := SpringUtils.Indirect(bd.Type())
		if t.Kind() == reflect.Struct {
			ctx.autoExport(bd, t)
		}
	}

	// 按照导出类型放入缓存
	for _, t := range bd.exports {

		// 检查是否实现了导出接口
		if ok := bd.Type().Implements(t); !ok {
			panic(fmt.Errorf("%s not implement %s interface", bd.Description(), t.String()))
		}

		m := ctx.findCacheItem(t)
		m.store(t, bd, false)
	}

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
		case *BeanDefinition:
			selector = e.BeanId()
			result = append(result, e)
		case string:
			selector = e
			typeName, beanName, _ := ParseBeanId(e)
			filter = func(b *BeanDefinition) bool {
				return b.Match(typeName, beanName)
			}
		default:
			t := reflect.TypeOf(e)
			// 如果是接口类型需要解除外层指针
			if t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Interface {
				t = t.Elem()
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

		bd.bean = newMethodBean(result[0], bean.method, bean.tags...)
		if bd.name == "" { // 使用默认名称
			bd.name = bd.bean.Type().String()
		}
		ctx.registerBeanDefinition(bd)
	}
}

// AutoWireBeans 完成自动绑定
func (ctx *defaultSpringContext) AutoWireBeans(watchers ...WiringWatcher) {

	// 不再接受 Bean 注册，因为性能的原因使用了缓存，并且在 AutoWireBeans 的过程中
	// 逐步建立起这个缓存，而随着缓存的建立，绑定的速度会越来越快，从而减少性能的损失。

	if ctx.autoWired {
		panic(errors.New("ctx.AutoWireBeans() already called"))
	}

	// 注册所有的 Method Bean
	ctx.registerMethodBeans()

	ctx.autoWired = true

	if ctx.eventNotify != nil {
		ctx.eventNotify(ContextEvent_ResolveStart)
	}

	// 对 config 函数进行决议
	for e := ctx.configers.Front(); e != nil; e = e.Next() {
		configer := e.Value.(*Configer)
		if ok := configer.Matches(ctx); !ok {
			ctx.configers.Remove(e)
		}
	}

	// 对 config 函数进行排序
	ctx.configers = sortConfigers(ctx.configers)

	// 首先决议 Bean 是否能够注册，否则会删除其注册信息
	for _, bd := range ctx.beanMap {
		ctx.resolveBean(bd)
	}

	if ctx.eventNotify != nil {
		ctx.eventNotify(ContextEvent_ResolveEnd)
	}

	w := newDefaultBeanAssembly(ctx, watchers)

	if ctx.eventNotify != nil {
		ctx.eventNotify(ContextEvent_AutoWireStart)
	}

	defer func() { // 捕获自动注入过程中的异常，打印错误日志然后重新抛出
		if err := recover(); err != nil {
			SpringLogger.Errorf("%v ↩\n%s", err, w.wiringStack.path())
			panic(err)
		}
	}()

	// 执行配置函数，过程中会自动完成部分注入
	for e := ctx.configers.Front(); e != nil; e = e.Next() {
		configer := e.Value.(*Configer)
		configer.run(ctx)
	}

	if ctx.Sort { // 自动注入期间是否排序注入
		beanKeyMap := map[string]beanKey{}
		for key, val := range ctx.beanMap {
			beanKeyMap[val.BeanId()] = key
		}

		beanIds := make([]string, 0)
		for s := range beanKeyMap {
			beanIds = append(beanIds, s)
		}

		sort.Strings(beanIds)

		for _, beanId := range beanIds {
			key := beanKeyMap[beanId]
			bd := ctx.beanMap[key]
			w.wireBeanDefinition(bd, false)
		}

	} else {
		for _, bd := range ctx.beanMap {
			w.wireBeanDefinition(bd, false)
		}
	}

	if ctx.eventNotify != nil {
		ctx.eventNotify(ContextEvent_AutoWireEnd)
	}
}

// WireBean 绑定外部的 Bean 源
func (ctx *defaultSpringContext) WireBean(bean interface{}, watchers ...WiringWatcher) {
	ctx.checkAutoWired()

	w := newDefaultBeanAssembly(ctx, watchers)
	bd := ToBeanDefinition("", bean)
	w.wireBeanDefinition(bd, false)
}

// GetBeanDefinitions 获取所有 Bean 的定义，一般仅供调试使用。
func (ctx *defaultSpringContext) GetBeanDefinitions() []*BeanDefinition {
	result := make([]*BeanDefinition, 0)
	for _, v := range ctx.beanMap {
		result = append(result, v)
	}
	return result
}

// Close 关闭容器上下文，用于通知 Bean 销毁等。
func (ctx *defaultSpringContext) Close() {

	if ctx.eventNotify != nil {
		ctx.eventNotify(ContextEvent_CloseStart)
	}

	// 执行销毁函数
	for _, bd := range ctx.beanMap {
		if bd.destroy != nil {
			fnValue := reflect.ValueOf(bd.destroy)
			fnValue.Call([]reflect.Value{bd.Value()})
		}
	}

	// 上下文结束
	ctx.cancel()

	if ctx.eventNotify != nil {
		ctx.eventNotify(ContextEvent_CloseEnd)
	}
}

// Run 立即执行一个一次性的任务
func (ctx *defaultSpringContext) Run(fn interface{}, tags ...string) *Runner {
	ctx.checkAutoWired()
	return newRunner(ctx, fn, tags)
}

// Config 注册一个配置函数
func (ctx *defaultSpringContext) Config(fn interface{}, tags ...string) *Configer {
	configer := newConfiger("", fn, tags)
	ctx.configers.PushBack(configer)
	return configer
}

// ConfigWithName 注册一个配置函数，name 的作用：区分，排重，排顺序。
func (ctx *defaultSpringContext) ConfigWithName(name string, fn interface{}, tags ...string) *Configer {
	configer := newConfiger(name, fn, tags)
	ctx.configers.PushBack(configer)
	return configer
}
