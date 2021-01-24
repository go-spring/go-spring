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
	"strings"

	"github.com/go-spring/spring-logger"
	"github.com/go-spring/spring-utils"
)

// beanAssembly Bean 组装车间
type beanAssembly interface {
	applicationContext() ApplicationContext

	// wireStructField 对结构体的字段进行绑定
	wireStructField(v reflect.Value, tag string, parent reflect.Value, field string)

	// collectBeans 收集符合要求的 Bean，结果可以是多个。自动模式下不对结
	// 果排序，指定模式会对结果排序。当允许结果为空时返回 false，否则 panic
	collectBeans(v reflect.Value, tag CollectionTag, field string) bool

	// getBeanValue 获取符合要求的 Bean，并且确保 Bean 完成自动注入过程，
	// 结果最多有一个，否则 panic，当允许结果为空时返回 false，否则 panic
	getBeanValue(v reflect.Value, tag SingletonTag, parent reflect.Value, field string) bool
}

// wiringStack 注入堆栈
type wiringStack struct {
	stack *list.List
}

func newWiringStack() *wiringStack {
	return &wiringStack{
		stack: list.New(),
	}
}

// pushBack 添加一个 Bean 到尾部
func (s *wiringStack) pushBack(bd beanDefinition) {
	SpringLogger.Tracef("wiring %s", bd.Description())
	s.stack.PushBack(bd)
}

// popBack 删除尾部的 Bean
func (s *wiringStack) popBack() {
	e := s.stack.Remove(s.stack.Back())
	SpringLogger.Tracef("wired %s", e.(beanDefinition).Description())
}

// path 返回 Bean 注入的路径
func (s *wiringStack) path() (path string) {
	for e := s.stack.Front(); e != nil; e = e.Next() {
		w := e.Value.(beanDefinition)
		path += fmt.Sprintf("=> %s ↩\n", w.Description())
	}
	return path[:len(path)-1]
}

// defaultBeanAssembly beanAssembly 的默认实现
type defaultBeanAssembly struct {
	appCtx      *applicationContext
	wiringStack *wiringStack
	destroys    *list.List // 具有销毁函数的 Bean 的堆栈
}

// newDefaultBeanAssembly defaultBeanAssembly 的构造函数
func newDefaultBeanAssembly(appCtx *applicationContext) *defaultBeanAssembly {
	return &defaultBeanAssembly{
		appCtx:      appCtx,
		wiringStack: newWiringStack(),
		destroys:    list.New(),
	}
}

func (assembly *defaultBeanAssembly) applicationContext() ApplicationContext {
	return assembly.appCtx
}

// getBeanValue 获取符合要求的 Bean，并且确保 Bean 完成自动注入过程，结果最多有一个，否则 panic，当允许结果为空时返回 false，否则 panic
func (assembly *defaultBeanAssembly) getBeanValue(v reflect.Value, tag SingletonTag, parent reflect.Value, field string) bool {

	var (
		ok       bool
		beanType reflect.Type
	)

	if beanType, ok = validBean(v); !ok {
		panic(fmt.Errorf("receiver must be ref type, bean: \"%s\" field: %s", tag, field))
	}

	foundBeans := make([]*BeanDefinition, 0)

	cache := assembly.appCtx.getTypeCacheItem(beanType)
	for _, bean := range cache.beans {
		// 不能将自身赋给自身的字段 && 类型全限定名匹配
		if bean.Value() != parent && bean.Match(tag.TypeName, tag.BeanName) {
			foundBeans = append(foundBeans, bean)
		}
	}

	// 扩展规则：如果指定了 Bean 名称则尝试通过名称获取以防没有通过 Export 显式导出接口
	if beanType.Kind() == reflect.Interface && tag.BeanName != "" {
		cache = assembly.appCtx.getNameCacheItem(tag.BeanName)
		for _, b := range cache.beans {
			// 不能将自身赋给自身的字段 && 类型匹配 && BeanName 匹配
			if b.Value() != parent && b.Type().AssignableTo(beanType) && b.Match(tag.TypeName, tag.BeanName) {
				found := false // 对结果进行排重
				for _, r := range foundBeans {
					if r == b {
						found = true
						break
					}
				}
				if !found {
					foundBeans = append(foundBeans, b)
					SpringLogger.Warnf("you should call Export() on %s", b.Description())
				}
			}
		}
	}

	// 没有找到，允许结果为空则返回 false，否则 panic
	if len(foundBeans) == 0 {
		if tag.Nullable {
			return false
		} else {
			panic(fmt.Errorf("can't find bean, bean: \"%s\" field: %s type: %s", tag, field, beanType))
		}
	}

	// 看看结果中有没有设置成主版本的，优先使用
	var primaryBeans []*BeanDefinition

	for _, bean := range foundBeans {
		if bean.primary {
			primaryBeans = append(primaryBeans, bean)
		}
	}

	if len(primaryBeans) > 1 { // 找到多于 1 个主版本则 panic
		msg := fmt.Sprintf("found %d primary beans, bean: \"%s\" field: %s type: %s [", len(primaryBeans), tag, field, beanType)
		for _, b := range primaryBeans {
			msg += "( " + b.Description() + " ), "
		}
		msg = msg[:len(msg)-2] + "]"
		panic(errors.New(msg))
	}

	var result *BeanDefinition

	if len(primaryBeans) == 0 {
		if len(foundBeans) > 1 { // 找到过个符合条件的 Bean 并且没有一个是主版本则 panic
			msg := fmt.Sprintf("found %d beans, bean: \"%s\" field: %s type: %s [", len(foundBeans), tag, field, beanType)
			for _, b := range foundBeans {
				msg += "( " + b.Description() + " ), "
			}
			msg = msg[:len(msg)-2] + "]"
			panic(errors.New(msg))
		}
		result = foundBeans[0]
	} else {
		result = primaryBeans[0]
	}

	// 对找到的 Bean 进行自动注入
	assembly.wireBeanDefinition(result, false)

	v0 := SpringUtils.PatchValue(v, assembly.appCtx.AllAccess())
	v0.Set(result.Value())
	return true
}

// collectBeans 收集符合要求的 Bean，结果可以是多个。自动模式下不对结果排序，指定模式会对结果排序。当允许结果为空时返回 false，否则 panic
func (assembly *defaultBeanAssembly) collectBeans(v reflect.Value, tag CollectionTag, field string) bool {

	t := v.Type()
	et := t.Elem()

	if !IsRefType(et.Kind()) { // 收集模式的数组元素必须是引用类型
		panic(errors.New("slice item in collection mode should be ref type"))
	}

	var result reflect.Value

	if len(tag.Items) == 0 { // 自动模式
		result = assembly.autoCollectBeans(t, et)
	} else { // 指定模式
		result = assembly.collectAndSortBeans(t, et, tag)
	}

	if result.Len() > 0 { // 找到多个符合条件的结果
		v = SpringUtils.PatchValue(v, assembly.appCtx.AllAccess())
		v.Set(result)
		return true
	}

	// 没有找到，允许结果为空则返回 false，否则 panic
	if tag.Nullable {
		return false
	} else {
		panic(fmt.Errorf("can't collect any beans: \"%s\" field: %s", tag, field))
	}
}

// findBeanFromCache 返回找到的符合条件的 Bean 在数组中的索引，找不到返回 -1。
func (assembly *defaultBeanAssembly) findBeanFromCache(beans []*BeanDefinition, tag SingletonTag, et reflect.Type) int {

	// 保存符合条件的 Bean 的索引
	var found []int

	// 查找符合条件的单例 Bean
	for i, d := range beans {
		if d.Match(tag.TypeName, tag.BeanName) {
			found = append(found, i)
		}
	}

	// 如果找到多个则 panic
	if len(found) > 1 {
		msg := fmt.Sprintf("found %d beans, bean: \"%s\" type: %s [", len(found), tag, et)
		for _, i := range found {
			msg += "( " + beans[i].Description() + " ), "
		}
		msg = msg[:len(msg)-2] + "]"
		panic(errors.New(msg))
	}

	// 如果必须找到符合条件的 Bean 则在没有找到时 panic
	if len(found) == 0 && !tag.Nullable {
		panic(fmt.Errorf("can't find bean, bean: \"%s\" type: %s", tag, et))
	}

	if len(found) > 0 {
		i := found[0]
		assembly.wireBeanDefinition(beans[i], false)
		return i
	}
	return -1
}

// collectAndSortBeans 收集符合条件的 Bean，并且根据指定的顺序对结果进行排序
func (assembly *defaultBeanAssembly) collectAndSortBeans(t reflect.Type, et reflect.Type, tag CollectionTag) reflect.Value {

	foundAny := false
	any := reflect.MakeSlice(t, 0, len(tag.Items))
	afterAny := reflect.MakeSlice(t, 0, len(tag.Items))
	beforeAny := reflect.MakeSlice(t, 0, len(tag.Items))

	// 只在单例类型中查找，数组类型的元素是否排序无法判断
	cache := assembly.appCtx.getTypeCacheItem(et)

	var beans []*BeanDefinition
	beans = append(beans, cache.beans...)

	for _, item := range tag.Items {

		// 是否遇到了"无序"标记
		if item.BeanName == "*" {
			if foundAny {
				panic(errors.New("more than one * in collection " + tag.String()))
			}
			foundAny = true
			continue
		}

		if i := assembly.findBeanFromCache(beans, item, et); i >= 0 {
			v := beans[i].Value()
			beans = append(beans[:i], beans[i+1:]...)
			if foundAny {
				afterAny = reflect.Append(afterAny, v)
			} else {
				beforeAny = reflect.Append(beforeAny, v)
			}
		}
	}

	if foundAny {
		for _, d := range beans {
			any = reflect.Append(any, d.Value())
		}
	}

	n := beforeAny.Len() + any.Len() + afterAny.Len()
	result := reflect.MakeSlice(t, n, n)

	i := 0
	reflect.Copy(result.Slice(i, i+beforeAny.Len()), beforeAny)
	i += beforeAny.Len()
	reflect.Copy(result.Slice(i, i+any.Len()), any)
	i += any.Len()
	reflect.Copy(result.Slice(i, i+afterAny.Len()), afterAny)

	return result // TODO 当收集接口类型的 Bean 时对于没有显式导出接口的 Bean 是否也需要收集？
}

// autoCollectBeans 收集符合条件的 Bean，不对结果进行排序，不排序是因为目前看起来没有必要
func (assembly *defaultBeanAssembly) autoCollectBeans(t reflect.Type, et reflect.Type) reflect.Value {
	result := reflect.MakeSlice(t, 0, 0)

	// 查找可以精确匹配的数组类型
	cache := assembly.appCtx.getTypeCacheItem(t)
	for _, d := range cache.beans {
		for i := 0; i < d.Value().Len(); i++ {
			di := d.Value().Index(i)

			// 对数组的元素进行自动注入
			if di.Kind() == reflect.Struct { // 结构体数组
				assembly.wireSliceItem(di.Addr(), d)
			} else if di.Kind() == reflect.Ptr { // 结构体指针数组
				if de := di.Elem(); de.Kind() == reflect.Struct {
					assembly.wireSliceItem(di, d)
				}
			}

			result = reflect.Append(result, di)
		}
	}

	// 查找可以精确匹配的单例类型
	cache = assembly.appCtx.getTypeCacheItem(et)
	for _, d := range cache.beans {

		// 对找到的 Bean 进行自动注入
		assembly.wireBeanDefinition(d, false)
		result = reflect.Append(result, d.Value())
	}

	return result // TODO 当收集接口类型的 Bean 时对于没有显式导出接口的 Bean 是否也需要收集？
}

// wireSliceItem 对 slice 的元素值进行注入
func (assembly *defaultBeanAssembly) wireSliceItem(v reflect.Value, d beanDefinition) {
	bd := ValueToBeanDefinition(v)
	bd.file = d.getFile()
	bd.line = d.getLine()
	assembly.wireBeanDefinition(bd, false)
}

// wireBeanDefinition 对特定的 BeanDefinition 进行注入，onlyAutoWire 是否只注入而不进行属性绑定
func (assembly *defaultBeanAssembly) wireBeanDefinition(bd beanDefinition, onlyAutoWire bool) {

	// Bean 是否已删除，已经删除的 Bean 不能再注入
	if bd.getStatus() == beanStatus_Deleted {
		panic(fmt.Errorf("bean: \"%s\" have been deleted", bd.BeanId()))
	}

	defer func() {
		if bd.getDestroy() != nil {
			assembly.destroys.Remove(assembly.destroys.Back())
		}
	}()

	// 如果有销毁函数则对其进行排序处理
	if bd.getDestroy() != nil {
		if curr, ok := bd.(*BeanDefinition); ok {
			de := assembly.appCtx.destroyer(curr)
			if i := assembly.destroys.Back(); i != nil {
				prev := i.Value.(*BeanDefinition)
				de.After(prev)
			}
			assembly.destroys.PushBack(curr)
		} else {
			panic(errors.New("let me known when it happened"))
		}
	}

	// Bean 是否已注入，已经注入的 Bean 无需再注入
	if bd.getStatus() == beanStatus_Wired {
		return
	}

	// 将当前 Bean 放入注入栈，以便检测循环依赖。
	assembly.wiringStack.pushBack(bd)

	// 正在注入的 Bean 再次注入则说明出现了循环依赖
	if bd.getStatus() == beanStatus_Wiring {
		if _, ok := bd.springBean().(*objectBean); !ok {
			panic(errors.New("found circle autowire"))
		}
		return
	}

	bd.setStatus(beanStatus_Wiring)

	// 首先对当前 Bean 的间接依赖项进行自动注入
	for _, selector := range bd.getDependsOn() {
		if bean, ok := assembly.appCtx.FindBean(selector); !ok {
			panic(fmt.Errorf("can't find bean: \"%v\"", selector))
		} else {
			assembly.wireBeanDefinition(bean, false)
		}
	}

	// 如果是成员方法 Bean，需要首先对它的父 Bean 进行自动注入
	if mBean, ok := bd.springBean().(*methodBean); ok {
		if l := len(mBean.parent); l > 1 {
			msg := fmt.Sprintf("found %d parent bean [", l)
			for _, b := range mBean.parent {
				msg += "( " + b.Description() + " ), "
			}
			msg = msg[:len(msg)-2] + "]"
			panic(errors.New(msg))
		}
		assembly.wireBeanDefinition(mBean.parent[0], false)
	}

	// 对当前 Bean 进行自动注入
	switch bean := bd.springBean().(type) {
	case *objectBean:
		assembly.wireObjectBean(bd, onlyAutoWire)
	case *constructorBean:
		fnValue := reflect.ValueOf(bean.fn)
		assembly.wireFunctionBean(fnValue, &bean.functionBean, bd)
	case *methodBean:
		fnValue := bean.parent[0].Value().MethodByName(bean.method)
		assembly.wireFunctionBean(fnValue, &bean.functionBean, bd)
	default:
		panic(errors.New("error spring bean type"))
	}

	// 如果用户设置了初始化函数则执行初始化函数
	if init := bd.getInit(); init != nil {
		if err := init.run(assembly); err != nil {
			panic(err)
		}
	}

	// 设置为已注入状态
	bd.setStatus(beanStatus_Wired)

	// 删除保存的注入帧
	assembly.wiringStack.popBack()
}

// wireObjectBean 对原始对象进行注入
func (assembly *defaultBeanAssembly) wireObjectBean(bd beanDefinition, onlyAutoWire bool) {
	st := bd.Type()
	switch sk := st.Kind(); sk {
	case reflect.Slice: // 对数组元素进行注入
		et := st.Elem()
		if ek := et.Kind(); ek == reflect.Struct { // 结构体数组
			v := bd.Value()
			for i := 0; i < v.Len(); i++ {
				iv := v.Index(i).Addr()
				assembly.wireSliceItem(iv, bd)
			}
		} else if ek == reflect.Ptr {
			it := et.Elem()
			if ik := it.Kind(); ik == reflect.Struct { // 结构体指针数组
				v := bd.Value()
				for p := 0; p < v.Len(); p++ {
					pv := v.Index(p)
					assembly.wireSliceItem(pv, bd)
				}
			}
		}
	case reflect.Ptr: // 对普通对象进行注入
		if et := st.Elem(); et.Kind() == reflect.Struct { // 结构体指针

			var etName string // 可能是内置类型
			if etName = et.Name(); etName == "" {
				etName = et.String()
			}

			sv := bd.Value()
			ev := sv.Elem()

			// 遍历 Bean 的每个字段，按照 tag 进行注入
			for i := 0; i < et.NumField(); i++ {

				// 避免父结构体有 value 标签时属性值重新解析
				fieldOnlyAutoWire := false

				ft := et.Field(i)
				fv := ev.Field(i)

				fieldName := etName + ".$" + ft.Name

				if !onlyAutoWire { // 防止 value 再次解析
					if tag, ok := ft.Tag.Lookup("value"); ok {
						fieldOnlyAutoWire = true
						bindStructField(assembly.appCtx, fv, tag, bindOption{
							allAccess: assembly.appCtx.AllAccess(),
							fieldName: fieldName,
						})
					}
				}

				// 处理 autowire 标签，autowire 与 inject 等价
				if beanId, ok := ft.Tag.Lookup("autowire"); ok {
					assembly.wireStructField(fv, beanId, sv, fieldName)
				}

				// 处理 inject 标签，inject 与 autowire 等价
				if beanId, ok := ft.Tag.Lookup("inject"); ok {
					assembly.wireStructField(fv, beanId, sv, fieldName)
				}

				// 只处理结构体类型的字段，防止递归所以不支持指针结构体字段
				if ft.Type.Kind() == reflect.Struct {

					// 开放私有字段，但是不会更新其原有可见属性
					fv0 := SpringUtils.PatchValue(fv, assembly.appCtx.AllAccess())
					if fv0.CanSet() {

						// 对 Bean 的结构体进行递归注入
						b := ValueToBeanDefinition(fv0.Addr())
						b.file = bd.getFile()
						b.line = bd.getLine()
						fbd := &fieldBeanDefinition{b, fieldName}
						assembly.wireBeanDefinition(fbd, fieldOnlyAutoWire)
					}
				}
			}
		}
	}
}

// wireFunctionBean 对函数定义的 Bean 进行注入
func (assembly *defaultBeanAssembly) wireFunctionBean(fnValue reflect.Value, fnBean *functionBean, bd beanDefinition) {

	// 获取输入参数
	var in []reflect.Value

	if fnBean.stringArg != nil {
		if r := fnBean.stringArg.Get(assembly, bd.FileLine()); len(r) > 0 {
			in = append(in, r...)
		}
	}

	if fnBean.optionArg != nil {
		if r := fnBean.optionArg.Get(assembly, bd.FileLine()); len(r) > 0 {
			in = append(in, r...)
		}
	}

	// 调用 Bean 函数
	out := fnValue.Call(in)

	// 获取第一个返回值
	val := out[0]

	if len(out) == 2 { // 如果有 error 返回则 panic
		if err := out[1].Interface(); err != nil {
			panic(fmt.Errorf("function bean: \"%s\" return error: %v", bd.FileLine(), err))
		}
	}

	// 将函数的返回值赋值给 Bean
	if IsRefType(val.Kind()) {
		// 如果实现接口的是值类型，那么需要转换成指针类型然后再赋值给接口
		if val.Kind() == reflect.Interface && IsValueType(val.Elem().Kind()) {
			ptrVal := reflect.New(val.Elem().Type())
			ptrVal.Elem().Set(val.Elem())
			fnBean.rValue.Set(ptrVal)
		} else {
			fnBean.rValue.Set(val)
		}
	} else {
		fnBean.rValue.Elem().Set(val)
	}

	if fnBean.Value().IsNil() {
		panic(fmt.Errorf("function bean: \"%s\" return nil", bd.FileLine()))
	}

	// 对函数的返回值进行自动注入
	b := &BeanDefinition{
		name:   bd.Name(),
		status: beanStatus_Default,
		file:   bd.getFile(),
		line:   bd.getLine(),
	}

	if fnBean.Type().Kind() == reflect.Interface {
		b.bean = newObjectBean(fnBean.Value().Elem())
	} else {
		b.bean = newObjectBean(fnBean.Value())
	}

	assembly.wireBeanDefinition(&fnValueBeanDefinition{b, bd}, false)
}

// wireStructField 对结构体的字段进行绑定
func (assembly *defaultBeanAssembly) wireStructField(v reflect.Value, tag string, parent reflect.Value, field string) {

	// tag 预处理，Bean 名称可以通过属性值指定
	if strings.HasPrefix(tag, "${") {
		s := ""
		sv := reflect.ValueOf(&s).Elem()
		bindStructField(assembly.appCtx, sv, tag, bindOption{})
		tag = s
	}

	if CollectionMode(tag) { // 收集模式，绑定对象必须是数组
		if v.Type().Kind() != reflect.Slice {
			panic(fmt.Errorf("field: %s should be slice", field))
		}
		assembly.collectBeans(v, ParseCollectionTag(tag), field)
	} else { // 单例模式
		assembly.getBeanValue(v, ParseSingletonTag(tag), parent, field)
	}
}
