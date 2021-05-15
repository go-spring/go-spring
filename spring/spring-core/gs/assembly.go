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

	"github.com/go-spring/spring-core/arg"
	"github.com/go-spring/spring-core/cond"
	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/log"
	"github.com/go-spring/spring-core/sort"
	"github.com/go-spring/spring-core/util"
)

// wiringStack 注入栈。
type wiringStack []*BeanDefinition

// pushBack 添加一个即将注入的 bean 。
func (s *wiringStack) pushBack(b *BeanDefinition) {
	log.Tracef("wiring %s", b.Description())
	*s = append(*s, b)
}

// popBack 删除一个已经注入的 bean 。
func (s *wiringStack) popBack() {
	n := len(*s)
	b := (*s)[n-1]
	*s = (*s)[:n-1]
	log.Tracef("wired %s", b.Description())
}

// path 返回注入路径。
func (s wiringStack) path() (path string) {
	for _, b := range s {
		path += fmt.Sprintf("=> %s ↩\n", b.Description())
	}
	return path[:len(path)-1]
}

// beanAssembly 装配工作台。
type beanAssembly struct {
	c            *Container
	stack        wiringStack
	destroyers   *list.List // 具有销毁函数的 bean 的列表。
	destroyerMap map[string]*destroyer
}

func toAssembly(c *Container) *beanAssembly {
	return &beanAssembly{
		c:            c,
		stack:        make([]*BeanDefinition, 0),
		destroyers:   list.New(),
		destroyerMap: make(map[string]*destroyer),
	}
}

// Matches 条件成立返回 true，否则返回 false。
func (assembly *beanAssembly) Matches(cond cond.Condition) bool {
	return cond.Matches(assembly.c)
}

// Bind 根据 tag 的内容进行属性绑定。
func (assembly *beanAssembly) Bind(tag string, v reflect.Value) error {
	return assembly.c.p.Bind(v, conf.Tag(tag))
}

// saveDestroyer 某个 Bean 可能会被多个 Bean 依赖，因此需要排重处理。
func (assembly *beanAssembly) saveDestroyer(b *BeanDefinition) *destroyer {
	d, ok := assembly.destroyerMap[b.ID()]
	if !ok {
		d = &destroyer{current: b}
		assembly.destroyerMap[b.ID()] = d
	}
	return d
}

// sortDestroyers 对销毁函数进行排序
func (assembly *beanAssembly) sortDestroyers() *list.List {
	for _, d := range assembly.destroyerMap {
		assembly.destroyers.PushBack(d)
	}
	assembly.destroyers = sort.Triple(assembly.destroyers, getBeforeDestroyers)
	return assembly.destroyers
}

// getBean 获取 tag 对应的 bean 然后赋值给 v，因此 v 应该是一个未初始化的值。
func (assembly *beanAssembly) getBean(tag singletonTag, v reflect.Value) error {

	if !v.IsValid() {
		return fmt.Errorf("receiver must be ref type, bean:%q", tag)
	}

	t := v.Type()
	if !util.IsBeanType(t) {
		return fmt.Errorf("receiver must be ref type, bean:%q", tag)
	}

	// TODO 如何检测 v 是否初始化过呢？如果初始化过需要输出一行下面的日志。
	// log.Warnf("receiver should not be unassigned, bean:%q", tag)

	foundBeans := make([]*BeanDefinition, 0)

	cache := assembly.c.beansByType[t]
	for i := 0; i < len(cache); i++ {
		b := cache[i]
		if b.Match(tag.typeName, tag.beanName) {
			foundBeans = append(foundBeans, b)
		}
	}

	// 指定 bean 名称时通过名称获取，防止未通过 Export 方法导出接口。
	if t.Kind() == reflect.Interface && tag.beanName != "" {
		cache = assembly.c.beansByName[tag.beanName]
		for i := 0; i < len(cache); i++ {
			b := cache[i]
			if b.Type().AssignableTo(t) && b.Match(tag.typeName, tag.beanName) {
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
		return fmt.Errorf("can't find bean, bean:%q type:%q", tag, t)
	}

	// 优先使用设置成主版本的 bean
	var primaryBeans []*BeanDefinition

	for _, b := range foundBeans {
		if b.primary {
			primaryBeans = append(primaryBeans, b)
		}
	}

	if len(primaryBeans) > 1 {
		msg := fmt.Sprintf("found %d primary beans, bean:%q type:%q [", len(primaryBeans), tag, t)
		for _, b := range primaryBeans {
			msg += "( " + b.Description() + " ), "
		}
		msg = msg[:len(msg)-2] + "]"
		return errors.New(msg)
	}

	if len(primaryBeans) == 0 && len(foundBeans) > 1 {
		msg := fmt.Sprintf("found %d beans, bean:%q type:%q [", len(foundBeans), tag, t)
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

	// 确保找到的 bean 已经完成依赖注入。
	err := assembly.wireBean(result)
	if err != nil {
		return err
	}

	v.Set(result.Value())
	return nil
}

// collectBeans 收集 tag 对应的 bean，自动模式下不排序，指定模式下根据 tag 对结果进行排序。
func (assembly *beanAssembly) collectBeans(tag collectionTag, v reflect.Value) error {

	t := v.Type()
	if t.Kind() != reflect.Slice {
		return fmt.Errorf("should be slice in collection mode")
	}

	if !util.IsBeanType(t.Elem()) { // 收集模式的数组元素应当是引用类型
		return errors.New("item in collection mode should be ref type")
	}

	var (
		err error
		ret reflect.Value
	)

	if len(tag.beanTags) == 0 {
		ret, err = assembly.autoCollectBeans(t)
	} else {
		ret, err = assembly.collectAndSortBeans(tag, t)
	}

	if err != nil {
		return err
	}

	if ret.Len() > 0 {
		v.Set(ret)
		return nil
	}

	if tag.nullable {
		return nil
	}

	return fmt.Errorf("no beans collected for %q", tag)
}

// autoCollectBeans 收集 tag 对应的 bean，不对结果进行排序，因为没有指定排序条件。
func (assembly *beanAssembly) autoCollectBeans(t reflect.Type) (reflect.Value, error) {
	result := reflect.MakeSlice(t, 0, 0)

	// 查找精确匹配的数组类型。
	cache := assembly.c.beansByType[t]
	for i := 0; i < len(cache); i++ {
		b := cache[i]
		if err := assembly.wireBean(b); err != nil {
			return reflect.Value{}, err
		}
		result = reflect.AppendSlice(result, b.Value())
	}

	// 查找精确匹配的单例类型。
	cache = assembly.c.beansByType[t.Elem()]
	for i := 0; i < len(cache); i++ {
		b := cache[i]
		if err := assembly.wireBean(b); err != nil {
			return reflect.Value{}, err
		}
		result = reflect.Append(result, b.Value())
	}

	return result, nil // TODO 收集接口类型的 bean 时没有显式导出接口的 bean 是否也收集？
}

// findBean 返回 tag 对应的 bean 在数组中的索引，找不到返回 -1。
func findBean(beans []*BeanDefinition, tag singletonTag, t reflect.Type) (int, error) {

	var found []int
	for i, b := range beans {
		if b.Match(tag.typeName, tag.beanName) {
			found = append(found, i)
		}
	}

	if len(found) > 1 {
		msg := fmt.Sprintf("found %d beans, bean:%q type:%q [", len(found), tag, t)
		for _, i := range found {
			msg += "( " + beans[i].Description() + " ), "
		}
		msg = msg[:len(msg)-2] + "]"
		return -1, errors.New(msg)
	}

	if len(found) > 0 {
		i := found[0]
		return i, nil
	}

	if tag.nullable {
		return -1, nil
	}

	return -1, fmt.Errorf("can't find bean, bean:%q type:%q", tag, t)
}

// collectAndSortBeans 收集 tag 对应的 bean，并根据 tag 的内容对结果进行排序。
func (assembly *beanAssembly) collectAndSortBeans(tag collectionTag,
	t reflect.Type) (reflect.Value, error) {

	et := t.Elem()
	foundAny := false

	any := reflect.MakeSlice(t, 0, len(tag.beanTags))
	afterAny := reflect.MakeSlice(t, 0, len(tag.beanTags))
	beforeAny := reflect.MakeSlice(t, 0, len(tag.beanTags))

	beans := make([]*BeanDefinition, 0)

	// 只收集数组元素对应的 bean，因为数组 bean 无法进行排序。
	cache := assembly.c.beansByType[et]
	for i := 0; i < len(cache); i++ {
		beans = append(beans, cache[i])
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

		index, err := findBean(beans, item, et)
		if err != nil {
			return reflect.Value{}, err
		}
		if index < 0 {
			continue
		}

		if err = assembly.wireBean(beans[index]); err != nil {
			return reflect.Value{}, err
		}

		v := beans[index].Value()
		beans = append(beans[:index], beans[index+1:]...)
		if foundAny {
			afterAny = reflect.Append(afterAny, v)
		} else {
			beforeAny = reflect.Append(beforeAny, v)
		}
	}

	if foundAny {
		for _, b := range beans {
			any = reflect.Append(any, b.Value())
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

	return result, nil // TODO 收集接口类型的 bean 时没有显式导出接口的 bean 是否也收集？
}

// wireBean 对 bean 进行注入，同时追踪其注入路径。如果 bean 有初始化函数，则在注入完成之后
// 执行其初始化函数。如果 bean 依赖了其他 bean，则首先尝试获取这些 bean 然后对它们进行注入。
func (assembly *beanAssembly) wireBean(b *BeanDefinition) error {

	if b.status == Deleted {
		return fmt.Errorf("bean:%q have been deleted", b.ID())
	}

	if assembly.c.state == Refreshed && b.status == Wired {
		return nil
	}

	defer func() {
		if b.destroy != nil {
			assembly.destroyers.Remove(assembly.destroyers.Back())
		}
	}()

	// 对注入路径上的销毁函数进行排序。
	if b.destroy != nil {
		d := assembly.saveDestroyer(b)
		if i := assembly.destroyers.Back(); i != nil {
			d.after(i.Value.(*BeanDefinition))
		}
		assembly.destroyers.PushBack(b)
	}

	if b.status == Wired {
		return nil
	}

	// 将当前 bean 放入注入栈，以便检测循环依赖。
	assembly.stack.pushBack(b)

	if b.status == Wiring {
		if b.f != nil { // 构造函数 bean 出现循环依赖。
			return errors.New("found circle autowire")
		}
		return nil
	}

	b.status = Wiring

	// 对当前 bean 的间接依赖项进行注入。
	for _, s := range b.dependsOn {
		d, err := assembly.c.Find(s)
		if err != nil {
			return err
		}
		if n := len(d); n != 1 {
			return fmt.Errorf("found %d bean(s) for:%q", n, s)
		}
		err = assembly.wireBean(d[0].(*BeanDefinition))
		if err != nil {
			return err
		}
	}

	v, err := assembly.getBeanValue(b)
	if err != nil {
		return err
	}

	err = assembly.wireBeanValue(v)
	if err != nil {
		return err
	}

	// 执行 bean 的初始化函数。
	if b.init != nil {
		_, err = b.init.Call(assembly, arg.Receiver(b.Value()))
		if err != nil {
			return err
		}
	}

	b.status = Wired
	assembly.stack.popBack()
	return nil
}

// getBeanValue 获取 bean 的值，如果是构造函数 bean 则执行其构造函数然后返回执行结果。
func (assembly *beanAssembly) getBeanValue(b *BeanDefinition) (reflect.Value, error) {

	if b.f == nil {
		return b.Value(), nil
	}

	out, err := b.f.Call(assembly)
	if err != nil {
		return reflect.Value{}, fmt.Errorf("constructor bean:%q return error: %v", b.FileLine(), err)
	}

	// 构造函数的返回值为值类型时 b.Type() 返回其指针类型。
	if val := out[0]; util.IsBeanType(val.Type()) {
		// 如果实现接口的是值类型，那么需要转换成指针类型然后再赋值给接口。
		if !val.IsNil() && val.Kind() == reflect.Interface && util.IsValueType(val.Elem().Type()) {
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
		return reflect.Value{}, fmt.Errorf("constructor bean:%q return nil", b.FileLine())
	}

	v := b.Value()
	// 结果以接口类型返回时需要将原始值取出来才能进行注入。
	if b.Type().Kind() == reflect.Interface {
		v = v.Elem()
	}
	return v, nil
}

// wireBeanValue 对 v 进行属性绑定和依赖注入，v 应该是一个已经初始化的值。
func (assembly *beanAssembly) wireBeanValue(v reflect.Value) error {

	t := v.Type()

	// 数组 bean 的每个元素单独注入。
	if t.Kind() == reflect.Slice {
		for i := 0; i < v.Len(); i++ {
			if ev := v.Index(i); ev.IsValid() {
				err := assembly.wireBeanValue(ev)
				if err != nil {
					return err
				}
			}
		}
		return nil
	}

	ev := v
	if t.Kind() == reflect.Ptr {
		ev = v.Elem()
	}

	// 如整数指针类型的 bean 是无需注入的。
	if ev.Kind() != reflect.Struct {
		return nil
	}

	// 属性绑定不是单纯的递归，需要单独处理。
	err := assembly.c.p.Bind(ev)
	if err != nil {
		return err
	}

	return assembly.wireStruct(ev)
}

// wireStruct 对结构体进行依赖注入，需要注意的是这里不需要进行属性绑定。
func (assembly *beanAssembly) wireStruct(v reflect.Value) error {

	t := v.Type()
	typeName := t.Name()
	if typeName == "" { // 简单类型没有名字
		typeName = t.String()
	}

	for i := 0; i < t.NumField(); i++ {

		ft := t.Field(i)
		fv := v.Field(i)

		if !fv.CanInterface() {
			fv = util.PatchValue(fv)
		}

		// 支持 autowire 和 inject 两种注入标签。
		tag, ok := ft.Tag.Lookup("autowire")
		if !ok {
			tag, ok = ft.Tag.Lookup("inject")
		}
		if ok {
			err := assembly.Autowire(tag, fv)
			if err != nil {
				fieldName := typeName + "." + ft.Name
				return fmt.Errorf("%q wired error: %s", fieldName, err.Error())
			}
		}

		// 递归处理结构体字段，指针字段不可以因为可能出现无限循环。
		if ft.Type.Kind() == reflect.Struct {
			err := assembly.wireStruct(fv)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Autowire 根据 tag 的内容自动判断注入模式，是单例模式，还是收集模式。
func (assembly *beanAssembly) Autowire(tag string, v reflect.Value) error {

	// tag 预处理，可以通过属性值进行指定。
	if strings.HasPrefix(tag, "${") {
		s := ""
		err := assembly.c.p.Bind(&s, conf.Tag(tag))
		if err != nil {
			return err
		}
		tag = s
	}

	if !collectionMode(tag) {
		return assembly.getBean(parseSingletonTag(tag), v)
	}
	return assembly.collectBeans(parseCollectionTag(tag), v)
}
