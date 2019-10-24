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
	"github.com/spf13/viper"
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

	// 收集模式得到的 Bean 列表，一个类型只需收集一次。
	Collect reflect.Value
}

//
// 工厂函数
//
func NewCachedBeanMapItem() *CachedBeanMapItem {
	return &CachedBeanMapItem{
		Named: make([]*BeanDefinition, 0),
	}
}

//
// 将一个 Bean 存储到 CachedBeanMapItem 里
//
func (item *CachedBeanMapItem) Store(d *BeanDefinition) {
	if d.Name == "" {
		panic("请给 Bean 指定一个名称")
	}
	item.Named = append(item.Named, d)
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
	Properties    map[string]interface{} // 所有属性值的集合
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
		Properties:    make(map[string]interface{}),
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

	// 生成默认名称
	if name == "" {
		name = t.String()
	}

	return &BeanDefinition{
		Init:  Uninitialized,
		Name:  name,
		Bean:  bean,
		Type:  t,
		Value: v,
	}
}

//
// 注册单例 Bean，不指定名称，重复注册会 panic。
//
func (ctx *DefaultSpringContext) RegisterBean(bean SpringBean) {
	ctx.RegisterNameBean("", bean)
}

//
// 注册单例 Bean，需指定名称，重复注册会 panic。
//
func (ctx *DefaultSpringContext) RegisterNameBean(name string, bean SpringBean) {
	beanDefinition := ToBeanDefinition(name, bean)
	ctx.RegisterBeanDefinition(beanDefinition)
}

//
// 注册单例 Bean，使用 BeanDefinition 对象，重复注册会 panic。
//
func (ctx *DefaultSpringContext) RegisterBeanDefinition(d *BeanDefinition) {
	fmt.Printf("register bean %s:%s\n", TypeName(d.Type), d.Name)
	item, _ := ctx.CachedBeanMap.Get(d.Type)
	ctx.BeanMap.Store(d)
	item.Store(d)
}

//
// 根据类型获取单例 Bean，多于 1 个会 panic，找不到也会 panic。
//
func (ctx *DefaultSpringContext) GetBean(i interface{}) {
	ctx.GetBeanByName("", i)
}

//
// 根据类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
//
func (ctx *DefaultSpringContext) FindBean(i interface{}) bool {
	return ctx.FindBeanByName("", i)
}

//
// 根据名称和类型获取单例 Bean，多于 1 个会 panic，找不到也会 panic。
//
func (ctx *DefaultSpringContext) GetBeanByName(beanId string, i interface{}) {
	if ok := ctx.FindBeanByName(beanId, i); !ok {
		panic("没有找到符合条件的 Bean")
	}
}

var EMPTY_VALUE = reflect.ValueOf(0)

//
// 根据名称和类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
//
func (ctx *DefaultSpringContext) FindBeanByName(beanId string, i interface{}) bool {

	it := reflect.TypeOf(i)

	// 使用指针才能够对外赋值
	if it.Kind() != reflect.Ptr {
		panic("i must be pointer")
	}

	iv := reflect.ValueOf(i)

	typeName, beanName, _ := ParseBeanId(beanId)
	return ctx.findBeanByName(typeName, beanName, EMPTY_VALUE, iv.Elem(), "")
}

//
// 解析 BeanId 的内容
//
func ParseBeanId(beanId string) (typeName string, beanName string, nullable bool) {

	if ss := strings.Split(beanId, ":"); len(ss) > 1 {
		typeName = ss[0]
		beanName = ss[1]
	} else {
		beanName = ss[0]
	}

	if strings.HasSuffix(beanName, "?") {
		beanName = beanName[:len(beanName)-1]
		nullable = true
	}

	return
}

func (ctx *DefaultSpringContext) findBeanByName(typeName string, beanName string, parentValue reflect.Value, fv reflect.Value, fName string) bool {

	t := fv.Type()

	// 检查接收者的类型，接收者必须是指针、数组、接口其中的一种，不能是原始类型。
	if t.Kind() != reflect.Ptr && t.Kind() != reflect.Slice && t.Kind() != reflect.Interface {
		panic("receiver \"" + fName + "\" must be pointer or slice or interface")
	}

	m, ok := ctx.CachedBeanMap.Get(t)

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
			return false
		}

		// 多于 1 个
		if count > 1 {
			panic("找到多个符合条件的值")
		}

		// 对结果进行自动注入
		ctx.WireBeanDefinition(result)

		// 恰好 1 个
		fv.Set(result.Value)
		return true
	}

	// 未命中缓存，则从注册列表里面查询，并更新缓存
	if !ok {

		// TODO 查询优化 MAP
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

//
// 收集数组或指针定义的所有符合条件的 Bean 对象，收集不到会 panic。
//
func (ctx *DefaultSpringContext) MustCollectBeans(i interface{}) {
	if ok := ctx.CollectBeans(i); !ok {
		panic("没有找到符合条件的 Bean")
	}
}

func (ctx *DefaultSpringContext) collectBeans(v reflect.Value) bool {

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

// 获取所有的 bean 对象
func (ctx *DefaultSpringContext) GetAllBeansDefinition() []*BeanDefinition {
	result := make([]*BeanDefinition, 0)
	for _, v := range ctx.BeanMap {
		result = append(result, v)
	}
	return result
}

//
// 加载属性配置文件
//
func (ctx *DefaultSpringContext) LoadProperties(filename string) {

	v := viper.New()
	v.SetConfigFile(filename)
	v.ReadInConfig()

	for _, key := range v.AllKeys() {
		val := v.Get(key)
		ctx.SetProperty(key, val)
	}
}

//
// 获取属性值，属性名称不支持大小写。
//
func (ctx *DefaultSpringContext) GetProperty(name string) interface{} {
	return ctx.Properties[name]
}

//
// 获取布尔型属性值，属性名称不支持大小写。
//
func (ctx *DefaultSpringContext) GetBoolProperty(name string) bool {
	return cast.ToBool(ctx.GetProperty(name))
}

//
// 获取有符号整型属性值，属性名称不支持大小写。
//
func (ctx *DefaultSpringContext) GetIntProperty(name string) int64 {
	return cast.ToInt64(ctx.GetProperty(name))
}

//
// 获取无符号整型属性值，属性名称不支持大小写。
//
func (ctx *DefaultSpringContext) GetUintProperty(name string) uint64 {
	return cast.ToUint64(ctx.GetProperty(name))
}

//
// 获取浮点型属性值，属性名称不支持大小写。
//
func (ctx *DefaultSpringContext) GetFloatProperty(name string) float64 {
	return cast.ToFloat64(ctx.GetProperty(name))
}

//
// 获取字符串型属性值，属性名称不支持大小写。
//
func (ctx *DefaultSpringContext) GetStringProperty(name string) string {
	return cast.ToString(ctx.GetProperty(name))
}

//
// 设置属性值，属性名称不支持大小写。
//
func (ctx *DefaultSpringContext) SetProperty(name string, value interface{}) {
	ctx.Properties[name] = value
}

//
// 获取指定前缀的属性值集合
//
func (ctx *DefaultSpringContext) GetPrefixProperties(prefix string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range ctx.Properties {
		if strings.HasPrefix(k, prefix) {
			result[k] = v
		}
	}
	return result
}

//
// 获取属性值，如果没有找到则使用指定的默认值
//
func (ctx *DefaultSpringContext) GetDefaultProperty(name string, defaultValue interface{}) (interface{}, bool) {
	if v, ok := ctx.Properties[name]; ok {
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

func (ctx *DefaultSpringContext) handleTagAutowire(parentValue reflect.Value, f reflect.StructField, fv reflect.Value, fName string) {
	beanId, ok := f.Tag.Lookup("autowire")
	if !ok { // 没有 autowire 标签
		return
	}

	typeName, beanName, nullable := ParseBeanId(beanId)

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

		ok := ctx.findBeanByName(typeName, beanName, parentValue, fv, fName)
		if !ok && !nullable { // 没找到且不能为空则 panic
			panic(fName + " 没有找到符合条件的 Bean")
		}
	}
}

func (ctx *DefaultSpringContext) subStructValue(prefix string, f reflect.StructField, fv reflect.Value) {
	ft := f.Type
	for i := 0; i < ft.NumField(); i++ {
		it := ft.Field(i)
		iv := fv.Field(i)
		ctx.handleTagValue(prefix, it, iv, "")
	}
}

func (ctx *DefaultSpringContext) handleTagValue(prefix string, f reflect.StructField, fv reflect.Value, fName string) {
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

	// 结构体不能指定默认值
	if fvk == reflect.Struct {

		if len(ss) > 1 {
			panic(fName + " 结构体属性不能指定默认值")
		}

		ctx.subStructValue(propName, f, fv)
		return
	}

	if prop, ok := ctx.GetDefaultProperty(propName, ""); ok {
		propValue = prop
	} else {
		if len(ss) < 2 {
			panic("property \"" + propName + "\" not config")
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
		panic(fName + " unsupported type " + fvk.String())
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

			fmt.Printf("wire bean %s:%s\n", TypeName(t), beanDefinition.Name)

			sv := beanDefinition.Value
			v := sv.Elem()

			for i := 0; i < t.NumField(); i++ {
				f := t.Field(i)
				fv := v.Field(i)

				fName := t.Name() + ".$" + f.Name

				// 处理 value 标签
				ctx.handleTagValue("", f, fv, fName)

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

	beanDefinition.Init = Initialized
	return nil
}
