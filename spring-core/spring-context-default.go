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

//
// 定义 Bean 的唯一标识符，类型+名称。
//
type BeanKey struct {
	Type reflect.Type
	Name string
}

//
// 定义 BeanMap 类型
//
type BeanMap map[BeanKey]*BeanDefinition

//
// 工厂函数
//
func NewBeanMap() BeanMap {
	return make(map[BeanKey]*BeanDefinition)
}

//
// 把一个 Bean 存储到 BeanMap 里
//
func (m BeanMap) Store(d *BeanDefinition) {

	k := BeanKey{
		Type: d.Type,
		Name: d.Name,
	}

	if _, ok := m[k]; ok {
		panic("Bean 重复注册")
	}

	m[k] = d
}

//
// 定义 CachedBeanMap 元素的类型
//
type CachedBeanMapItem struct {
	// 已命名 Bean 的列表，实现了相同接口的不同类型有可能具有相同
	// 的名称，因此名称不能做 Map 的 Key，兼顾性能用 Array 存储.
	Named []*BeanDefinition

	// 未命名 Bean 的列表
	Unnamed []*BeanDefinition

	// 收集模式得到的 Bean 列表，一个类型只需收集一次。
	Collect reflect.Value
}

//
// 工厂函数
//
func NewCachedBeanMapItem() *CachedBeanMapItem {
	return &CachedBeanMapItem{
		Named:   make([]*BeanDefinition, 0),
		Unnamed: make([]*BeanDefinition, 0),
	}
}

//
// 将一个 Bean 存储到 CachedBeanMapItem 里
//
func (item *CachedBeanMapItem) Store(d *BeanDefinition) {
	if d.Name != "" {
		item.Named = append(item.Named, d)
	} else {
		item.Unnamed = append(item.Unnamed, d)
	}
}

//
// 将收集到的 Bean 列表的值存储到 CachedBeanMapItem 里
//
func (item *CachedBeanMapItem) StoreCollect(v reflect.Value) {
	item.Collect = v
}

//
// 定义 CachedBeanMap 类型
//
type CachedBeanMap map[reflect.Type]*CachedBeanMapItem

//
// 工厂函数
//
func NewCachedBeanMap() CachedBeanMap {
	return make(map[reflect.Type]*CachedBeanMapItem)
}

//
// 获取缓存的 CachedBeanMapItem 对象，如果已经存在则返回 true，
// 否则创建并缓存一个新的 CachedBeanMapItem 对象同时返回 false。
//
func (m CachedBeanMap) Get(t reflect.Type) (*CachedBeanMapItem, bool) {
	c, ok := m[t]
	if !ok {
		c = NewCachedBeanMapItem()
		m[t] = c
	}
	return c, ok
}

//
// SpringContext 的默认版本
//
type DefaultSpringContext struct {
	Wired         bool                   // 绑定过程已经完成
	propertiesMap map[string]interface{} // 所有属性值的集合
	BeanMap       BeanMap                // 所有 Bean 的集合
	CachedBeanMap CachedBeanMap          // 根据类型分组 Bean
}

//
// 工厂函数
//
func NewDefaultSpringContext() *DefaultSpringContext {
	return &DefaultSpringContext{
		Wired:         false,
		BeanMap:       NewBeanMap(),
		CachedBeanMap: NewCachedBeanMap(),
		propertiesMap: make(map[string]interface{}),
	}
}

//
// 将 SpringBean 转换为 BeanDefinition 对象
//
func ToBeanDefinition(name string, bean SpringBean) *BeanDefinition {

	t := reflect.TypeOf(bean)

	// 检查 Bean 的类型，只能注册指针或者数组类型的 Bean
	if t.Kind() != reflect.Ptr && t.Kind() != reflect.Slice {
		panic("bean must be pointer or slice")
	}

	v := reflect.ValueOf(bean)

	return &BeanDefinition{
		Init:  Uninitialized,
		Name:  name,
		Bean:  bean,
		Type:  t,
		Value: v,
	}
}

//
// 注册单例 Bean，不指定名称
//
func (ctx *DefaultSpringContext) RegisterBean(bean SpringBean) {
	ctx.RegisterNameBean("", bean)
}

//
// 注册单例 Bean，需指定名称
//
func (ctx *DefaultSpringContext) RegisterNameBean(name string, bean SpringBean) {
	beanDefinition := ToBeanDefinition(name, bean)
	ctx.RegisterBeanDefinition(beanDefinition)
}

//
// 注册单例 Bean，使用 BeanDefinition 对象
//
func (ctx *DefaultSpringContext) RegisterBeanDefinition(d *BeanDefinition) {
	item, _ := ctx.CachedBeanMap.Get(d.Type)
	ctx.BeanMap.Store(d)
	item.Store(d)
}

//
// 根据类型获取单例 Bean，若多于 1 个则 panic
//
func (ctx *DefaultSpringContext) GetBean(i interface{}) {
	ctx.GetBeanByName("", i)
}

func (ctx *DefaultSpringContext) getBeanByName(name string, v reflect.Value) {

	t := v.Type()

	// 检查接收者的类型，接收者必须是指针、数组、接口其中的一种，不能是原始类型。
	if t.Kind() != reflect.Ptr && t.Kind() != reflect.Slice && t.Kind() != reflect.Interface {
		panic("receiver must be pointer or slice or interface")
	}

	m, ok := ctx.CachedBeanMap.Get(t)

	// 未命中缓存，则从注册列表里面查询，并更新缓存
	if !ok {

		var (
			count  int
			result *BeanDefinition
		)

		for _, v := range ctx.BeanMap {
			if v.Type.AssignableTo(t) {
				m.Store(v)

				// 如果不指定名称，则所有结果都匹配；
				// 如果指定了名称，则名称相同才匹配。
				if name == "" || v.Name == name {
					result = v
					count++
				}
			}
		}

		// 没有找到
		if count == 0 {
			return
		}

		// 多于 1 个
		if count > 1 {
			panic("找到多个符合条件的值")
		}

		// 恰好 1 个
		v.Set(result.Value)
		return
	}

	// 命中缓存，则从缓存中查询

	if name == "" {

		ln := len(m.Named)
		lu := len(m.Unnamed)

		if ln+lu > 1 {
			panic("找到多个符合条件的值")
		} else if ln == 1 {
			v.Set(m.Named[0].Value)
		} else if lu == 1 {
			v.Set(m.Unnamed[0].Value)
		}

		return // 名称为空使用快速方法查询
	}

	var (
		count  int
		result *BeanDefinition
	)

	for _, v := range m.Named {
		if v.Name == name {
			result = v
			count++
		}
	}

	// 没有找到
	if count == 0 {
		return
	}

	// 多于 1 个
	if count > 1 {
		panic("找到多个符合条件的值")
	}

	// 恰好 1 个
	v.Set(result.Value)
}

//
// 根据名称和类型获取单例 Bean，若多于 1 个则 panic
//
func (ctx *DefaultSpringContext) GetBeanByName(name string, i interface{}) {

	it := reflect.TypeOf(i)

	// 使用指针才能够对外赋值 TODO
	if it.Kind() != reflect.Ptr {
		panic("i must be pointer")
	}

	iv := reflect.ValueOf(i)

	ctx.getBeanByName(name, iv.Elem())
}

func (ctx *DefaultSpringContext) collectBeans(v reflect.Value) {

	t := v.Type()
	et := t.Elem()

	m, ok := ctx.CachedBeanMap.Get(t)

	// 未命中缓存，或者还没有收集到数据，
	// 则从注册列表里面查询，并更新缓存
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
		v.Set(ev)
		return
	}

	// 命中缓存，则从缓存中查询

	v.Set(m.Collect)
}

//
// 收集数组或指针定义的所有符合条件的 Bean 对象
//
func (ctx *DefaultSpringContext) CollectBeans(i interface{}) {

	it := reflect.TypeOf(i)

	if it.Kind() != reflect.Ptr {
		panic("i must be pointer")
	}

	et := it.Elem()

	if et.Kind() != reflect.Slice {
		panic("i.Elem() must be slice")
	}

	ev := reflect.ValueOf(i).Elem()

	ctx.collectBeans(ev)
}

// 获取所有的 bean 对象
func (ctx *DefaultSpringContext) GetAllBeansDefinition() []*BeanDefinition {
	result := make([]*BeanDefinition, 0)
	for _, v := range ctx.BeanMap {
		result = append(result, v)
	}
	return result
}

//
// 获取属性值
//
func (ctx *DefaultSpringContext) GetProperties(name string) interface{} {
	return ctx.propertiesMap[name]
}

//
// 设置属性值
//
func (ctx *DefaultSpringContext) SetProperties(name string, value interface{}) {
	ctx.propertiesMap[name] = value
}

//
// 获取指定前缀的属性值集合
//
func (ctx *DefaultSpringContext) GetPrefixProperties(prefix string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range ctx.propertiesMap {
		if strings.HasPrefix(k, prefix) {
			result[k] = v
		}
	}
	return result
}

//
// 获取属性值，如果没有找到则使用指定的默认值
//
func (ctx *DefaultSpringContext) GetDefaultProperties(name string, defaultValue interface{}) (interface{}, bool) {
	if v, ok := ctx.propertiesMap[name]; ok {
		return v, true
	}
	return defaultValue, false
}

//
// 自动绑定所有的 SpringBean
//
func (ctx *DefaultSpringContext) AutoWireBeans() {
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

func (ctx *DefaultSpringContext) handleTagAutowire(f reflect.StructField, fv reflect.Value) {
	if beanName, ok := f.Tag.Lookup("autowire"); ok {
		if len(beanName) > 0 { // autowire:"NAME" or autowire:"[]"

			if beanName == "[]" { // 收集模式
				fvk := fv.Type().Kind()

				// 收集模式的绑定对象必须是数组
				if fvk == reflect.Slice {
					ctx.collectBeans(fv)
				} else {
					panic("must be slice when autowire []")
				}

			} else {
				ctx.getBeanByName(beanName, fv)
			}

		} else { // autowire:""
			ctx.getBeanByName("", fv)
		}
	}
}

func (ctx *DefaultSpringContext) handleTagValue(f reflect.StructField, fv reflect.Value) {
	if value, ok := f.Tag.Lookup("value"); ok && len(value) > 0 {
		// TODO 数组绑定

		if strings.HasPrefix(value, "${") {
			str := value[2 : len(value)-1]
			ss := strings.Split(str, ":=")

			var (
				propName  string
				propValue interface{}
			)

			propName = ss[0]
			if len(ss) > 1 {
				propValue = ss[1]
			}

			if prop, ok := ctx.GetDefaultProperties(propName, ""); ok {
				propValue = prop
			} else {
				if len(ss) < 2 {
					panic("properties " + propName + " not config!")
				}
			}

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
			default:
				panic("unsupported type " + fv.Type().String())
			}
		}
	}
}

//
// 绑定 BeanDefinition 指定的 SpringBean
//
func (ctx *DefaultSpringContext) WireBeanDefinition(beanDefinition *BeanDefinition) error {

	// 解决循环依赖问题
	if beanDefinition.Init != Uninitialized {
		return nil
	}

	beanDefinition.Init = Initializing

	st := beanDefinition.Type

	// 目标对象必须是结构体指针才能绑定
	if st.Kind() == reflect.Ptr {
		t := st.Elem()
		if t.Kind() == reflect.Struct {

			fmt.Printf("wire bean %v:\"%s\"\n", beanDefinition.Type, beanDefinition.Name)

			sv := beanDefinition.Value
			v := sv.Elem()

			for i := 0; i < t.NumField(); i++ {
				f := t.Field(i)
				fv := v.Field(i)

				// 处理 value 标签
				ctx.handleTagValue(f, fv)

				// 处理 autowire 标签
				ctx.handleTagAutowire(f, fv)
			}

			// 初始化当前的 SpringBean
			if c, ok := beanDefinition.Bean.(BeanInitialization); ok {
				c.InitBean(ctx)
			}

			fmt.Printf("success wire bean %v:\"%s\"\n", beanDefinition.Type, beanDefinition.Name)
		}
	}

	beanDefinition.Init = Initialized
	return nil
}
