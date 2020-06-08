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

	"github.com/go-spring/go-spring-parent/spring-logger"
	"github.com/go-spring/go-spring-parent/spring-utils"
)

// beanAssembly Bean 组装车间
type beanAssembly interface {
	springContext() SpringContext
	collectBeans(v reflect.Value, tag CollectionTag) bool
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
	springCtx   *defaultSpringContext
	wiringStack *wiringStack
}

// newDefaultBeanAssembly defaultBeanAssembly 的构造函数
func newDefaultBeanAssembly(springContext *defaultSpringContext) *defaultBeanAssembly {
	return &defaultBeanAssembly{
		springCtx:   springContext,
		wiringStack: newWiringStack(),
	}
}

func (assembly *defaultBeanAssembly) springContext() SpringContext {
	return assembly.springCtx
}

// getBeanValue 获取单例 Bean
func (assembly *defaultBeanAssembly) getBeanValue(v reflect.Value, tag SingletonTag, parent reflect.Value, field string) bool {

	var (
		ok       bool
		beanType reflect.Type
	)

	if beanType, ok = ValidBeanValue(v); !ok {
		panic(fmt.Errorf("receiver must be ref type, bean: \"%s\" field: %s", tag, field))
	}

	result := make([]*BeanDefinition, 0)

	m := assembly.springCtx.getTypeCacheItem(beanType)
	for _, bean := range m.beans {
		// 不能将自身赋给自身的字段 && 类型全限定名匹配
		if bean.Value() != parent && bean.Match(tag.TypeName, tag.BeanName) {
			result = append(result, bean)
		}
	}

	// 扩展规则：如果指定了 Bean 名称则尝试通过名称获取以防没有通过 Export 显示导出接口
	if beanType.Kind() == reflect.Interface && tag.BeanName != "" {
		beanCache := assembly.springCtx.beanCacheByName
		if cache, o := beanCache[tag.BeanName]; o {
			// 遍历所有的 Bean
			for _, b := range cache.beans {
				// 不能将自身赋给自身的字段 && 类型匹配 && BeanName 匹配
				if b.Value() != parent && b.Type().AssignableTo(beanType) && b.Match(tag.TypeName, tag.BeanName) {
					found := false // 排重
					for _, r := range result {
						if r == b {
							found = true
							break
						}
					}
					if !found {
						result = append(result, b)
						SpringLogger.Warnf("you should call Export() on %s", b.Description())
					}
				}
			}
		}
	}

	// 没有找到
	if len(result) == 0 {
		if tag.Nullable {
			return false
		} else {
			panic(fmt.Errorf("can't find bean, bean: \"%s\" field: %s type: %s", tag, field, beanType))
		}
	}

	var primaryBeans []*BeanDefinition

	for _, bean := range result {
		if bean.primary {
			primaryBeans = append(primaryBeans, bean)
		}
	}

	if len(primaryBeans) > 1 {
		msg := fmt.Sprintf("found %d primary beans, bean: \"%s\" field: %s type: %s [", len(primaryBeans), tag, field, beanType)
		for _, b := range primaryBeans {
			msg += "( " + b.Description() + " ), "
		}
		msg = msg[:len(msg)-2] + "]"
		panic(errors.New(msg))
	}

	if len(primaryBeans) == 0 {
		if len(result) > 1 {
			msg := fmt.Sprintf("found %d beans, bean: \"%s\" field: %s type: %s [", len(result), tag, field, beanType)
			for _, b := range result {
				msg += "( " + b.Description() + " ), "
			}
			msg = msg[:len(msg)-2] + "]"
			panic(errors.New(msg))
		}
		primaryBeans = append(primaryBeans, result[0])
	}

	// 依赖注入
	assembly.wireBeanDefinition(primaryBeans[0], false)

	v0 := SpringUtils.ValuePatchIf(v, assembly.springCtx.AllAccess())
	v0.Set(primaryBeans[0].Value())
	return true
}

// collectBeans 收集符合条件的 Bean，自动模式下不对结果排序，如果需要排序请使用指定模式。
func (assembly *defaultBeanAssembly) collectBeans(v reflect.Value, tag CollectionTag) bool {

	t := v.Type()
	et := t.Elem()

	// 收集模式的数组元素必须是引用类型，否则使用单例注入模式
	if !IsRefType(et.Kind()) {
		panic(errors.New("slice item in collection mode should be ref type"))
	}

	var ev reflect.Value

	if len(tag.Items) == 0 { // 自动模式
		ev = assembly.autoCollectBeans(t, et)
	} else { // 指定模式
		ev = assembly.collectAndSortBeans(t, et, tag)
	}

	if ev.Len() > 0 {
		v = SpringUtils.ValuePatchIf(v, assembly.springCtx.AllAccess())
		v.Set(ev)
		return true
	}
	return false
}

// collectAndSortBeans 收集符合条件的 Bean，并且根据指定的顺序对结果进行排序
func (assembly *defaultBeanAssembly) collectAndSortBeans(t reflect.Type, et reflect.Type, tag CollectionTag) reflect.Value {
	ev := reflect.MakeSlice(t, 0, len(tag.Items))

	// 只在单例类型中查找，数组类型的元素是否排序无法判断
	m := assembly.springCtx.getTypeCacheItem(et)
	for _, item := range tag.Items {

		var found []*BeanDefinition
		for _, d := range m.beans {
			if d.Match(item.TypeName, item.BeanName) {
				found = append(found, d)
			}
		}

		if len(found) > 1 {
			msg := fmt.Sprintf("found %d beans, bean: \"%s\" type: %s [", len(found), item, et)
			for _, b := range found {
				msg += "( " + b.Description() + " ), "
			}
			msg = msg[:len(msg)-2] + "]"
			panic(errors.New(msg))
		}

		if len(found) == 0 && !item.Nullable {
			panic(fmt.Errorf("can't find bean, bean: \"%s\" type: %s", item, et))
		}

		if len(found) > 0 {
			d := found[0]
			assembly.wireBeanDefinition(d, false)
			ev = reflect.Append(ev, d.Value())
		}
	}

	return ev
}

// autoCollectBeans 收集符合条件的 Bean，不对结果排序是因为目前看起来没有必要
func (assembly *defaultBeanAssembly) autoCollectBeans(t reflect.Type, et reflect.Type) reflect.Value {
	ev := reflect.MakeSlice(t, 0, 0)

	// 查找数组类型
	for _, d := range assembly.springCtx.getTypeCacheItem(t).beans {

		// 遍历数组元素
		for i := 0; i < d.Value().Len(); i++ {
			di := d.Value().Index(i)

			if di.Kind() == reflect.Struct { // 结构体数组
				assembly.wireSliceItem(di.Addr(), d)
			} else if di.Kind() == reflect.Ptr { // 结构体指针数组
				if de := di.Elem(); de.Kind() == reflect.Struct {
					assembly.wireSliceItem(di, d)
				}
			}
			ev = reflect.Append(ev, di)
		}
	}

	// 查找单例类型
	for _, d := range assembly.springCtx.getTypeCacheItem(et).beans {
		assembly.wireBeanDefinition(d, false)
		ev = reflect.Append(ev, d.Value())
	}

	return ev
}

// wireSliceItem 对 slice 的元素值进行注入
func (assembly *defaultBeanAssembly) wireSliceItem(v reflect.Value, d beanDefinition) {
	bd := ValueToBeanDefinition("", v)
	bd.file = d.getFile()
	bd.line = d.getLine()
	assembly.wireBeanDefinition(bd, false)
}

// wireBeanDefinition 对特定的 BeanDefinition 进行注入
func (assembly *defaultBeanAssembly) wireBeanDefinition(bd beanDefinition, onlyAutoWire bool) {

	// 是否已删除
	if bd.getStatus() == beanStatus_Deleted {
		panic(fmt.Errorf("bean: \"%s\" have been deleted", bd.BeanId()))
	}

	// 是否已绑定
	if bd.getStatus() == beanStatus_Wired {
		return
	}

	// 将当前 Bean 放入注入栈，以便检测循环依赖。
	assembly.wiringStack.pushBack(bd)

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
		if bean, ok := assembly.springCtx.FindBeanByName(selector); !ok {
			panic(fmt.Errorf("can't find bean: \"%v\"", selector))
		} else {
			assembly.wireBeanDefinition(bean, false)
		}
	}

	// 如果是成员方法 Bean，需要首先初始化它的父 Bean
	if mBean, ok := bd.springBean().(*methodBean); ok {
		assembly.wireBeanDefinition(mBean.parent, false)
	}

	switch bean := bd.springBean().(type) {
	case *objectBean:
		assembly.wireObjectBean(bd, onlyAutoWire)
	case *constructorBean:
		fnValue := reflect.ValueOf(bean.fn)
		assembly.wireFunctionBean(fnValue, &bean.functionBean, bd)
	case *methodBean:
		fnValue := bean.parent.Value().MethodByName(bean.method)
		assembly.wireFunctionBean(fnValue, &bean.functionBean, bd)
	default:
		panic(errors.New("unknown spring bean type"))
	}

	// 如果有则执行用户设置的初始化函数
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
	case reflect.Ptr:
		if et := st.Elem(); et.Kind() == reflect.Struct { // 结构体指针

			var etName string
			if etName = et.Name(); etName == "" {
				etName = et.String()
			}

			sv := bd.Value()
			ev := sv.Elem()

			for i := 0; i < et.NumField(); i++ {
				// 避免父结构体有 value 标签时重新解析
				fieldOnlyAutoWire := false

				ft := et.Field(i)
				fv := ev.Field(i)

				fieldName := etName + ".$" + ft.Name

				if !onlyAutoWire {
					if tag, ok := ft.Tag.Lookup("value"); ok {
						fieldOnlyAutoWire = true
						bindStructField(assembly.springCtx, fv, tag, bindOption{
							fieldName: fieldName,
							allAccess: assembly.springCtx.AllAccess(),
						})
					}
				}

				// 处理 autowire 标签
				if beanId, ok := ft.Tag.Lookup("autowire"); ok {
					assembly.wireStructField(fv, beanId, sv, fieldName)
				}

				// 处理 inject 标签
				if beanId, ok := ft.Tag.Lookup("inject"); ok {
					assembly.wireStructField(fv, beanId, sv, fieldName)
				}

				// 处理结构体类型的字段，防止递归所以不支持指针结构体字段
				if ft.Type.Kind() == reflect.Struct {

					// 开放私有字段，但是不会更新原有可见属性
					fv0 := SpringUtils.ValuePatchIf(fv, assembly.springCtx.AllAccess())
					if fv0.CanSet() {

						b := ValueToBeanDefinition("", fv0.Addr())
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
func (assembly *defaultBeanAssembly) wireFunctionBean(fnValue reflect.Value, bean *functionBean, bd beanDefinition) {

	// 获取输入参数
	var in []reflect.Value

	if bean.stringArg != nil {
		if r := bean.stringArg.Get(assembly, bd.FileLine()); len(r) > 0 {
			in = append(in, r...)
		}
	}

	if bean.optionArg != nil {
		if r := bean.optionArg.Get(assembly, bd.FileLine()); len(r) > 0 {
			in = append(in, r...)
		}
	}

	// 调用 Bean 函数
	out := fnValue.Call(in)

	// 获取第一个返回值
	val := out[0]

	// 检查是否有 error 返回
	if len(out) == 2 {
		if err := out[1].Interface(); err != nil {
			panic(fmt.Errorf("function bean: \"%s\" return error: %v", bd.FileLine(), err))
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
		panic(fmt.Errorf("function bean: \"%s\" return nil", bd.FileLine()))
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

	assembly.wireBeanDefinition(&fnValueBeanDefinition{b, bd}, false)
}

// wireStructField 对结构体的字段进行绑定
func (assembly *defaultBeanAssembly) wireStructField(v reflect.Value, str string,
	parent reflect.Value, field string) {

	if strings.HasPrefix(str, "${") { // tag 预处理
		s := new(string)
		sv := reflect.ValueOf(s).Elem()
		bindStructField(assembly.springCtx, sv, str, bindOption{})
		str = *s
	}

	if CollectMode(str) { // 收集模式

		// 收集模式的绑定对象必须是数组
		if v.Type().Kind() != reflect.Slice {
			panic(fmt.Errorf("field: %s should be slice", field))
		}

		tag := ParseCollectionTag(str)
		ok := assembly.collectBeans(v, tag)
		if !ok && !tag.Nullable { // 没找到且不能为空则 panic
			panic(fmt.Errorf("can't find bean: \"%s\" field: %s", tag, field))
		}

	} else { // 匹配模式，autowire:"" or autowire:"name"
		assembly.getBeanValue(v, ParseSingletonTag(str), parent, field)
	}
}
