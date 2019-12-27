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
	"errors"
	"fmt"
	"reflect"
)

// beanKey Bean's unique key, type+name.
type beanKey struct {
	Type reflect.Type
	Name string
}

// beanCacheItem BeanCache's item.
type beanCacheItem struct {
	// Different typed beans implemented same interface maybe
	// have same name, so name can't be a map's key. therefor
	// we use a list to store the cached beans.
	Named []*BeanDefinition

	// 收集模式得到的 Bean 列表，一个类型只需收集一次。
	Collect reflect.Value
}

// newBeanCacheItem beanCacheItem 的构造函数
func newBeanCacheItem() *beanCacheItem {
	return &beanCacheItem{
		Named: make([]*BeanDefinition, 0),
	}
}

// Store 将一个 Bean 存储到 CachedBeanMapItem 里
func (item *beanCacheItem) Store(d *BeanDefinition) {
	if d.name == "" {
		panic(errors.New("bean must have name"))
	}
	item.Named = append(item.Named, d)
}

// StoreCollect 将收集到的 Bean 列表的值存储到 CachedBeanMapItem 里
func (item *beanCacheItem) StoreCollect(v reflect.Value) {
	item.Collect = v
}

// wiringItem wiringStack 的 Item
type wiringItem struct {
	bean  *BeanDefinition
	field string // 字段名，可能为空
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
func (s *wiringStack) pushBack(bean *BeanDefinition) {
	s.l.PushBack(&wiringItem{
		bean: bean,
	})
}

// setField 设置最后一个元素的 field 字段值
func (s *wiringStack) setField(field string) {
	if e := s.l.Back(); e != nil {
		w := e.Value.(*wiringItem)
		w.field = field
	}
}

// popBack 删除尾部的 item
func (s *wiringStack) popBack() {
	s.l.Remove(s.l.Back())
}

// panic 检测到循环依赖后抛出 panic
func (s *wiringStack) panic(bean *BeanDefinition) {
	err := "found circle autowire: "
	for e := s.l.Front(); e != nil; e = e.Next() {
		w := e.Value.(*wiringItem)
		if w.field == "" {
			err += w.bean.name + " => "
		} else {
			err += w.bean.name + ":$" + w.field + " => "
		}
	}
	err += bean.name
	panic(errors.New(err))
}

// defaultSpringContext SpringContext 的默认版本
type defaultSpringContext struct {
	// 属性值列表接口
	Properties

	profile   string // 运行环境
	autoWired bool   // 已经开始自动绑定

	beanMap   map[beanKey]*BeanDefinition     // Bean 的集合
	beanCache map[reflect.Type]*beanCacheItem // Bean 的缓存

	wiringStack wiringStack // 保存正在进行绑定的 Bean 列表
}

// NewDefaultSpringContext defaultSpringContext 的构造函数
func NewDefaultSpringContext() *defaultSpringContext {
	return &defaultSpringContext{
		wiringStack: newWiringStack(),
		Properties:  NewDefaultProperties(),
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

// RegisterBean 注册单例 Bean，不指定名称，重复注册会 panic。
func (ctx *defaultSpringContext) RegisterBean(bean interface{}) *BeanDefinition {
	return ctx.RegisterNameBean("", bean)
}

// RegisterNameBean 注册单例 Bean，需要指定名称，重复注册会 panic。
func (ctx *defaultSpringContext) RegisterNameBean(name string, bean interface{}) *BeanDefinition {
	beanDefinition := ToBeanDefinition(name, bean)
	ctx.registerBeanDefinition(beanDefinition)
	return beanDefinition
}

// RegisterBeanFn 注册单例构造函数 Bean，不指定名称，重复注册会 panic。
func (ctx *defaultSpringContext) RegisterBeanFn(fn interface{}, tags ...string) *BeanDefinition {
	return ctx.RegisterNameBeanFn("", fn, tags...)
}

// RegisterNameBeanFn 注册单例构造函数 Bean，需指定名称，重复注册会 panic。
func (ctx *defaultSpringContext) RegisterNameBeanFn(name string, fn interface{}, tags ...string) *BeanDefinition {
	beanDefinition := FnToBeanDefinition(name, fn, tags...)
	ctx.registerBeanDefinition(beanDefinition)
	return beanDefinition
}

// RegisterMethodBean 注册成员方法单例 Bean，不指定名称，重复注册会 panic。
func (ctx *defaultSpringContext) RegisterMethodBean(parent *BeanDefinition, method string, tags ...string) *BeanDefinition {
	return ctx.RegisterNameMethodBean("", parent, method, tags...)
}

// RegisterNameMethodBean 注册成员方法单例 Bean，需指定名称，重复注册会 panic。
func (ctx *defaultSpringContext) RegisterNameMethodBean(name string, parent *BeanDefinition, method string, tags ...string) *BeanDefinition {
	beanDefinition := MethodToBeanDefinition(name, parent, method, tags...)
	ctx.registerBeanDefinition(beanDefinition)
	return beanDefinition
}

// registerBeanDefinition 注册单例 BeanDefinition，重复注册会 panic。
func (ctx *defaultSpringContext) registerBeanDefinition(d *BeanDefinition) {

	if ctx.autoWired { // 注册已被冻结
		panic(errors.New("bean registration frozen"))
	}

	k := beanKey{
		Type: d.Type(),
		Name: d.name,
	}

	if _, ok := ctx.beanMap[k]; ok {
		panic(errors.New("重复注册 " + d.BeanId()))
	}

	ctx.beanMap[k] = d
}

// GetBean 根据类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
func (ctx *defaultSpringContext) GetBean(i interface{}) bool {
	return ctx.GetBeanByName("?", i)
}

// GetBeanValue 根据名称和类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
func (ctx *defaultSpringContext) GetBeanValue(beanId string, v reflect.Value) bool {
	return ctx.getBeanValue(beanId, reflect.Value{}, v, "")
}

// GetBeanByName 根据名称和类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
func (ctx *defaultSpringContext) GetBeanByName(beanId string, i interface{}) bool {

	if !ctx.autoWired {
		panic(errors.New("should call after ctx.AutoWireBeans()"))
	}

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
		panic(errors.New("receiver \"" + field + "\" must be ref type"))
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
				panic(errors.New(field + " 没有找到符合条件的 Bean"))
			}
		}

		var primaryBean *BeanDefinition

		for _, bean := range result {
			if bean.primary {
				if primaryBean != nil {
					panic(errors.New(field + " 找到多个 primary bean"))
				}
				primaryBean = bean
			}
		}

		if primaryBean == nil {
			if count > 1 {
				panic(errors.New(field + " 找到多个符合条件的值"))
			}
			primaryBean = result[0]
		}

		// 依赖注入
		ctx.wireBeanDefinition(primaryBean)

		// 恰好 1 个
		beanValue.Set(primaryBean.Value())
		return true
	}

	// 未命中缓存，则从注册列表里面查询，并更新缓存
	if !ok {

		for _, bean := range ctx.beanMap {
			if found(bean) {
				m.Store(bean)
				if bean.Match(typeName, beanName) {
					result = append(result, bean)
				}
			}
		}

	} else { // 命中缓存，则从缓存中查询

		for _, bean := range m.Named {
			if found(bean) && bean.Match(typeName, beanName) {
				result = append(result, bean)
			}
		}
	}

	return checkResult()
}

// CollectBeans 收集数组或指针定义的所有符合条件的 Bean 对象，收集到返回 true，否则返回 false。
func (ctx *defaultSpringContext) CollectBeans(i interface{}) bool {

	if !ctx.autoWired {
		panic(errors.New("should call after ctx.AutoWireBeans()"))
	}

	t := reflect.TypeOf(i)

	if t.Kind() != reflect.Ptr {
		panic(errors.New("i must be slice ptr"))
	}

	et := t.Elem()

	if et.Kind() != reflect.Slice {
		panic(errors.New("i must be slice ptr"))
	}

	return ctx.collectBeans(reflect.ValueOf(i).Elem())
}

// collectBeans 收集符合条件的 Bean 源
func (ctx *defaultSpringContext) collectBeans(v reflect.Value) bool {

	t := v.Type()
	et := t.Elem()

	m, ok := ctx.findCacheItem(t)

	// 未命中缓存，或者还没有收集到数据，则从注册列表里面查询，并更新缓存
	if !ok || !m.Collect.IsValid() {

		// 创建一个空数组
		ev := reflect.New(t).Elem()

		for _, d := range ctx.beanMap {
			dt := d.Type()

			if dt.AssignableTo(et) { // Bean 自身符合条件
				ctx.wireBeanDefinition(d)
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
							ctx.WireBean(di.Addr().Interface())
						} else if di.Kind() == reflect.Ptr {
							if de := di.Elem(); de.Kind() == reflect.Struct {
								ctx.WireBean(di.Interface())
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
		m.StoreCollect(ev)

		// 给外面的数组赋值
		if ev.Len() > 0 {
			v.Set(ev)
			return true
		}
		return false
	}

	// 命中缓存，则从缓存中查询

	if m.Collect.Len() > 0 {
		v.Set(m.Collect)
		return true
	}
	return false
}

// 根据名称和类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
func (ctx *defaultSpringContext) FindBeanByName(beanId string) (*BeanDefinition, bool) {

	if !ctx.autoWired {
		panic(errors.New("should call after ctx.AutoWireBeans()"))
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
		panic(errors.New(beanId + " 找到多个符合条件的值"))
	}

	// 恰好 1 个 & 仅供查询无需绑定
	return result, true
}

// resolveBean 对 Bean 进行决议是否能够创建 Bean 的实例
func (ctx *defaultSpringContext) resolveBean(beanDefinition *BeanDefinition) {

	if beanDefinition.status > beanStatus_Default {
		return
	}

	beanDefinition.status = beanStatus_Resolving

	// 如果是成员方法 Bean，需要首先决议它的父 Bean 是否能实例化
	if mBean, ok := beanDefinition.SpringBean.(*methodBean); ok {
		ctx.resolveBean(mBean.parent)

		// 父 Bean 已经被删除了，子 Bean 也不应该存在
		if mBean.parent.status == beanStatus_Deleted {
			key := beanKey{beanDefinition.Type(), beanDefinition.name}
			beanDefinition.status = beanStatus_Deleted
			delete(ctx.beanMap, key)
			return
		}
	}

	if ok := beanDefinition.Matches(ctx); !ok { // 不满足则删除注册
		key := beanKey{beanDefinition.Type(), beanDefinition.name}
		beanDefinition.status = beanStatus_Deleted
		delete(ctx.beanMap, key)
		return
	}

	// 将符合注册条件的 Bean 放入到缓存里面
	fmt.Printf("register bean \"%s\" %s:%d\n", beanDefinition.BeanId(), beanDefinition.file, beanDefinition.line)
	item, _ := ctx.findCacheItem(beanDefinition.Type())
	item.Store(beanDefinition)

	beanDefinition.status = beanStatus_Resolved
}

// AutoWireBeans 完成自动绑定
func (ctx *defaultSpringContext) AutoWireBeans() {

	// 不再接受 Bean 注册，因为性能的原因使用了缓存，并且在 AutoWireBeans 的过程中
	// 逐步建立起这个缓存，而随着缓存的建立，绑定的速度会越来越快，从而减少性能的损失。

	if ctx.autoWired {
		panic(errors.New("ctx.AutoWireBeans() already called"))
	}

	ctx.autoWired = true

	// 首先决议 Bean 是否能够注册，否则会删除其注册信息
	for _, beanDefinition := range ctx.beanMap {
		ctx.resolveBean(beanDefinition)
	}

	// 然后执行 Bean 绑定
	for _, beanDefinition := range ctx.beanMap {
		ctx.wireBeanDefinition(beanDefinition)
	}
}

// WireBean 绑定外部的 Bean 源
func (ctx *defaultSpringContext) WireBean(bean interface{}) {

	if !ctx.autoWired {
		panic(errors.New("should call after ctx.AutoWireBeans()"))
	}

	beanDefinition := ToBeanDefinition("", bean)
	ctx.wireBeanDefinition(beanDefinition)
}

// wireBeanDefinition 绑定 BeanDefinition 指定的 Bean
func (ctx *defaultSpringContext) wireBeanDefinition(beanDefinition *BeanDefinition) {

	// 是否已删除
	if beanDefinition.status == beanStatus_Deleted {
		panic(errors.New(beanDefinition.BeanId() + " 已经被删除"))
	}

	// 是否循环依赖
	if beanDefinition.status == beanStatus_Wiring {
		if _, ok := beanDefinition.SpringBean.(*originalBean); !ok {
			ctx.wiringStack.panic(beanDefinition)
		}
		return
	}

	// 是否已绑定
	if beanDefinition.status == beanStatus_Wired {
		return
	}

	beanDefinition.status = beanStatus_Wiring

	// 将当前 Bean 放入注入栈，以便检测循环依赖。
	ctx.wiringStack.pushBack(beanDefinition)

	// 首先初始化当前 Bean 不直接依赖的那些 Bean
	for _, beanId := range beanDefinition.dependsOn {
		if bean, ok := ctx.FindBeanByName(beanId); !ok {
			panic(errors.New(beanId + " 没有找到符合条件的 Bean"))
		} else {
			ctx.wireBeanDefinition(bean)
		}
	}

	// 如果是成员方法 Bean，需要首先初始化它的父 Bean
	if mBean, ok := beanDefinition.SpringBean.(*methodBean); ok {
		ctx.wireBeanDefinition(mBean.parent)
	}

	switch beanDefinition.SpringBean.(type) {
	case *originalBean: // 原始对象
		ctx.wireOriginalBean(beanDefinition)
	case *constructorBean: // 构造函数
		ctx.wireConstructorBean(beanDefinition)
	case *methodBean: // 成员方法
		ctx.wireMethodBean(beanDefinition)
	default:
		panic(errors.New("unknown spring bean type"))
	}

	// 如果有则执行用户设置的初始化函数
	if beanDefinition.initFunc != nil {
		fnValue := reflect.ValueOf(beanDefinition.initFunc)
		fnValue.Call([]reflect.Value{beanDefinition.Value()})
	}

	// 删除保存的注入帧
	ctx.wiringStack.popBack()

	beanDefinition.status = beanStatus_Wired
}

// wireOriginalBean 对原始对象进行注入
func (ctx *defaultSpringContext) wireOriginalBean(beanDefinition *BeanDefinition) {
	st := beanDefinition.Type()
	sk := st.Kind()

	if sk == reflect.Slice { // 处理数组 Bean
		et := st.Elem()
		ek := et.Kind()

		if ek == reflect.Struct { // 结构体数组
			v := beanDefinition.Value()
			for i := 0; i < v.Len(); i++ {
				iv := v.Index(i).Addr()
				ctx.WireBean(iv.Interface())
			}

		} else if ek == reflect.Ptr { // 指针数组
			it := et.Elem()
			ik := it.Kind()

			if ik == reflect.Struct { // 结构体指针数组
				v := beanDefinition.Value()
				for p := 0; p < v.Len(); p++ {
					pv := v.Index(p)
					ctx.WireBean(pv.Interface())
				}
			}
		}

	} else if sk == reflect.Ptr { // 处理指针 Bean
		et := st.Elem()
		if et.Kind() == reflect.Struct { // 结构体指针
			fmt.Printf("wire bean \"%s\"\n", beanDefinition.BeanId())

			sv := beanDefinition.Value()
			ev := sv.Elem()

			for i := 0; i < et.NumField(); i++ {
				ft := et.Field(i)
				fv := ev.Field(i)

				fieldName := et.Name() + ".$" + ft.Name

				// 处理 value 标签
				if tag, ok := ft.Tag.Lookup("value"); ok {
					bindStructField(ctx, ft.Type, fv, fieldName, "", tag)
				} else {
					if ft.Type.Kind() == reflect.Struct {
						bindStruct(ctx, ft.Type, fv, fieldName, "")
					}
				}

				// 处理 autowire 标签
				if beanId, ok := ft.Tag.Lookup("autowire"); ok {
					ctx.wiringStack.setField(ft.Name)
					ctx.wireStructField(sv, fv, fieldName, beanId)
				}
			}

			fmt.Printf("success wire bean \"%s\"\n", beanDefinition.BeanId())
		}
	}
}

// wireConstructorBean 对构造函数 Bean 进行注入
func (ctx *defaultSpringContext) wireConstructorBean(beanDefinition *BeanDefinition) {
	bean := beanDefinition.SpringBean.(*constructorBean)
	ctx.wireFunctionBean(&bean.functionBean, beanDefinition)
	fmt.Printf("success wire constructor bean \"%s\"\n", beanDefinition.BeanId())
}

// wireMethodBean 对成员方法 Bean 进行注入
func (ctx *defaultSpringContext) wireMethodBean(beanDefinition *BeanDefinition) {
	bean := beanDefinition.SpringBean.(*methodBean)
	ctx.wireFunctionBean(&bean.functionBean, beanDefinition)
	fmt.Printf("success wire method bean \"%s\"\n", beanDefinition.BeanId())
}

// wireFunctionBean 对函数定义 Bean 进行注入
func (ctx *defaultSpringContext) wireFunctionBean(bean *functionBean, beanDefinition *BeanDefinition) {
	fnType := bean.fnValue.Type()

	in := bean.arg.Get(ctx, fnType)
	out := bean.fnValue.Call(in)

	// 获取第一个返回值
	val := out[0]

	// 检查是否有 error 返回
	if len(out) == 2 {
		if err := out[1].Interface(); err != nil {
			fmt.Printf("bean error: %s:%d\n", beanDefinition.file, beanDefinition.line)
			panic(err)
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

	bean.bean = bean.rValue.Interface()

	// 对返回值进行依赖注入
	if bean.Type().Kind() == reflect.Interface {
		ctx.WireBean(bean.Value().Elem().Interface())
	} else {
		ctx.WireBean(bean.Value().Interface())
	}
}

// wireStructField 对结构体的字段进行绑定
func (ctx *defaultSpringContext) wireStructField(parentValue reflect.Value,
	beanValue reflect.Value, field string, beanId string) {

	_, beanName, nullable := ParseBeanId(beanId)
	if beanName == "[]" { // 收集模式，autowire:"[]"

		// 收集模式的绑定对象必须是数组
		if beanValue.Type().Kind() != reflect.Slice {
			panic(errors.New(field + " must be slice when autowire []"))
		}

		ok := ctx.collectBeans(beanValue)
		if !ok && !nullable { // 没找到且不能为空则 panic
			panic(errors.New(field + " 没有找到符合条件的 Bean"))
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
