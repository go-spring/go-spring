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

	"github.com/go-spring/go-spring-parent/spring-logger"
	"github.com/go-spring/go-spring-parent/spring-utils"
)

// beanAssembly Bean 组装车间
type beanAssembly interface {
	springContext() SpringContext
	collectBeans(v reflect.Value, beanTag BeanTag) bool
	getBeanValue(v reflect.Value, beanId BeanId, parent reflect.Value, field string) bool
}

// wiringStack 存储绑定中的 Bean
type wiringStack struct {
	stack *list.List
}

// newWiringStack wiringStack 的构造函数
func newWiringStack() *wiringStack {
	return &wiringStack{
		stack: list.New(),
	}
}

// pushBack 添加一个 Item 到尾部
func (s *wiringStack) pushBack(bd beanDefinition) {
	s.stack.PushBack(bd)
	SpringLogger.Tracef("wiring %s", bd.Description())
}

// popBack 删除尾部的 item
func (s *wiringStack) popBack() {
	e := s.stack.Remove(s.stack.Back())
	SpringLogger.Tracef("wired %s", e.(beanDefinition).Description())
}

// path 返回依赖注入的路径
func (s *wiringStack) path() (path string) {
	for e := s.stack.Front(); e != nil; e = e.Next() {
		w := e.Value.(beanDefinition)
		path += fmt.Sprintf("=> %s ↩\n", w.Description())
	}
	return path[:len(path)-1]
}

// defaultBeanAssembly beanAssembly 的默认版本
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

func (beanAssembly *defaultBeanAssembly) springContext() SpringContext {
	return beanAssembly.springCtx
}

// getBeanValue 根据 BeanId 查找 Bean 并返回 Bean 源的值
func (beanAssembly *defaultBeanAssembly) getBeanValue(v reflect.Value, beanId BeanId, parent reflect.Value, field string) bool {

	var (
		ok       bool
		beanType reflect.Type
	)

	if beanType, ok = ValidBeanValue(v); !ok {
		panic(fmt.Errorf("receiver must be ref type, bean: \"%s\" field: %s", beanId, field))
	}

	result := make([]*BeanDefinition, 0)

	m := beanAssembly.springCtx.getTypeCacheItem(beanType)
	for _, bean := range m.beans {
		// 不能将自身赋给自身的字段 && 类型全限定名匹配
		if bean.Value() != parent && bean.Match(beanId.TypeName, beanId.BeanName) {
			result = append(result, bean)
		}
	}

	// 对接口匹配开个绿灯，如果指定了 Bean 名称则尝试通过名称获取以防没有通过 Export 显示导出接口
	if beanType.Kind() == reflect.Interface && beanId.BeanName != "" {
		beanCache := beanAssembly.springCtx.beanCacheByName
		if cache, o := beanCache[beanId.BeanName]; o {
			for _, b := range cache.beans {
				// 不能将自身赋给自身的字段 && 类型匹配
				if b.Value() != parent && b.Type().AssignableTo(beanType) && b.Match(beanId.TypeName, beanId.BeanName) {
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

	count := len(result)

	// 没有找到
	if count == 0 {
		if beanId.Nullable {
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

// collectBeans 收集符合条件的 Bean 源 TODO 使用 beanTag
func (beanAssembly *defaultBeanAssembly) collectBeans(v reflect.Value, beanTag BeanTag) bool {

	t := v.Type()
	ev := reflect.New(t).Elem()

	// 查找数组类型
	{
		m := beanAssembly.springCtx.getTypeCacheItem(t)
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
		if et := t.Elem(); IsRefType(et.Kind()) {
			m := beanAssembly.springCtx.getTypeCacheItem(et)
			for _, d := range m.beans {
				beanAssembly.wireBeanDefinition(d, false)
				ev = reflect.Append(ev, d.Value())
			}
		}
	}

	if ev.Len() > 0 {
		v = SpringUtils.ValuePatchIf(v, beanAssembly.springCtx.AllAccess())
		v.Set(ev)
		return true
	}
	return false
}

// wireSliceItem 注入 slice 的元素值
func (beanAssembly *defaultBeanAssembly) wireSliceItem(v reflect.Value, d beanDefinition) {
	bd := ValueToBeanDefinition("", v)
	bd.file = d.getFile()
	bd.line = d.getLine()
	beanAssembly.wireBeanDefinition(bd, false)
}

// wireBeanDefinition 绑定 BeanDefinition 指定的 Bean
func (beanAssembly *defaultBeanAssembly) wireBeanDefinition(bd beanDefinition, onlyAutoWire bool) {

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
	if init := bd.getInit(); init != nil {
		if err := init.run(beanAssembly.springCtx); err != nil {
			panic(err)
		}
	}

	bd.setStatus(beanStatus_Wired)

	// 删除保存的注入帧
	beanAssembly.wiringStack.popBack()
}

// wireObjectBean 对原始对象进行注入
func (beanAssembly *defaultBeanAssembly) wireObjectBean(bd beanDefinition, onlyAutoWire bool) {
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

				// 处理 inject 标签
				if beanId, ok := ft.Tag.Lookup("inject"); ok {
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
func (beanAssembly *defaultBeanAssembly) wireFunctionBean(fnValue reflect.Value, bean *functionBean, bd beanDefinition) {

	var in []reflect.Value

	if bean.stringArg != nil {
		if r := bean.stringArg.Get(beanAssembly, bd.FileLine()); len(r) > 0 {
			in = append(in, r...)
		}
	}

	if bean.optionArg != nil {
		if r := bean.optionArg.Get(beanAssembly, bd.FileLine()); len(r) > 0 {
			in = append(in, r...)
		}
	}

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

	beanAssembly.wireBeanDefinition(&delegateBeanDefinition{b, bd}, false)
}

// wireStructField 对结构体的字段进行绑定
func (beanAssembly *defaultBeanAssembly) wireStructField(v reflect.Value,
	tag string, parent reflect.Value, field string) {

	if beanTag := ParseBeanTag(tag); beanTag.CollectMode { // 收集模式

		// 收集模式的绑定对象必须是数组
		if v.Type().Kind() != reflect.Slice {
			panic(fmt.Errorf("field: %s should be slice", field))
		}

		ok := beanAssembly.collectBeans(v, beanTag)
		if !ok && !beanTag.Nullable { // 没找到且不能为空则 panic
			panic(fmt.Errorf("can't find bean: \"%s\" field: %s", tag, field))
		}

	} else { // 匹配模式，autowire:"" or autowire:"name"
		beanAssembly.getBeanValue(v, beanTag.Items[0], parent, field)
	}
}
