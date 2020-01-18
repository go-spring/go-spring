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

	"github.com/go-spring/go-spring-parent/spring-logger"
	"github.com/go-spring/go-spring-parent/spring-utils"
)

// beanKey Bean's unique key, type+name.
type beanKey struct {
	rType reflect.Type
	name  string
}

// beanCacheItem BeanCache's item.
type beanCacheItem struct {
	// Different typed beans implemented same interface maybe
	// have same name, so name can't be a map's key. therefor
	// we use a list to store the cached beans.
	named []*BeanDefinition

	// 收集模式得到的 Bean 列表，一个类型只需收集一次。
	collect reflect.Value
}

// newBeanCacheItem beanCacheItem 的构造函数
func newBeanCacheItem() *beanCacheItem {
	return &beanCacheItem{
		named: make([]*BeanDefinition, 0),
	}
}

// Store 将一个 Bean 存储到 CachedBeanMapItem 里
func (item *beanCacheItem) store(d *BeanDefinition) {
	item.named = append(item.named, d)
}

// StoreCollect 将收集到的 Bean 列表的值存储到 CachedBeanMapItem 里
func (item *beanCacheItem) storeCollect(v reflect.Value) {
	item.collect = v
}

// wiringStack 存储正在进行绑定的 Bean
type wiringStack struct {
	l *list.List
}

// newWiringStack wiringStack 的构造函数
func newWiringStack() wiringStack {
	return wiringStack{
		l: list.New(),
	}
}

// pushBack 添加一个 Item 到尾部
func (s *wiringStack) pushBack(bd beanDefinition) {
	s.l.PushBack(bd)
}

// popBack 删除尾部的 item
func (s *wiringStack) popBack() {
	s.l.Remove(s.l.Back())
}

// path 返回依赖注入的路径
func (s *wiringStack) path() (path string) {
	for e := s.l.Front(); e != nil; e = e.Next() {
		w := e.Value.(beanDefinition)
		path += fmt.Sprintf("=> %s ↩\n", w.description())
	}
	return path
}

// circle 检测到循环依赖后抛出 panic
func (s *wiringStack) circle(bd beanDefinition) {
	str := "found circle autowire: ↩\n"
	str += s.path()
	str += fmt.Sprintf("=> %s ↩", bd.description())
	SpringLogger.Panic(str)
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

	beanMap   map[beanKey]*BeanDefinition     // Bean 的集合
	beanCache map[reflect.Type]*beanCacheItem // Bean 的缓存

	wiringStack wiringStack // 保存正在进行绑定的 Bean 列表

	methodBeans []*BeanDefinition // 延迟创建的 Method Bean 的集合
}

// NewDefaultSpringContext defaultSpringContext 的构造函数
func NewDefaultSpringContext() *defaultSpringContext {
	ctx, cancel := context.WithCancel(context.Background())
	return &defaultSpringContext{
		Context:     ctx,
		cancel:      cancel,
		wiringStack: newWiringStack(),
		Properties:  NewDefaultProperties(),
		methodBeans: make([]*BeanDefinition, 0),
		beanMap:     make(map[beanKey]*BeanDefinition),
		beanCache:   make(map[reflect.Type]*beanCacheItem),
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
func (ctx *defaultSpringContext) RegisterMethodBean(selector interface{}, method string, tags ...string) *BeanDefinition {
	return ctx.RegisterNameMethodBean("", selector, method, tags...)
}

// RegisterNameMethodBean 注册成员方法单例 Bean，需指定名称，重复注册会 panic。
// selector 可以是 *BeanDefinition，可以是 BeanId，还可以是 (Type)(nil) 变量。
func (ctx *defaultSpringContext) RegisterNameMethodBean(name string, selector interface{}, method string, tags ...string) *BeanDefinition {

	if selector == nil {
		panic(errors.New("selector can't be nil"))
	}

	if ctx.autoWired { // 注册已被冻结
		SpringLogger.Panic("bean registration frozen")
	}

	bd := MethodToBeanDefinition(name, selector, method, tags...)
	ctx.methodBeans = append(ctx.methodBeans, bd)
	return bd
}

// registerBeanDefinition 注册单例 BeanDefinition，重复注册会 panic。
func (ctx *defaultSpringContext) registerBeanDefinition(d *BeanDefinition) {

	if ctx.autoWired { // 注册已被冻结
		SpringLogger.Panic("bean registration frozen")
	}

	k := beanKey{
		rType: d.Type(),
		name:  d.name,
	}

	if _, ok := ctx.beanMap[k]; ok {
		SpringLogger.Panic("duplicate bean registration " + d.BeanId())
	}

	ctx.beanMap[k] = d
}

// GetBean 根据类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
func (ctx *defaultSpringContext) GetBean(i interface{}) bool {
	return ctx.GetBeanByName("?", i)
}

// GetBeanByName 根据名称和类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
func (ctx *defaultSpringContext) GetBeanByName(beanId string, i interface{}) bool {

	if !ctx.autoWired {
		SpringLogger.Panic("should call after ctx.AutoWireBeans()")
	}

	// 确保存在可空标记，抑制 panic 效果。
	if beanId == "" || beanId[len(beanId)-1] != '?' {
		beanId += "?"
	}

	t := reflect.TypeOf(i)

	// 使用指针才能够对外赋值
	if t.Kind() != reflect.Ptr {
		SpringLogger.Panic("i must be pointer")
	}

	v := reflect.ValueOf(i)

	return ctx.getBeanValue(beanId, reflect.Value{}, v.Elem(), "")
}

// findCacheItem 查找指定类型的缓存项
func (ctx *defaultSpringContext) findCacheItem(t reflect.Type) (*beanCacheItem, bool) {
	c, ok := ctx.beanCache[t]
	if !ok {
		c = newBeanCacheItem()
		ctx.beanCache[t] = c
	}
	return c, ok
}

// getBeanValue 根据 BeanId 查找 Bean 并返回 Bean 源的值
func (ctx *defaultSpringContext) getBeanValue(beanId string, parentValue reflect.Value, beanValue reflect.Value, field string) bool {
	typeName, beanName, nullable := ParseBeanId(beanId)
	beanType := beanValue.Type()

	if ok := IsRefType(beanType.Kind()); !ok {
		SpringLogger.Panic("receiver \"" + field + "\" must be ref type")
	}

	m, ok := ctx.findCacheItem(beanType)

	found := func(bean *BeanDefinition) bool {

		// 不能将自身赋给自身的字段
		if bean.Value() == parentValue {
			return false
		}

		// 类型必须相容
		return bean.Type().AssignableTo(beanType)
	}

	result := make([]*BeanDefinition, 0)

	checkResult := func() bool {
		count := len(result)

		// 没有找到
		if count == 0 {
			if nullable {
				return false
			} else {
				if field == "" {
					str := "没有找到符合条件的 Bean: ↩\n"
					str += ctx.wiringStack.path()
					str += beanValue.Type().String() + " ↩"
					SpringLogger.Panic(str)
				} else {
					SpringLogger.Panic(beanValue.Type().String() + " 没有找到符合条件的 Bean")
				}
			}
		}

		var primaryBean *BeanDefinition

		for _, bean := range result {
			if bean.primary {
				if primaryBean != nil {
					SpringLogger.Panic(field + " 找到多个 primary bean")
				}
				primaryBean = bean
			}
		}

		if primaryBean == nil {
			if count > 1 {
				SpringLogger.Panic(field + " 找到多个符合条件的值")
			}
			primaryBean = result[0]
		}

		// 依赖注入
		ctx.wireBeanDefinition(primaryBean, false)

		// 恰好 1 个
		v := SpringUtils.ValuePatchIf(beanValue, ctx.allAccess)
		v.Set(primaryBean.Value())
		return true
	}

	// 未命中缓存，则从注册列表里面查询，并更新缓存
	if !ok {

		for _, bean := range ctx.beanMap {
			if found(bean) {
				m.store(bean)
				if bean.Match(typeName, beanName) {
					result = append(result, bean)
				}
			}
		}

	} else { // 命中缓存，则从缓存中查询

		for _, bean := range m.named {
			if found(bean) && bean.Match(typeName, beanName) {
				result = append(result, bean)
			}
		}
	}

	return checkResult()
}

// 根据名称和类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
func (ctx *defaultSpringContext) FindBeanByName(beanId string) (*BeanDefinition, bool) {

	if !ctx.autoWired {
		SpringLogger.Panic("should call after ctx.AutoWireBeans()")
	}

	typeName, beanName, _ := ParseBeanId(beanId)

	var (
		count  int
		result *BeanDefinition
	)

	for _, bean := range ctx.beanMap {
		if bean.Match(typeName, beanName) {

			// 避免 Bean 还未解析
			ctx.resolveBean(bean)

			if bean.status != beanStatus_Deleted {
				result = bean
				count++
			}
		}
	}

	// 没有找到
	if count == 0 {
		return nil, false
	}

	// 多于 1 个
	if count > 1 {
		SpringLogger.Panic(beanId + " 找到多个符合条件的值")
	}

	// 恰好 1 个 & 仅供查询无需绑定
	return result, true
}

// CollectBeans 收集数组或指针定义的所有符合条件的 Bean 对象，收集到返回 true，否则返回 false。
func (ctx *defaultSpringContext) CollectBeans(i interface{}) bool {

	if !ctx.autoWired {
		SpringLogger.Panic("should call after ctx.AutoWireBeans()")
	}

	t := reflect.TypeOf(i)

	if t.Kind() != reflect.Ptr {
		SpringLogger.Panic("i must be slice ptr")
	}

	et := t.Elem()

	if et.Kind() != reflect.Slice {
		SpringLogger.Panic("i must be slice ptr")
	}

	return ctx.collectBeans(reflect.ValueOf(i).Elem())
}

// collectBeans 收集符合条件的 Bean 源
func (ctx *defaultSpringContext) collectBeans(v reflect.Value) bool {

	t := v.Type()
	et := t.Elem()

	m, ok := ctx.findCacheItem(t)

	// 未命中缓存，或者还没有收集到数据，则从注册列表里面查询，并更新缓存
	if !ok || !m.collect.IsValid() {

		// 创建一个空数组
		ev := reflect.New(t).Elem()

		for _, d := range ctx.beanMap {
			dt := d.Type()

			if dt.AssignableTo(et) { // Bean 自身符合条件
				ctx.wireBeanDefinition(d, false)
				ev = reflect.Append(ev, d.Value())

			} else if dt.Kind() == reflect.Slice { // 找到一个 Bean 数组
				if dt.Elem().AssignableTo(et) {

					// 数组扩容
					size := ev.Len() + d.Value().Len()
					newSlice := reflect.MakeSlice(t, size, size)

					reflect.Copy(newSlice, ev)

					// 拷贝新元素
					for i := 0; i < d.Value().Len(); i++ {
						di := d.Value().Index(i)

						if di.Kind() == reflect.Struct {
							bd := ValueToBeanDefinition("", di.Addr())
							ctx.wireBeanDefinition(bd, false)

						} else if di.Kind() == reflect.Ptr {
							if de := di.Elem(); de.Kind() == reflect.Struct {
								bd := ValueToBeanDefinition("", di)
								ctx.wireBeanDefinition(bd, false)
							}
						}

						newSlice.Index(i + ev.Len()).Set(di)
					}

					// 完成扩容
					ev = newSlice
				}
			}
		}

		// 把查询结果缓存起来
		m.storeCollect(ev)

		// 给外面的数组赋值
		if ev.Len() > 0 {
			v = SpringUtils.ValuePatchIf(v, ctx.allAccess)
			v.Set(ev)
			return true
		}
		return false
	}

	// 命中缓存，则从缓存中查询

	if m.collect.Len() > 0 {
		v = SpringUtils.ValuePatchIf(v, ctx.allAccess)
		v.Set(m.collect)
		return true
	}
	return false
}

// GetBeanValue 根据 beanId 获取符合条件的 Bean 对象，成功返回 true，否则返回 false。
func (ctx *defaultSpringContext) GetBeanValue(beanId string, v reflect.Value) bool {
	if !ctx.autoWired {
		SpringLogger.Panic("should call after ctx.AutoWireBeans()")
	}

	if _, beanName, _ := ParseBeanId(beanId); beanName == "[]" {
		return ctx.collectBeans(v)
	} else {
		return ctx.getBeanValue(beanId, reflect.Value{}, v, "")
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
			key := beanKey{bd.Type(), bd.name}
			bd.status = beanStatus_Deleted
			delete(ctx.beanMap, key)
			return
		}
	}

	if ok := bd.Matches(ctx); !ok { // 不满足则删除注册
		key := beanKey{bd.Type(), bd.name}
		bd.status = beanStatus_Deleted
		delete(ctx.beanMap, key)
		return
	}

	// 将符合注册条件的 Bean 放入到缓存里面
	SpringLogger.Debugf("register bean \"%s\" %s", bd.Name(), bd.Caller())
	item, _ := ctx.findCacheItem(bd.Type())
	item.store(bd)

	bd.status = beanStatus_Resolved
}

// AutoWireBeans 完成自动绑定
func (ctx *defaultSpringContext) AutoWireBeans() {

	// 不再接受 Bean 注册，因为性能的原因使用了缓存，并且在 AutoWireBeans 的过程中
	// 逐步建立起这个缓存，而随着缓存的建立，绑定的速度会越来越快，从而减少性能的损失。

	if ctx.autoWired {
		SpringLogger.Panic("ctx.AutoWireBeans() already called")
	}

	// 注册所有的 Method Bean
	for _, bd := range ctx.methodBeans {
		bean := bd.bean.(*fakeMethodBean)

		var (
			count  int
			parent *BeanDefinition
		)

		switch e := bean.selector.(type) {
		case *BeanDefinition:
			parent = e
		case string:
			{
				typeName, beanName, _ := ParseBeanId(e)
				for _, b := range ctx.beanMap {
					if b.Match(typeName, beanName) {
						parent = b
						count++
					}
				}

				if parent == nil {
					panic(errors.New("can't find parent bean \"" + e + "\""))
				}
			}
		default:
			{
				t := reflect.TypeOf(e) // 类型精确匹配
				for _, b := range ctx.beanMap {
					if b.Type() == t {
						parent = b
						count++
					}
				}

				if parent == nil {
					panic(errors.New("can't find parent bean \"" + t.String() + "\""))
				}
			}
		}

		bd.bean = newMethodBean(parent, bean.method, bean.tags...)
		if bd.name == "" { // 使用默认名称
			bd.name = bd.bean.Type().String()
		}
		ctx.registerBeanDefinition(bd)
	}

	ctx.autoWired = true

	// 首先决议 Bean 是否能够注册，否则会删除其注册信息
	for _, bd := range ctx.beanMap {
		ctx.resolveBean(bd)
	}

	// 然后执行 Bean 绑定
	for _, bd := range ctx.beanMap {
		ctx.wireBeanDefinition(bd, false)
	}
}

// WireBean 绑定外部的 Bean 源
func (ctx *defaultSpringContext) WireBean(bean interface{}) {
	if !ctx.autoWired {
		SpringLogger.Panic("should call after ctx.AutoWireBeans()")
	}
	bd := ToBeanDefinition("", bean)
	ctx.wireBeanDefinition(bd, false)
}

// fieldBeanDefinition 带字段名称的 BeanDefinition 实现
type fieldBeanDefinition struct {
	*BeanDefinition
	field string // 字段名称
}

// description 返回 Bean 的详细描述
func (d *fieldBeanDefinition) description() string {
	return fmt.Sprintf("%s field %s %s", d.bean.beanClass(), d.field, d.Caller())
}

// delegateBeanDefinition 代理功能的 BeanDefinition 实现
type delegateBeanDefinition struct {
	*BeanDefinition
	delegate beanDefinition // 代理项
}

// description 返回 Bean 的详细描述
func (d *delegateBeanDefinition) description() string {
	return fmt.Sprintf("%s value %s", d.delegate.springBean().beanClass(), d.delegate.Caller())
}

// wireBeanDefinition 绑定 BeanDefinition 指定的 Bean
func (ctx *defaultSpringContext) wireBeanDefinition(bd beanDefinition, onlyAutoWire bool) {

	// 是否已删除
	if bd.getStatus() == beanStatus_Deleted {
		SpringLogger.Panic(bd.BeanId() + " 已经被删除")
	}

	// 是否循环依赖
	if bd.getStatus() == beanStatus_Wiring {
		if _, ok := bd.springBean().(*originalBean); !ok {
			ctx.wiringStack.circle(bd)
		}
		return
	}

	// 是否已绑定
	if bd.getStatus() == beanStatus_Wired {
		return
	}

	defer SpringLogger.Debugf("wired %s", bd.description())
	SpringLogger.Debugf("wiring %s", bd.description())

	bd.setStatus(beanStatus_Wiring)

	// 将当前 Bean 放入注入栈，以便检测循环依赖。
	ctx.wiringStack.pushBack(bd)

	// 首先初始化当前 Bean 不直接依赖的那些 Bean
	for _, beanId := range bd.getDependsOn() {
		if bean, ok := ctx.FindBeanByName(beanId); !ok {
			SpringLogger.Panic(beanId + " 没有找到符合条件的 Bean")
		} else {
			ctx.wireBeanDefinition(bean, false)
		}
	}

	// 如果是成员方法 Bean，需要首先初始化它的父 Bean
	if mBean, ok := bd.springBean().(*methodBean); ok {
		ctx.wireBeanDefinition(mBean.parent, false)
	}

	switch bean := bd.springBean().(type) {
	case *originalBean: // 原始对象
		ctx.wireOriginalBean(bd, onlyAutoWire)
	case *constructorBean: // 构造函数
		ctx.wireFunctionBean(&bean.functionBean, bd)
	case *methodBean: // 成员方法
		ctx.wireFunctionBean(&bean.functionBean, bd)
	default:
		SpringLogger.Panic("unknown spring bean type")
	}

	// 如果有则执行用户设置的初始化函数
	if bd.getInit() != nil {
		fnValue := reflect.ValueOf(bd.getInit())
		fnValue.Call([]reflect.Value{bd.Value()})
	}

	// 删除保存的注入帧
	ctx.wiringStack.popBack()

	bd.setStatus(beanStatus_Wired)
}

// wireOriginalBean 对原始对象进行注入
func (ctx *defaultSpringContext) wireOriginalBean(bd beanDefinition, onlyAutoWire bool) {

	st := bd.Type()
	sk := st.Kind()

	if sk == reflect.Slice { // 处理数组 Bean
		et := st.Elem()
		ek := et.Kind()

		if ek == reflect.Struct { // 结构体数组
			v := bd.Value()
			for i := 0; i < v.Len(); i++ {
				iv := v.Index(i).Addr()
				bd := ValueToBeanDefinition("", iv)
				ctx.wireBeanDefinition(bd, false)
			}

		} else if ek == reflect.Ptr { // 指针数组
			it := et.Elem()
			ik := it.Kind()

			if ik == reflect.Struct { // 结构体指针数组
				v := bd.Value()
				for p := 0; p < v.Len(); p++ {
					pv := v.Index(p)
					bd := ValueToBeanDefinition("", pv)
					ctx.wireBeanDefinition(bd, false)
				}
			}
		}

	} else if sk == reflect.Ptr { // 处理指针 Bean
		et := st.Elem()
		if et.Kind() == reflect.Struct { // 结构体指针

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
						bindStructField(ctx, ft.Type, fv, fieldName, "", tag, ctx.allAccess)
					}
				}

				// 处理 autowire 标签
				if beanId, ok := ft.Tag.Lookup("autowire"); ok {
					ctx.wireStructField(sv, fv, fieldName, beanId)
				}

				// 处理结构体类型的字段，防止递归所以不支持指针结构体字段
				if ft.Type.Kind() == reflect.Struct {
					// 开放私有字段，但是不会更新原属性
					fv0 := SpringUtils.ValuePatchIf(fv, ctx.allAccess)
					if fv0.CanSet() {

						b := ValueToBeanDefinition("", fv0.Addr())
						b.file = bd.getFile()
						b.line = bd.getLine()
						fbd := &fieldBeanDefinition{b, fieldName}
						ctx.wireBeanDefinition(fbd, fieldOnlyAutoWire)
					}
				}
			}
		}
	}
}

// wireFunctionBean 对函数定义 Bean 进行注入
func (ctx *defaultSpringContext) wireFunctionBean(bean *functionBean, bd beanDefinition) {

	in := bean.arg.Get(bd, ctx)
	out := bean.fnValue.Call(in)

	// 获取第一个返回值
	val := out[0]

	// 检查是否有 error 返回
	if len(out) == 2 {
		if err := out[1].Interface(); err != nil {
			SpringLogger.Panic("function bean", bd.Caller(), "return error:", err)
		}
	}

	if IsRefType(val.Kind()) {
		// 如果实现接口的值是个结构体，那么需要转换成指针类型然后赋给接口
		if val.Kind() == reflect.Interface && val.Elem().Kind() == reflect.Struct {
			ptrVal := reflect.New(val.Elem().Type())
			ptrVal.Elem().Set(val.Elem())
			bean.rValue.Set(ptrVal)
		} else {
			bean.rValue.Set(val)
		}
	} else {
		bean.rValue.Elem().Set(val)
	}

	if bean.bean = bean.rValue.Interface(); bean.bean == nil {
		SpringLogger.Panic("function bean", bd.Caller(), "return nil")
	}

	// 对返回值进行依赖注入
	b := &BeanDefinition{
		name:   bd.Name(),
		status: beanStatus_Default,
		file:   bd.getFile(),
		line:   bd.getLine(),
	}

	if bean.Type().Kind() == reflect.Interface {
		b.bean = newOriginalBean(bean.Value().Elem())
	} else {
		b.bean = newOriginalBean(bean.Value())
	}

	ctx.wireBeanDefinition(&delegateBeanDefinition{b, bd}, false)
}

// wireStructField 对结构体的字段进行绑定
func (ctx *defaultSpringContext) wireStructField(parentValue reflect.Value,
	beanValue reflect.Value, field string, beanId string) {

	_, beanName, nullable := ParseBeanId(beanId)
	if beanName == "[]" { // 收集模式，autowire:"[]"

		// 收集模式的绑定对象必须是数组
		if beanValue.Type().Kind() != reflect.Slice {
			SpringLogger.Panic(field + " must be slice when autowire []")
		}

		ok := ctx.collectBeans(beanValue)
		if !ok && !nullable { // 没找到且不能为空则 panic
			SpringLogger.Panic(field + " 没有找到符合条件的 Bean")
		}

	} else { // 匹配模式，autowire:"" or autowire:"name"
		ctx.getBeanValue(beanId, parentValue, beanValue, field)
	}
}

// GetAllBeanDefinitions 获取所有 Bean 的定义，一般仅供调试使用。
func (ctx *defaultSpringContext) GetAllBeanDefinitions() []*BeanDefinition {
	result := make([]*BeanDefinition, 0)
	for _, v := range ctx.beanMap {
		result = append(result, v)
	}
	return result
}

// Close 关闭容器上下文，用于通知 Bean 销毁等。
func (ctx *defaultSpringContext) Close() {

	// 执行销毁函数
	for _, bd := range ctx.beanMap {
		if bd.destroy != nil {
			fnValue := reflect.ValueOf(bd.destroy)
			fnValue.Call([]reflect.Value{bd.Value()})
		}
	}

	// 上下文结束
	ctx.cancel()
}
