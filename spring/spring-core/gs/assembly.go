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

package gs

import (
	"container/list"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-spring/spring-core/cond"
	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/log"
	"github.com/go-spring/spring-core/util"
)

// wiringStack 注入堆栈
type wiringStack []beanDefinition

// pushBack 添加一个即将注入的 Bean 。
func (s *wiringStack) pushBack(b beanDefinition) {
	log.Tracef("wiring %s", b.Description())
	*s = append(*s, b)
}

// popBack 删除一个已经注入的 Bean 。
func (s *wiringStack) popBack() {
	n := len(*s)
	b := (*s)[n-1]
	*s = (*s)[:n-1]
	log.Tracef("wired %s", b.Description())
}

// path 返回正在注入的 Bean 的集合。
func (s wiringStack) path() (path string) {
	for _, b := range s {
		path += fmt.Sprintf("=> %s ↩\n", b.Description())
	}
	return path[:len(path)-1]
}

// beanAssembly 装配工作台
type beanAssembly struct {
	c        *applicationContext
	stack    wiringStack
	destroys *list.List // 记录具有销毁函数的 Bean 的堆栈
}

// toAssembly beanAssembly 的构造函数。
func toAssembly(appCtx *applicationContext) *beanAssembly {
	return &beanAssembly{
		c:        appCtx,
		destroys: list.New(),
		stack:    make([]beanDefinition, 0),
	}
}

// Matches 返回 Condition 的判断结果。
func (assembly *beanAssembly) Matches(cond cond.Condition) bool {
	return cond.Matches(assembly.c)
}

// Bind 对值进行属性绑定。
func (assembly *beanAssembly) Bind(tag string, v reflect.Value) error {
	return conf.BindValue(assembly.c.p, v, tag, conf.BindOption{})
}

// Wire 对值进行依赖注入。
func (assembly *beanAssembly) Wire(tag string, v reflect.Value) error {
	return assembly.wireField(v, tag, reflect.Value{}, "")
}

// getBean 获取符合 tag 要求的 Bean，并且确保 Bean 已经完成依赖注入。
func (assembly *beanAssembly) getBean(v reflect.Value, tag singletonTag, parent reflect.Value, field string) error {

	if !v.IsValid() {
		return fmt.Errorf("receiver must be ref type, bean:%q field:%q", tag, field)
	}

	t := v.Type()
	if !util.RefType(t.Kind()) {
		return fmt.Errorf("receiver must be ref type, bean:%q field:%q", tag, field)
	}

	foundBeans := make([]*BeanDefinition, 0)

	cache := assembly.c.getCacheByType(t)
	for i := 0; i < cache.Len(); i++ {
		b := cache.Get(i).(*BeanDefinition)
		// 不能将自身赋值给自身的字段 && 类型全限定名匹配
		if b.Value() != parent && b.Match(tag.typeName, tag.beanName) {
			foundBeans = append(foundBeans, b)
		}
	}

	// 指定 beanName 时通过名称获取防止未通过 Export 显式导出接口
	if t.Kind() == reflect.Interface && tag.beanName != "" {
		cache = assembly.c.getCacheByName(tag.beanName)
		for i := 0; i < cache.Len(); i++ {
			b := cache.Get(i).(*BeanDefinition)
			// 不能将自身赋值给自身的字段 && 类型匹配 && beanName 匹配
			if b.Value() != parent && b.Type().AssignableTo(t) && b.Match(tag.typeName, tag.beanName) {
				found := false // 对结果排重
				for _, r := range foundBeans {
					if r == b {
						found = true
						break
					}
				}
				if !found {
					foundBeans = append(foundBeans, b)
					log.Warnf("you should call Export() on %s", b.Description())
				}
			}
		}
	}

	if len(foundBeans) == 0 {
		if tag.nullable {
			return nil
		}
		return fmt.Errorf("can't find bean, bean:%q field:%q type:%q", tag, field, t)
	}

	// 优先使用设置成主版本的 Bean
	var primaryBeans []*BeanDefinition

	for _, b := range foundBeans {
		if b.primary {
			primaryBeans = append(primaryBeans, b)
		}
	}

	if len(primaryBeans) > 1 {
		msg := fmt.Sprintf("found %d primary beans, bean:%q field:%q type:%q [", len(primaryBeans), tag, field, t)
		for _, b := range primaryBeans {
			msg += "( " + b.Description() + " ), "
		}
		msg = msg[:len(msg)-2] + "]"
		return errors.New(msg)
	}

	if len(primaryBeans) == 0 && len(foundBeans) > 1 {
		msg := fmt.Sprintf("found %d beans, bean:%q field:%q type:%q [", len(foundBeans), tag, field, t)
		for _, b := range foundBeans {
			msg += "( " + b.Description() + " ), "
		}
		msg = msg[:len(msg)-2] + "]"
		return errors.New(msg)
	}

	var result *BeanDefinition
	if len(primaryBeans) == 1 {
		result = primaryBeans[0]
	} else {
		result = foundBeans[0]
	}

	// 确保找到的 Bean 已经完成依赖注入
	err := assembly.wire(result, false)
	if err != nil {
		return err
	}

	util.PatchValue(v).Set(result.Value())
	return nil
}

// collectBeans 收集符合 tag 要求的 Bean，自动模式下不对结果排序，指定模式下按照 tag 规定的顺序对结果进行排序。
func (assembly *beanAssembly) collectBeans(v reflect.Value, tag collectionTag, field string) error {

	t := v.Type()
	et := t.Elem()

	if !util.RefType(et.Kind()) { // 收集模式的数组元素必须是引用类型
		return errors.New("slice item in collection mode should be ref type")
	}

	var (
		err    error
		result reflect.Value
	)

	if len(tag.beanTags) == 0 {
		result, err = assembly.autoCollectBeans(t, et)
	} else {
		result, err = assembly.collectAndSortBeans(t, et, tag)
	}

	if err != nil {
		return err
	}

	if result.Len() > 0 {
		util.PatchValue(v).Set(result)
		return nil
	}

	if tag.nullable {
		return nil
	}

	return fmt.Errorf("no beans collected for bean:%q field:%q", tag, field)
}

// findBean 返回找到的符合条件的 Bean 在数组中的索引，找不到返回 -1。
func findBean(beans []*BeanDefinition, tag singletonTag, et reflect.Type) (int, error) {

	// 保存符合条件的 Bean 的索引
	var found []int

	// 查找符合条件的单例 Bean
	for i, d := range beans {
		if d.Match(tag.typeName, tag.beanName) {
			found = append(found, i)
		}
	}

	if len(found) > 1 {
		msg := fmt.Sprintf("found %d beans, bean:%q type:%q [", len(found), tag, et)
		for _, i := range found {
			msg += "( " + beans[i].Description() + " ), "
		}
		msg = msg[:len(msg)-2] + "]"
		return -1, errors.New(msg)
	}

	if len(found) == 0 && !tag.nullable {
		return -1, fmt.Errorf("can't find bean, bean:%q type:%q", tag, et)
	}

	if len(found) > 0 {
		i := found[0]
		return i, nil
	}
	return -1, nil
}

// collectAndSortBeans 收集符合条件的 Bean，并且根据指定的顺序对结果进行排序
func (assembly *beanAssembly) collectAndSortBeans(t reflect.Type, et reflect.Type, tag collectionTag) (reflect.Value, error) {

	foundAny := false
	any := reflect.MakeSlice(t, 0, len(tag.beanTags))
	afterAny := reflect.MakeSlice(t, 0, len(tag.beanTags))
	beforeAny := reflect.MakeSlice(t, 0, len(tag.beanTags))

	beans := make([]*BeanDefinition, 0)

	// 只在单例类型中查找，数组类型的元素是否排序无法判断
	cache := assembly.c.getCacheByType(et)
	for i := 0; i < cache.Len(); i++ {
		b := cache.Get(i).(*BeanDefinition)
		beans = append(beans, b)
	}

	for _, item := range tag.beanTags {

		// 是否遇到了"无序"标记
		if item.beanName == "*" {
			if foundAny {
				return reflect.Value{}, fmt.Errorf("more than one * in collection %q", tag)
			}
			foundAny = true
			continue
		}

		idx, err := findBean(beans, item, et)
		if err != nil {
			return reflect.Value{}, err
		}

		if idx >= 0 {

			if err := assembly.wire(beans[idx], false); err != nil {
				return reflect.Value{}, err
			}

			v := beans[idx].Value()
			beans = append(beans[:idx], beans[idx+1:]...)
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

	return result, nil // TODO 当收集接口类型的 Bean 时对于没有显式导出接口的 Bean 是否也需要收集？
}

// autoCollectBeans 收集符合条件的 Bean，不对结果进行排序，不排序是因为目前看起来没有必要
func (assembly *beanAssembly) autoCollectBeans(t reflect.Type, et reflect.Type) (reflect.Value, error) {
	result := reflect.MakeSlice(t, 0, 0)

	// 查找可以精确匹配的数组类型
	cache := assembly.c.getCacheByType(t)
	for i := 0; i < cache.Len(); i++ {
		b := cache.Get(i).(*BeanDefinition)
		if err := assembly.wireSlice(b); err != nil {
			return reflect.Value{}, err
		}
		result = reflect.AppendSlice(result, b.Value())
	}

	// 查找可以精确匹配的单例类型，对找到的 Bean 进行自动注入
	cache = assembly.c.getCacheByType(et)
	for i := 0; i < cache.Len(); i++ {
		d := cache.Get(i).(*BeanDefinition)
		if err := assembly.wire(d, false); err != nil {
			return reflect.Value{}, err
		}
		result = reflect.Append(result, d.Value())
	}

	return result, nil // TODO 当收集接口类型的 Bean 时对于没有显式导出接口的 Bean 是否也需要收集？
}

// wire 对特定的 bean.BeanDefinition 进行注入，onlyAutoWire 是否只注入而不进行属性绑定
func (assembly *beanAssembly) wire(b beanDefinition, onlyAutoWire bool) error {

	// Bean 是否已删除，已经删除的 Bean 不能再注入
	if b.getStatus() == Deleted {
		return fmt.Errorf("bean:%q have been deleted", b.BeanId())
	}

	// 如果刷新阶段已完成并且 Bean 已经注入则无需再次进行下面的步骤
	if assembly.c.state == 2 && b.getStatus() == Wired {
		return nil
	}

	defer func() {
		if b.getDestroy() != nil {
			assembly.destroys.Remove(assembly.destroys.Back())
		}
	}()

	// 如果有销毁函数则对其进行排序处理
	if b.getDestroy() != nil {
		if curr, ok := b.(*BeanDefinition); ok {
			de := assembly.c.destroyer(curr)
			if i := assembly.destroys.Back(); i != nil {
				prev := i.Value.(*BeanDefinition)
				de.after(prev)
			}
			assembly.destroys.PushBack(curr)
		} else {
			return errors.New("let me known when it happened")
		}
	}

	// Bean 是否已注入，已经注入的 Bean 无需再注入
	if b.getStatus() == Wired {
		return nil
	}

	// 将当前 Bean 放入注入栈，以便检测循环依赖。
	assembly.stack.pushBack(b)

	// 正在注入的 Bean 再次注入则说明出现了循环依赖
	if b.getStatus() == Wiring {
		if b.getFactory() != nil {
			return errors.New("found circle autowire")
		}
		return nil
	}

	b.setStatus(Wiring)

	// 首先对当前 Bean 的间接依赖项进行自动注入
	for _, selector := range b.getDependsOn() {
		d, err := assembly.c.FindBean(selector)
		if err != nil {
			return err
		}
		if n := len(d); n != 1 {
			return fmt.Errorf("found %d bean(s) for:%q", n, selector)
		}
		if err = assembly.wire(d[0].(beanDefinition), false); err != nil {
			return err
		}
	}

	// 对当前 Bean 进行自动注入
	if b.getFactory() == nil {
		if err := assembly.wireObject(b, onlyAutoWire); err != nil {
			return err
		}
	} else {
		if err := assembly.wireFactory(b); err != nil {
			return err
		}
	}

	// 如果用户设置了初始化函数则执行初始化函数
	if init := b.getInit(); init != nil {
		if _, err := init.Call(assembly, b.Value()); err != nil {
			return err
		}
	}

	// 设置为已注入状态
	b.setStatus(Wired)

	// 删除保存的注入帧
	assembly.stack.popBack()
	return nil
}

func (assembly *beanAssembly) wireSlice(b beanDefinition) error {

	typ := 0 // 0 表示不可注入，1 表示结构体注入，2 表示结构体指针注入

	v := b.Value()
	et := v.Type().Elem()
	if ek := et.Kind(); ek == reflect.Struct { // 结构体数组
		typ = 1
	} else if ek == reflect.Ptr && et.Elem().Kind() == reflect.Struct { // 结构体指针数组
		typ = 2
	}

	for i := 0; i < v.Len(); i++ {
		var ev reflect.Value
		if typ == 1 { // 结构体数组
			ev = v.Index(i).Addr()
		} else if typ == 2 { // 结构体指针数组
			ev = v.Index(i)
		}
		if ev.IsValid() {
			eb := NewBean(ev, b.getFile(), b.getLine())
			if err := assembly.wire(eb, false); err != nil {
				return err
			}
		}
	}

	return nil
}

// wireObjectBean 对原始对象进行注入
func (assembly *beanAssembly) wireObject(b beanDefinition, onlyAutoWire bool) error {

	t := b.Type()
	if t.Kind() == reflect.Slice {
		return assembly.wireSlice(b)
	}

	if t.Kind() != reflect.Ptr {
		return nil
	}

	et := t.Elem()
	if et.Kind() != reflect.Struct { // 结构体指针
		return nil
	}

	etName := et.Name()
	if etName == "" { // 可能是内置类型
		etName = et.String()
	}

	v := b.Value()
	ev := v.Elem()

	// 遍历 Bean 的每个字段，按照 tag 进行注入
	for i := 0; i < et.NumField(); i++ {

		// 避免父结构体有 value 标签时属性值重新解析
		fieldOnlyAutoWire := false

		ft := et.Field(i)
		fv := ev.Field(i)

		fieldName := etName + "." + ft.Name

		if !onlyAutoWire { // 防止 value 再次解析
			if tag, ok := ft.Tag.Lookup("value"); ok {
				fieldOnlyAutoWire = true
				err := conf.BindValue(assembly.c.p, fv, tag, conf.BindOption{Path: fieldName})
				if err != nil {
					return err
				}
			}
		}

		// 处理 autowire 标签，autowire 与 inject 等价
		if beanId, ok := ft.Tag.Lookup("autowire"); ok {
			if err := assembly.wireField(fv, beanId, v, fieldName); err != nil {
				return err
			}
		}

		// 处理 inject 标签，inject 与 autowire 等价
		if beanId, ok := ft.Tag.Lookup("inject"); ok {
			if err := assembly.wireField(fv, beanId, v, fieldName); err != nil {
				return err
			}
		}

		// 只处理结构体类型的字段，防止递归所以不支持指针结构体字段
		if ft.Type.Kind() == reflect.Struct {
			// 开放私有字段，但是不会更新其原有可见属性
			if fv0 := util.PatchValue(fv); fv0.CanSet() {
				// 对 Bean 的结构体进行递归注入
				b := NewBean(fv0.Addr(), b.getFile(), b.getLine())
				fbd := &fieldBeanDefinition{beanDefinition: b, field: fieldName}
				if err := assembly.wire(fbd, fieldOnlyAutoWire); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (assembly *beanAssembly) wireFactory(b beanDefinition) error {

	out, err := b.getFactory().Call(assembly)
	if err != nil {
		return fmt.Errorf("ctor bean:%q return error: %v", b.FileLine(), err)
	}

	// 构造函数的返回值为值类型时 b.Type() 返回其指针类型。
	if val := out[0]; util.RefType(val.Kind()) {
		// 如果实现接口的是值类型，那么需要转换成指针类型然后再赋值给接口。
		if val.Kind() == reflect.Interface && util.ValueType(val.Elem().Kind()) {
			v := reflect.New(val.Elem().Type())
			v.Elem().Set(val.Elem())
			b.Value().Set(v)
		} else {
			b.Value().Set(val)
		}
	} else {
		b.Value().Elem().Set(val)
	}

	if b.Value().IsNil() {
		return fmt.Errorf("ctor bean:%q return nil", b.FileLine())
	}

	// 对函数的返回值进行自动注入
	var beanValue reflect.Value
	if b.Type().Kind() == reflect.Interface {
		beanValue = b.Value().Elem()
	} else {
		beanValue = b.Value()
	}

	d := NewBean(beanValue, b.getFile(), b.getLine()).WithName(b.BeanName())
	return assembly.wire(&fnValueBeanDefinition{beanDefinition: d, f: b}, false)
}

func (assembly *beanAssembly) wireField(v reflect.Value, tag string, parent reflect.Value, field string) error {

	// tag 预处理，Bean 名称可以通过属性值指定
	if strings.HasPrefix(tag, "${") {
		s := ""
		sv := reflect.ValueOf(&s).Elem()
		err := conf.BindValue(assembly.c.p, sv, tag, conf.BindOption{})
		if err != nil {
			return err
		}
		tag = s
	}

	if collectionMode(tag) { // 收集模式，绑定对象必须是数组
		if v.Type().Kind() != reflect.Slice {
			return fmt.Errorf("field: %s should be slice", field)
		}
		return assembly.collectBeans(v, parseCollectionTag(tag), field)
	}
	return assembly.getBean(v, parseSingletonTag(tag), parent, field)
}

type fieldBeanDefinition struct {
	beanDefinition
	field string // 字段名称
}

// Description 返回 Bean 的详细描述
func (d *fieldBeanDefinition) Description() string {
	return fmt.Sprintf("%s field:%q %s", d.getClass(), d.field, d.FileLine())
}

type fnValueBeanDefinition struct {
	beanDefinition
	f beanDefinition // 函数 Bean 定义
}

// Description 返回 Bean 的详细描述
func (d *fnValueBeanDefinition) Description() string {
	return fmt.Sprintf("%s value %s", d.f.getClass(), d.f.FileLine())
}
