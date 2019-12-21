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
	"fmt"
	"reflect"
	"runtime"
	"strings"
)

var (
	_EMPTY_VALUE = reflect.Value{}
)

//
// BeanKey defines a bean's unique key, type+name.
//
type BeanKey struct {
	Type reflect.Type
	Name string
}

//
// BeanCacheItem defines a BeanCache's item.
//
type BeanCacheItem struct {
	// Different typed beans implemented same interface maybe
	// have same name, so name can't be a map's key. therefor
	// we use a list to store the cached beans.
	Named []*BeanDefinition

	// 收集模式得到的 Bean 列表，一个类型只需收集一次。
	Collect reflect.Value
}

//
// BeanCacheItem's factory method.
//
func NewBeanCacheItem() *BeanCacheItem {
	return &BeanCacheItem{
		Named: make([]*BeanDefinition, 0),
	}
}

//
// 将一个 Bean 存储到 CachedBeanMapItem 里
//
func (item *BeanCacheItem) Store(d *BeanDefinition) {
	if d.Name == "" {
		panic("bean must have name")
	}
	item.Named = append(item.Named, d)
}

//
// 将收集到的 Bean 列表的值存储到 CachedBeanMapItem 里
//
func (item *BeanCacheItem) StoreCollect(v reflect.Value) {
	item.Collect = v
}

//
// SpringContext 的默认版本
//
type DefaultSpringContext struct {
	// 属性值列表接口
	*DefaultProperties

	profile   string // 运行环境
	autoWired bool   // 已经执行自动绑定

	BeanMap   map[BeanKey]*BeanDefinition     // Bean 的集合
	BeanCache map[reflect.Type]*BeanCacheItem // Bean 的缓存
}

//
// 工厂函数
//
func NewDefaultSpringContext() *DefaultSpringContext {
	return &DefaultSpringContext{
		DefaultProperties: NewDefaultProperties(),
		BeanMap:           make(map[BeanKey]*BeanDefinition),
		BeanCache:         make(map[reflect.Type]*BeanCacheItem),
	}
}

//
// 获取运行环境
//
func (ctx *DefaultSpringContext) GetProfile() string {
	return ctx.profile
}

//
// 设置运行环境
//
func (ctx *DefaultSpringContext) SetProfile(profile string) {
	ctx.profile = profile
}

//
// 注册单例 Bean，无需指定名称，重复注册会 panic。
//
func (ctx *DefaultSpringContext) RegisterBean(bean interface{}) *BeanDefinition {
	return ctx.RegisterNameBean("", bean)
}

//
// 注册单例 Bean，需要指定名称，重复注册会 panic。
//
func (ctx *DefaultSpringContext) RegisterNameBean(name string, bean interface{}) *BeanDefinition {
	beanDefinition := ToBeanDefinition(name, bean)
	ctx.registerBeanDefinition(beanDefinition)
	return beanDefinition
}

//
// 通过构造函数注册单例 Bean，无需指定名称，重复注册会 panic。
//
func (ctx *DefaultSpringContext) RegisterBeanFn(fn interface{}, tags ...string) *BeanDefinition {
	return ctx.RegisterNameBeanFn("", fn, tags...)
}

//
// 通过构造函数注册单例 Bean，需要指定名称，重复注册会 panic。
//
func (ctx *DefaultSpringContext) RegisterNameBeanFn(name string, fn interface{}, tags ...string) *BeanDefinition {
	beanDefinition := FnToBeanDefinition(name, fn, tags...)
	ctx.registerBeanDefinition(beanDefinition)
	return beanDefinition
}

//
// 通过成员方法注册单例 Bean，不指定名称，重复注册会 panic。
//
func (ctx *DefaultSpringContext) RegisterMethodBean(parent *BeanDefinition, method string, tags ...string) *BeanDefinition {
	return ctx.RegisterNameMethodBean("", parent, method, tags...)
}

//
// 通过成员方法注册单例 Bean，需指定名称，重复注册会 panic。
//
func (ctx *DefaultSpringContext) RegisterNameMethodBean(name string, parent *BeanDefinition, method string, tags ...string) *BeanDefinition {
	beanDefinition := MethodToBeanDefinition(name, parent, method, tags...)
	ctx.registerBeanDefinition(beanDefinition)
	return beanDefinition
}

//
// 注册单例 Bean，使用 BeanDefinition 对象，重复注册会 panic。
//
func (ctx *DefaultSpringContext) registerBeanDefinition(d *BeanDefinition) {

	// 获取注册点信息
	for i := 3; i < 10; i++ {
		_, file, line, _ := runtime.Caller(i)
		if !strings.Contains(file, "/go-spring/go-spring/spring-") || strings.HasSuffix(file, "_test.go") {
			d.file = file
			d.line = line
			break
		}
	}

	if ctx.autoWired { // 注册已被冻结
		panic("bean registration frozen")
	}

	// store the bean into BeanMap
	{
		k := BeanKey{
			Type: d.Type(),
			Name: d.Name,
		}

		if _, ok := ctx.BeanMap[k]; ok {
			panic("Bean 重复注册")
		}

		ctx.BeanMap[k] = d
	}
}

func (ctx *DefaultSpringContext) findCache(t reflect.Type) (*BeanCacheItem, bool) {
	c, ok := ctx.BeanCache[t]
	if !ok {
		c = NewBeanCacheItem()
		ctx.BeanCache[t] = c
	}
	return c, ok
}

//
// 根据类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
//
func (ctx *DefaultSpringContext) GetBean(i interface{}) bool {
	return ctx.GetBeanByName("?", i)
}

//
// 根据名称和类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
//
func (ctx *DefaultSpringContext) GetBeanByName(beanId string, i interface{}) bool {

	if !ctx.autoWired {
		panic("should call after ctx.AutoWireBeans()")
	}

	// 确保存在可空标记，抑制 panic 效果。
	if beanId == "" || beanId[len(beanId)-1] != '?' {
		beanId += "?"
	}

	it := reflect.TypeOf(i)

	// 使用指针才能够对外赋值
	if it.Kind() != reflect.Ptr {
		panic("i must be pointer")
	}

	iv := reflect.ValueOf(i)

	return ctx.getBeanByName(beanId, _EMPTY_VALUE, iv.Elem(), "")
}

//
// 查找 bean
//
func (ctx *DefaultSpringContext) getBeanByName(beanId string, parentValue reflect.Value, fv reflect.Value, field string) bool {
	typeName, beanName, nullable := ParseBeanId(beanId)

	t := fv.Type()

	if !IsRefType(t.Kind()) {
		panic("receiver \"" + field + "\" must be ref type")
	}

	m, ok := ctx.findCache(t)

	found := func(bean *BeanDefinition) bool {

		// 不能将自身赋给自身的字段
		if bean.Value() == parentValue {
			return false
		}

		// 类型必须相容
		return bean.Type().AssignableTo(t)
	}

	result := make([]*BeanDefinition, 0)

	checkResult := func() bool {
		count := len(result)

		// 没有找到
		if count == 0 {
			if nullable {
				return false
			} else {
				panic(field + " 没有找到符合条件的 Bean")
			}
		}

		var primaryBean *BeanDefinition

		for _, bean := range result {
			if bean.primary {
				if primaryBean != nil {
					panic(field + " 找到多个 primary bean")
				}
				primaryBean = bean
			}
		}

		if primaryBean == nil {
			if count > 1 {
				panic(field + " 找到多个符合条件的值")
			}
			primaryBean = result[0]
		}

		// 依赖注入
		ctx.wireBeanDefinition(primaryBean)

		// 恰好 1 个
		fv.Set(primaryBean.Value())
		return true
	}

	// 未命中缓存，则从注册列表里面查询，并更新缓存
	if !ok {

		for _, bean := range ctx.BeanMap {
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

//
// 收集数组或指针定义的所有符合条件的 Bean 对象，收集到返回 true，否则返回 false。
//
func (ctx *DefaultSpringContext) CollectBeans(i interface{}) bool {

	if !ctx.autoWired {
		panic("should call after ctx.AutoWireBeans()")
	}

	it := reflect.TypeOf(i)

	if it.Kind() != reflect.Ptr {
		panic("i must be slice ptr")
	}

	et := it.Elem()

	if et.Kind() != reflect.Slice {
		panic("i must be slice ptr")
	}

	ev := reflect.ValueOf(i).Elem()
	return ctx.collectBeans(ev)
}

//
// 收集 Bean
//
func (ctx *DefaultSpringContext) collectBeans(v reflect.Value) bool {

	t := v.Type()
	et := t.Elem()

	m, ok := ctx.findCache(t)

	// 未命中缓存，或者还没有收集到数据，则从注册列表里面查询，并更新缓存
	if !ok || !m.Collect.IsValid() {

		// 创建一个空数组
		ev := reflect.New(t).Elem()

		for _, d := range ctx.BeanMap {
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
							ctx.wireValue(di.Addr())
						} else if di.Kind() == reflect.Ptr {
							if de := di.Elem(); de.Kind() == reflect.Struct {
								ctx.wireValue(di)
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

//
// 根据名称和类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
// 该方法不能保证 Bean 已经执行依赖注入和属性绑定，仅供查询 Bean 是否存在。
//
func (ctx *DefaultSpringContext) FindBeanByName(beanId string) (interface{}, bool) {

	if !ctx.autoWired {
		panic("should call after ctx.AutoWireBeans()")
	}

	typeName, beanName, _ := ParseBeanId(beanId)

	var (
		count  int
		result *BeanDefinition
	)

	for _, bean := range ctx.BeanMap {
		if bean.Match(typeName, beanName) {
			result = bean
			count++
		}
	}

	// 没有找到
	if count == 0 {
		return nil, false
	}

	// 多于 1 个
	if count > 1 {
		panic(beanId + " 找到多个符合条件的值")
	}

	// 恰好 1 个 & 仅供查询无需绑定
	// ctx.wireBeanDefinition(result)
	return result.Value().Interface(), true
}

//
// 对 Bean 进行决议是否能够创建 Bean 的实例
//
func (ctx *DefaultSpringContext) resolveBean(beanDefinition *BeanDefinition) {

	if beanDefinition.status != BeanStatus_Default {
		return
	}

	if ok := beanDefinition.GetResult(ctx); !ok { // 不满足则删除注册
		key := BeanKey{beanDefinition.Type(), beanDefinition.Name}
		beanDefinition.status = BeanStatus_Deleted
		delete(ctx.BeanMap, key)
		return
	}

	beanDefinition.status = BeanStatus_Resolved

	// 将符合注册条件的 Bean 放入到缓存里面
	fmt.Printf("register bean \"%s\" %s:%d\n", beanDefinition.BeanId(), beanDefinition.file, beanDefinition.line)
	item, _ := ctx.findCache(beanDefinition.Type())
	item.Store(beanDefinition)
}

//
// 自动绑定所有的 Bean
//
func (ctx *DefaultSpringContext) AutoWireBeans() {

	// 不再接受 Bean 注册，因为性能的原因使用了缓存，并且在 AutoWireBeans 的过程中
	// 逐步建立起这个缓存，而随着缓存的建立，绑定的速度会越来越快，从而减少性能的损失。

	ctx.autoWired = true

	// 首先决议当前 Bean 是否能够注册，否则会删除其注册信息
	for key, beanDefinition := range ctx.BeanMap {

		// 如果是成员方法 Bean，需要首先决议它的父 Bean 是否能实例化
		if mBean, ok := beanDefinition.SpringBean.(*MethodBean); ok {
			ctx.resolveBean(mBean.parent)

			// 父 Bean 已经被删除了，子 Bean 也不应该存在
			if mBean.parent.status == BeanStatus_Deleted {
				beanDefinition.status = BeanStatus_Deleted
				delete(ctx.BeanMap, key)
				continue
			}
		}

		ctx.resolveBean(beanDefinition)
	}

	// 然后执行 Bean 绑定
	for _, beanDefinition := range ctx.BeanMap {

		// 并且首先初始化当前 bean 不直接依赖的那些 bean
		for _, beanId := range beanDefinition.dependsOn {
			if _, ok := ctx.FindBeanByName(beanId); !ok {
				panic(beanId + " 没有找到符合条件的 Bean")
			}
		}

		ctx.wireBeanDefinition(beanDefinition)
	}
}

func (ctx *DefaultSpringContext) wireValue(v reflect.Value) {
	t := v.Type()
	bean := &OriginalBean{
		bean:     v.Interface(),
		rType:    t,
		typeName: TypeName(t),
		rValue:   v,
	}
	d := NewBeanDefinition(bean, "")
	ctx.wireBeanDefinition(d)
}

//
// 绑定外部指定的 Bean
//
func (ctx *DefaultSpringContext) WireBean(bean interface{}) {

	if !ctx.autoWired {
		panic("should call after ctx.AutoWireBeans()")
	}

	beanDefinition := ToBeanDefinition("", bean)
	ctx.wireBeanDefinition(beanDefinition)
}

//
// 绑定 BeanDefinition 指定的 Bean
//
func (ctx *DefaultSpringContext) wireBeanDefinition(beanDefinition *BeanDefinition) {

	// 如果是成员方法 Bean，需要首先初始化它的父 Bean
	if mBean, ok := beanDefinition.SpringBean.(*MethodBean); ok {
		ctx.wireBeanDefinition(mBean.parent)
	}

	// 解决循环依赖问题
	if beanDefinition.status >= BeanStatus_Wiring {
		return
	}

	beanDefinition.status = BeanStatus_Wiring

	switch beanDefinition.SpringBean.(type) {
	case *OriginalBean: // 原始对象
		ctx.wireOriginalBean(beanDefinition)
	case *ConstructorBean: // 构造函数
		ctx.wireConstructorBean(beanDefinition)
	case *MethodBean: // 成员方法
		ctx.wireMethodBean(beanDefinition)
	default:
		panic("unknown spring bean type")
	}

	// 如果有则执行用户设置的初始化函数
	if beanDefinition.initFunc != nil {
		fnValue := reflect.ValueOf(beanDefinition.initFunc)
		fnValue.Call([]reflect.Value{beanDefinition.Value()})
	}

	beanDefinition.status = BeanStatus_Wired
}

func (ctx *DefaultSpringContext) wireStructField(parentValue reflect.Value,
	fv reflect.Value, field string, beanId string) {

	_, beanName, nullable := ParseBeanId(beanId)

	if beanName == "[]" { // 收集模式，autowire:"[]"
		fvk := fv.Type().Kind()

		// 收集模式的绑定对象必须是数组
		if fvk != reflect.Slice {
			panic(field + " must be slice when autowire []")
		}

		ok := ctx.collectBeans(fv)
		if !ok && !nullable { // 没找到且不能为空则 panic
			panic(field + " 没有找到符合条件的 Bean")
		}

	} else { // 匹配模式，autowire:"" or autowire:"name"
		ctx.getBeanByName(beanId, parentValue, fv, field)
	}
}

//
// 对原始对象进行注入
//
func (ctx *DefaultSpringContext) wireOriginalBean(beanDefinition *BeanDefinition) {
	st := beanDefinition.Type()
	sk := st.Kind()

	if sk == reflect.Slice {
		et := st.Elem()
		ek := et.Kind()

		if ek == reflect.Struct {

			// 绑定结构体数组
			v := beanDefinition.Value()
			for i := 0; i < v.Len(); i++ {
				iv := v.Index(i).Addr()
				ctx.wireValue(iv)
			}

		} else if ek == reflect.Ptr {

			it := et.Elem()
			ik := it.Kind()

			// 结构体指针数组
			if ik == reflect.Struct {
				v := beanDefinition.Value()
				for p := 0; p < v.Len(); p++ {
					pv := v.Index(p)
					ctx.wireValue(pv)
				}
			}
		}
	}

	if sk == reflect.Ptr {
		t := st.Elem()
		if t.Kind() == reflect.Struct {

			fmt.Printf("wire bean %s:%s\n", TypeName(t), beanDefinition.Name)

			sv := beanDefinition.Value()
			v := sv.Elem()

			for i := 0; i < t.NumField(); i++ {
				f := t.Field(i)
				fv := v.Field(i)

				fieldName := t.Name() + ".$" + f.Name

				// 处理 value 标签
				if tag, ok := f.Tag.Lookup("value"); ok {
					bindStructField(ctx, f.Type, fv, fieldName, "", tag)
				} else {
					if f.Type.Kind() == reflect.Struct {
						bindStruct(ctx, f.Type, fv, fieldName, "")
					}
				}

				// 处理 autowire 标签
				if beanId, ok := f.Tag.Lookup("autowire"); ok {
					ctx.wireStructField(sv, fv, fieldName, beanId)
				}
			}

			fmt.Printf("success wire bean \"%s\"\n", beanDefinition.BeanId())
		}
	}
}

//
// 对构造函数进行注入
//
func (ctx *DefaultSpringContext) wireConstructorBean(beanDefinition *BeanDefinition) {
	cBean := beanDefinition.SpringBean.(*ConstructorBean)

	// 获取输入参数
	fnType := reflect.TypeOf(cBean.fn)
	in := cBean.arg.Get(ctx, fnType)

	// 运行构造函数
	fnValue := reflect.ValueOf(cBean.fn)
	out := fnValue.Call(in)

	// 获取第一个返回值
	val := out[0]

	// 检查是否有 error 返回
	if len(out) == 2 {
		if err := out[1].Interface(); err != nil {
			fmt.Printf("error: %s:%d\n", beanDefinition.file, beanDefinition.line)
			panic(err)
		}
	}

	if IsRefType(val.Kind()) {
		// 如果实现接口的值是个结构体，那么需要转换成指针类型然后赋给接口
		if val.Kind() == reflect.Interface && val.Elem().Kind() == reflect.Struct {
			ptrVal := reflect.New(val.Elem().Type())
			ptrVal.Elem().Set(val.Elem())
			cBean.rValue.Set(ptrVal)
		} else {
			cBean.rValue.Set(val)
		}
	} else {
		cBean.rValue.Elem().Set(val)
	}

	cBean.bean = cBean.rValue.Interface()

	// 对返回值进行依赖注入
	if cBean.Type().Kind() == reflect.Interface {
		ctx.wireValue(cBean.Value().Elem())
	} else {
		ctx.wireValue(cBean.Value())
	}

	fmt.Printf("success wire constructor bean \"%s\"\n", beanDefinition.BeanId())
}

//
// 对成员方法进行注入
//
func (ctx *DefaultSpringContext) wireMethodBean(beanDefinition *BeanDefinition) {
	mBean := beanDefinition.SpringBean.(*MethodBean)

	fnValue := mBean.parent.Value().MethodByName(mBean.method)
	fnType := fnValue.Type()

	in := mBean.arg.Get(ctx, fnType)
	out := fnValue.Call(in)

	// 获取第一个返回值
	val := out[0]

	// 检查是否有 error 返回
	if len(out) == 2 {
		if err := out[1].Interface(); err != nil {
			fmt.Printf("error: %s:%d\n", beanDefinition.file, beanDefinition.line)
			panic(err)
		}
	}

	if IsRefType(val.Kind()) {
		// 如果实现接口的值是个结构体，那么需要转换成指针类型然后赋给接口
		if val.Kind() == reflect.Interface && val.Elem().Kind() == reflect.Struct {
			ptrVal := reflect.New(val.Elem().Type())
			ptrVal.Elem().Set(val.Elem())
			mBean.rValue.Set(ptrVal)
		} else {
			mBean.rValue.Set(val)
		}
	} else {
		mBean.rValue.Elem().Set(val)
	}

	mBean.bean = mBean.rValue.Interface()

	// 对返回值进行依赖注入
	if mBean.Type().Kind() == reflect.Interface {
		ctx.wireValue(mBean.Value().Elem())
	} else {
		ctx.wireValue(mBean.Value())
	}

	fmt.Printf("success wire method bean \"%s\"\n", beanDefinition.BeanId())
}

//
// 获取所有 Bean 的定义，一般仅供调试使用。
//
func (ctx *DefaultSpringContext) GetAllBeanDefinitions() []*BeanDefinition {
	result := make([]*BeanDefinition, 0)
	for _, v := range ctx.BeanMap {
		result = append(result, v)
	}
	return result
}
