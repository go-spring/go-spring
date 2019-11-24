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
	"strings"

	"github.com/spf13/cast"
)

var (
	EMPTY_VALUE = reflect.Value{}
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
		panic("请给 Bean 指定一个名称")
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

	BeanMap   map[BeanKey]*BeanDefinition     // 所有 Bean 的集合
	BeanCache map[reflect.Type]*BeanCacheItem // Bean 的分组缓存
	autoWired bool                            // 已经执行自动绑定
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

func (ctx *DefaultSpringContext) getBeanCacheItem(t reflect.Type) (*BeanCacheItem, bool) {
	c, ok := ctx.BeanCache[t]
	if !ok {
		c = NewBeanCacheItem()
		ctx.BeanCache[t] = c
	}
	return c, ok
}

//
// 注册单例 Bean，不指定名称，重复注册会 panic。
//
func (ctx *DefaultSpringContext) RegisterBean(bean SpringBean) *Conditional {
	return ctx.RegisterNameBean("", bean)
}

//
// 注册单例 Bean，需指定名称，重复注册会 panic。
//
func (ctx *DefaultSpringContext) RegisterNameBean(name string, bean SpringBean) *Conditional {
	beanDefinition := ToBeanDefinition(name, bean)
	return ctx.RegisterBeanDefinition(beanDefinition)
}

//
// 注册单例 Bean，使用 BeanDefinition 对象，重复注册会 panic。
//
func (ctx *DefaultSpringContext) RegisterBeanDefinition(d *BeanDefinition) *Conditional {

	if ctx.autoWired { // 注册已被冻结
		panic("bean registration frozen")
	}

	// Store the bean into BeanMap
	{
		k := BeanKey{
			Type: d.Type,
			Name: d.Name,
		}

		if _, ok := ctx.BeanMap[k]; ok {
			panic("Bean 重复注册")
		}

		ctx.BeanMap[k] = d
	}

	d.cond = NewConditional()
	return d.cond
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

	return ctx.findBeanByName(beanId, EMPTY_VALUE, iv.Elem(), "")
}

func (ctx *DefaultSpringContext) findBeanByName(beanId string, parentValue reflect.Value, fv reflect.Value, fName string) bool {
	typeName, beanName, nullable := ParseBeanId(beanId)

	t := fv.Type()

	// 检查接收者的类型，接收者必须是指针、数组、接口其中的一种，不能是原始类型。
	if t.Kind() != reflect.Ptr && t.Kind() != reflect.Slice && t.Kind() != reflect.Interface && t.Kind() != reflect.Map {
		panic("receiver \"" + fName + "\" must be pointer or slice or interface or map")
	}

	m, ok := ctx.getBeanCacheItem(t)

	found := func(bean *BeanDefinition) bool {

		// 不能将自身赋给自身的字段
		if bean.Value == parentValue {
			return false
		}

		// 类型不相容
		return bean.Type.AssignableTo(t)
	}

	var (
		count  int
		result *BeanDefinition
	)

	checkResult := func() bool {

		// 没有找到
		if count == 0 {
			if nullable {
				return false
			} else {
				panic(fName + " 没有找到符合条件的 Bean")
			}
		}

		// 多于 1 个
		if count > 1 {
			panic("找到多个符合条件的值")
		}

		// 对依赖项进行依赖注入
		ctx.WireBeanDefinition(result)

		// 恰好 1 个
		fv.Set(result.Value)
		return true
	}

	// 未命中缓存，则从注册列表里面查询，并更新缓存
	if !ok {

		for _, bean := range ctx.BeanMap {
			if found(bean) {
				m.Store(bean)
				if bean.Match(typeName, beanName) {
					result = bean
					count++
				}
			}
		}

		return checkResult()
	}

	// 命中缓存，则从缓存中查询

	for _, bean := range m.Named {
		if found(bean) && bean.Match(typeName, beanName) {
			result = bean
			count++
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
		panic("i must be pointer")
	}

	et := it.Elem()

	if et.Kind() != reflect.Slice {
		panic("i.Elem() must be slice")
	}

	ev := reflect.ValueOf(i).Elem()

	return ctx.collectBeans(ev)
}

func (ctx *DefaultSpringContext) collectBeans(v reflect.Value) bool {

	t := v.Type()
	et := t.Elem()

	m, ok := ctx.getBeanCacheItem(t)

	// 未命中缓存，或者还没有收集到数据，则从注册列表里面查询，并更新缓存
	if !ok || !m.Collect.IsValid() {

		// 创建一个空数组
		ev := reflect.New(t).Elem()

		for _, d := range ctx.BeanMap {
			dt := d.Type

			if dt.AssignableTo(et) { // Bean 自身符合条件
				ev = reflect.Append(ev, d.Value)

			} else if dt.Kind() == reflect.Slice { // 找到一个数组
				if dt.Elem().AssignableTo(et) {

					// 数组扩容
					size := ev.Len() + d.Value.Len()
					newSlice := reflect.MakeSlice(t, size, size)

					reflect.Copy(newSlice, ev)

					// 拷贝新元素
					for i := 0; i < d.Value.Len(); i++ {
						newSlice.Index(i + ev.Len()).Set(d.Value.Index(i))
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
//
func (ctx *DefaultSpringContext) FindBeanByName(beanId string) (interface{}, bool) {

	// 确保存在可空标记，抑制 panic 效果。
	if beanId == "" || beanId[len(beanId)-1] != '?' {
		beanId += "?"
	}

	typeName, beanName, nullable := ParseBeanId(beanId)

	var (
		count  int
		key    BeanKey
		result *BeanDefinition
	)

	checkResult := func() (interface{}, bool) {

		// 没有找到
		if count == 0 {
			if nullable {
				return nil, false
			} else {
				panic("没有找到符合条件的 Bean")
			}
		}

		// 多于 1 个
		if count > 1 {
			panic("找到多个符合条件的值")
		}

		if result.Status < BeanStatus_Resolved {
			if !result.cond.Matches(ctx) {
				delete(ctx.BeanMap, key)
				return nil, false
			}
		}

		// 恰好 1 个
		return result.Value.Interface(), true
	}

	var bean *BeanDefinition

	for key, bean = range ctx.BeanMap {
		if bean.Match(typeName, beanName) {
			result = bean
			count++
		}
	}

	return checkResult()
}

//
// 获取所有的 bean 对象
//
func (ctx *DefaultSpringContext) GetAllBeanDefinitions() []*BeanDefinition {
	result := make([]*BeanDefinition, 0)
	for _, v := range ctx.BeanMap {
		result = append(result, v)
	}
	return result
}

//
// 自动绑定所有的 SpringBean
//
func (ctx *DefaultSpringContext) AutoWireBeans() {

	// 不再接受 Bean 注册，因为性能的原因使用了缓存，并且在 AutoWireBeans 的过程中
	// 逐步建立起这个缓存，而随着缓存的建立，绑定的速度会越来越快，从而减少性能的损失。

	ctx.autoWired = true

	for key, beanDefinition := range ctx.BeanMap {

		// 检查是否符合注册条件，不符合的立即删除
		if !beanDefinition.cond.Matches(ctx) {
			delete(ctx.BeanMap, key)
			continue
		}

		beanDefinition.Status = BeanStatus_Resolved

		// 将符合注册条件的 Bean 放入到缓存里面
		fmt.Printf("register bean %s:%s\n", TypeName(beanDefinition.Type), beanDefinition.Name)
		item, _ := ctx.getBeanCacheItem(beanDefinition.Type)
		item.Store(beanDefinition)
	}

	for _, beanDefinition := range ctx.BeanMap {
		if err := ctx.WireBeanDefinition(beanDefinition); err != nil {
			panic(err)
		}
	}
}

//
// 绑定外部指定的 SpringBean
//
func (ctx *DefaultSpringContext) WireBean(bean SpringBean) error {
	beanDefinition := ToBeanDefinition("", bean)
	return ctx.WireBeanDefinition(beanDefinition)
}

func (ctx *DefaultSpringContext) handleTagAutowire(parentValue reflect.Value, f reflect.StructField, fv reflect.Value, fName string) {
	beanId, ok := f.Tag.Lookup("autowire")
	if !ok { // 没有 autowire 标签
		return
	}

	_, beanName, nullable := ParseBeanId(beanId)

	if beanName == "[]" { // 收集模式，autowire:"[]"
		fvk := fv.Type().Kind()

		// 收集模式的绑定对象必须是数组
		if fvk != reflect.Slice {
			panic(fName + " must be slice when autowire []")
		}

		ok := ctx.collectBeans(fv)
		if !ok && !nullable { // 没找到且不能为空则 panic
			panic(fName + " 没有找到符合条件的 Bean")
		}

	} else { // 匹配模式，autowire:"" or autowire:"name"
		ctx.findBeanByName(beanId, parentValue, fv, fName)
	}
}

type PropertyHolder interface {
	GetDefaultProperty(name string, defaultValue interface{}) (interface{}, bool)
}

type MapPropertyHolder struct {
	m map[interface{}]interface{}
}

func NewMapPropertyHolder(m map[interface{}]interface{}) *MapPropertyHolder {
	return &MapPropertyHolder{
		m: m,
	}
}

func (p *MapPropertyHolder) GetDefaultProperty(name string, defaultValue interface{}) (interface{}, bool) {
	v, ok := p.m[name]
	return v, ok
}

func subStructValue(prop PropertyHolder, prefix string, ft reflect.Type, fv reflect.Value) {
	for i := 0; i < ft.NumField(); i++ {
		it := ft.Field(i)
		iv := fv.Field(i)
		handleTagValue(prop, prefix, it, iv, "")
	}
}

func handleTagValue(prop PropertyHolder, prefix string, f reflect.StructField, fv reflect.Value, fName string) {
	tagValue, ok := f.Tag.Lookup("value")
	if !ok { // 没有 value 标签
		return
	}

	// 检查语法是否正确
	if !(strings.HasPrefix(tagValue, "${") && strings.HasSuffix(tagValue, "}")) {
		panic(fName + " 属性绑定的语法发生错误")
	}

	fvk := fv.Kind()

	// 指针不能作为属性绑定的目标
	if fvk == reflect.Ptr {
		panic(fName + " 属性绑定的目标不能是指针")
	}

	ss := strings.Split(tagValue[2:len(tagValue)-1], ":=")

	var (
		propName  string
		propValue interface{}
	)

	propName = ss[0]

	// 属性名如果有前缀要加上前缀
	if prefix != "" {
		propName = prefix + "." + propName
	}

	if len(ss) > 1 {
		propValue = ss[1]
	}

	// 检查是否有默认值
	checkDefaultProperty := func() {
		if prop, ok := prop.GetDefaultProperty(propName, ""); ok {
			propValue = prop
		} else {
			if len(ss) < 2 {
				panic("properties \"" + propName + "\" not config")
			}
		}
	}

	// 结构体不能指定默认值
	if fvk == reflect.Struct {

		// 存在类型转换器的情况下优先使用属性值绑定，否则才考虑属性嵌套
		if fn, ok := typeConverters[f.Type]; ok {

			checkDefaultProperty()

			v := reflect.ValueOf(fn)
			res := v.Call([]reflect.Value{reflect.ValueOf(propValue)})
			fv.Set(res[0])
			return
		}

		if len(ss) > 1 {
			panic(fName + " 结构体属性不能指定默认值")
		}

		subStructValue(prop, propName, f.Type, fv)
		return
	}

	checkDefaultProperty()

	switch fv.Kind() {
	case reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8, reflect.Uint:
		u := cast.ToUint64(propValue)
		fv.SetUint(u)
	case reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8, reflect.Int:
		i := cast.ToInt64(propValue)
		fv.SetInt(i)
	case reflect.Float64, reflect.Float32:
		fv.SetFloat(cast.ToFloat64(propValue))
	case reflect.String:
		s := cast.ToString(propValue)
		fv.SetString(s)
	case reflect.Bool:
		b := cast.ToBool(propValue)
		fv.SetBool(b)
	case reflect.Slice:
		{
			elemType := fv.Type().Elem()
			elemKind := elemType.Kind()

			switch elemKind {
			case reflect.Int:
				i := cast.ToIntSlice(propValue)
				fv.Set(reflect.ValueOf(i))
			case reflect.String:
				i := cast.ToStringSlice(propValue)
				fv.Set(reflect.ValueOf(i))
			default:
				if fn, ok := typeConverters[elemType]; ok {
					// 首先处理使用类型转换器的场景

					v := reflect.ValueOf(fn)
					s0 := cast.ToStringSlice(propValue)
					sv := reflect.MakeSlice(f.Type, len(s0), len(s0))

					for i, iv := range s0 {
						res := v.Call([]reflect.Value{reflect.ValueOf(iv)})
						sv.Index(i).Set(res[0])
					}

					fv.Set(sv)

				} else { // 然后处理结构体嵌套的场景

					if s, ok := propValue.([]interface{}); ok {
						result := reflect.MakeSlice(f.Type, len(s), len(s))

						for i, si := range s {
							if sv, ok := si.(map[interface{}]interface{}); ok {
								ev := reflect.New(elemType)
								subStructValue(NewMapPropertyHolder(sv), "", elemType, ev.Elem())
								result.Index(i).Set(ev.Elem())
							} else {
								panic(fmt.Sprintf("property %s isn't []map[string]interface{}", propName))
							}
						}

						fv.Set(result)

					} else {
						panic(fmt.Sprintf("property %s isn't []map[string]interface{}", propName))
					}
				}
			}
		}
	default:
		panic(fName + " unsupported type " + fvk.String())
	}
}

//
// 绑定 BeanDefinition 指定的 SpringBean
//
func (ctx *DefaultSpringContext) WireBeanDefinition(beanDefinition *BeanDefinition) error {

	// 解决循环依赖问题
	if beanDefinition.Status >= BeanStatus_Wiring {
		return nil
	}

	beanDefinition.Status = BeanStatus_Wiring

	st := beanDefinition.Type

	// 目标对象必须是结构体指针才能绑定
	if st.Kind() == reflect.Ptr {
		t := st.Elem()
		if t.Kind() == reflect.Struct {

			fmt.Printf("wire bean %s:%s\n", TypeName(t), beanDefinition.Name)

			sv := beanDefinition.Value
			v := sv.Elem()

			for i := 0; i < t.NumField(); i++ {
				f := t.Field(i)
				fv := v.Field(i)

				fName := t.Name() + ".$" + f.Name

				// 处理 value 标签
				handleTagValue(ctx, "", f, fv, fName)

				// 处理 autowire 标签
				ctx.handleTagAutowire(sv, f, fv, fName)
			}

			// 初始化当前的 SpringBean
			if c, ok := beanDefinition.Bean.(BeanInitialization); ok {
				c.InitBean(ctx)
			}

			fmt.Printf("success wire bean %s:%s\n", TypeName(t), beanDefinition.Name)
		}
	}

	beanDefinition.Status = BeanStatus_Wired
	return nil
}
