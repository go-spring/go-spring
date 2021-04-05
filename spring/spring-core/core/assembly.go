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

package core

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

// pushBack 添加一个 Bean 到尾部
func (s *wiringStack) pushBack(bd beanDefinition) {
	log.Tracef("wiring %s", bd.Description())
	*s = append(*s, bd)
}

// popBack 删除尾部的 Bean
func (s *wiringStack) popBack() {
	n := len(*s)
	bd := (*s)[n-1]
	*s = (*s)[:n-1]
	log.Tracef("wired %s", bd.(beanDefinition).Description())
}

// path 返回 Bean 注入的路径
func (s wiringStack) path() (path string) {
	for _, bd := range s {
		path += fmt.Sprintf("=> %s ↩\n", bd.Description())
	}
	return path[:len(path)-1]
}

type beanAssembly struct {
	ctx      *appCtx
	stack    wiringStack
	destroys *list.List // 具有销毁函数的 Bean 的堆栈
}

// toAssembly beanAssembly 的构造函数
func toAssembly(appCtx *appCtx) *beanAssembly {
	return &beanAssembly{
		ctx:      appCtx,
		stack:    make([]beanDefinition, 0),
		destroys: list.New(),
	}
}

// Matches 条件表达式成立返回 true
func (assembly *beanAssembly) Matches(cond cond.Condition) bool {
	return cond.Matches(assembly.ctx)
}

// BindStructField 对结构体的字段进行属性绑定
func (assembly *beanAssembly) BindValue(v reflect.Value, str string) error {
	return conf.BindValue(assembly.ctx.properties, v, str, conf.BindOption{})
}

// getBeanValue 获取符合要求的 Bean，并且确保 Bean 完成自动注入过程。
func (assembly *beanAssembly) getBeanValue(v reflect.Value, tag SingletonTag, parent reflect.Value, field string) error {

	if !v.IsValid() {
		return fmt.Errorf("receiver must be ref type, bean:%q field:%q", tag, field)
	}

	beanType := v.Type()
	if !util.IsRefType(beanType.Kind()) {
		return fmt.Errorf("receiver must be ref type, bean:%q field:%q", tag, field)
	}

	foundBeans := make([]*BeanDefinition, 0)

	cache := assembly.ctx.getCacheByType(beanType)
	for i := 0; i < cache.Len(); i++ {
		b := cache.Get(i).(*BeanDefinition)
		// 不能将自身赋给自身的字段 && 类型全限定名匹配
		if b.Value() != parent && b.Match(tag.TypeName, tag.BeanName) {
			foundBeans = append(foundBeans, b)
		}
	}

	// 扩展规则：如果指定了 Bean 名称则尝试通过名称获取以防没有通过 Export 显式导出接口
	if beanType.Kind() == reflect.Interface && tag.BeanName != "" {
		cache = assembly.ctx.getCacheByName(tag.BeanName)
		for i := 0; i < cache.Len(); i++ {
			b := cache.Get(i).(*BeanDefinition)
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
					log.Warnf("you should call Export() on %s", b.Description())
				}
			}
		}
	}

	if len(foundBeans) == 0 {
		if tag.Nullable {
			return nil
		} else {
			return fmt.Errorf("can't find bean, bean:%q field:%q type:%q", tag, field, beanType)
		}
	}

	// 看看结果中有没有设置成主版本的，优先使用
	var primaryBeans []*BeanDefinition

	for _, b := range foundBeans {
		if b.primary {
			primaryBeans = append(primaryBeans, b)
		}
	}

	if len(primaryBeans) > 1 {
		msg := fmt.Sprintf("found %d primary beans, bean:%q field:%q type:%q [", len(primaryBeans), tag, field, beanType)
		for _, b := range primaryBeans {
			msg += "( " + b.Description() + " ), "
		}
		msg = msg[:len(msg)-2] + "]"
		return errors.New(msg)
	}

	var result *BeanDefinition

	if len(primaryBeans) == 0 {
		if len(foundBeans) > 1 {
			msg := fmt.Sprintf("found %d beans, bean:%q field:%q type:%q [", len(foundBeans), tag, field, beanType)
			for _, b := range foundBeans {
				msg += "( " + b.Description() + " ), "
			}
			msg = msg[:len(msg)-2] + "]"
			return errors.New(msg)
		}
		result = foundBeans[0]
	} else {
		result = primaryBeans[0]
	}

	// 对找到的 Bean 进行自动注入
	if err := assembly.wireBeanDefinition(result, false); err != nil {
		return err
	}
	util.PatchValue(v).Set(result.Value())
	return nil
}

// collectBeans 收集符合要求的 Bean，结果可以是多个，自动模式下不对结果排序，指定模式会对结果排序。
func (assembly *beanAssembly) collectBeans(v reflect.Value, tag collectionTag, field string) error {

	t := v.Type()
	et := t.Elem()

	if !util.IsRefType(et.Kind()) { // 收集模式的数组元素必须是引用类型
		return errors.New("slice item in collection mode should be ref type")
	}

	var (
		err error
		ret reflect.Value
	)

	if len(tag.Items) == 0 { // 自动模式
		ret, err = assembly.autoCollectBeans(t, et)
	} else { // 指定模式
		ret, err = assembly.collectAndSortBeans(t, et, tag)
	}

	if err != nil {
		return err
	}

	if ret.Len() > 0 {
		util.PatchValue(v).Set(ret)
		return nil
	}

	if tag.Nullable {
		return nil
	}

	return fmt.Errorf("no beans collected for bean:%q field:%q", tag, field)
}

// findBeanFromCache 返回找到的符合条件的 Bean 在数组中的索引，找不到返回 -1。
func (assembly *beanAssembly) findBeanFromCache(beans []*BeanDefinition, tag SingletonTag, et reflect.Type) (int, error) {

	// 保存符合条件的 Bean 的索引
	var found []int

	// 查找符合条件的单例 Bean
	for i, d := range beans {
		if d.Match(tag.TypeName, tag.BeanName) {
			found = append(found, i)
		}
	}

	if len(found) > 1 {
		msg := fmt.Sprintf("found %d beans, bean:%q type:%q [", len(found), tag, et)
		for _, i := range found {
			msg += "( " + beans[i].Description() + " ), "
		}
		msg = msg[:len(msg)-2] + "]"
		return -2, errors.New(msg)
	}

	if len(found) == 0 && !tag.Nullable {
		return -2, fmt.Errorf("can't find bean, bean:%q type:%q", tag, et)
	}

	if len(found) > 0 {
		i := found[0]
		if err := assembly.wireBeanDefinition(beans[i], false); err != nil {
			return -2, err
		}
		return i, nil
	}
	return -1, nil
}

// collectAndSortBeans 收集符合条件的 Bean，并且根据指定的顺序对结果进行排序
func (assembly *beanAssembly) collectAndSortBeans(t reflect.Type, et reflect.Type, tag collectionTag) (reflect.Value, error) {

	foundAny := false
	any := reflect.MakeSlice(t, 0, len(tag.Items))
	afterAny := reflect.MakeSlice(t, 0, len(tag.Items))
	beforeAny := reflect.MakeSlice(t, 0, len(tag.Items))

	beans := make([]*BeanDefinition, 0)

	// 只在单例类型中查找，数组类型的元素是否排序无法判断
	cache := assembly.ctx.getCacheByType(et)
	for i := 0; i < cache.Len(); i++ {
		b := cache.Get(i).(*BeanDefinition)
		beans = append(beans, b)
	}

	for _, item := range tag.Items {

		// 是否遇到了"无序"标记 TODO 返回固定零值内存
		if item.BeanName == "*" {
			if foundAny {
				return reflect.Value{}, errors.New("more than one * in collection " + tag.String())
			}
			foundAny = true
			continue
		}

		idx, err := assembly.findBeanFromCache(beans, item, et)
		if err != nil {
			return reflect.Value{}, err
		}

		if idx >= 0 {
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
	cache := assembly.ctx.getCacheByType(t)
	for i := 0; i < cache.Len(); i++ {
		d := cache.Get(i).(*BeanDefinition)
		for i := 0; i < d.Value().Len(); i++ {
			di := d.Value().Index(i)

			// 对数组的元素进行自动注入
			if di.Kind() == reflect.Struct { // 结构体数组
				if err := assembly.wireSliceItem(di.Addr(), d); err != nil {
					return reflect.Value{}, err
				}
			} else if di.Kind() == reflect.Ptr { // 结构体指针数组
				if de := di.Elem(); de.Kind() == reflect.Struct {
					if err := assembly.wireSliceItem(di, d); err != nil {
						return reflect.Value{}, err
					}
				}
			}

			result = reflect.Append(result, di)
		}
	}

	// 查找可以精确匹配的单例类型，对找到的 Bean 进行自动注入
	cache = assembly.ctx.getCacheByType(et)
	for i := 0; i < cache.Len(); i++ {
		d := cache.Get(i).(*BeanDefinition)
		if err := assembly.wireBeanDefinition(d, false); err != nil {
			return reflect.Value{}, err
		}
		result = reflect.Append(result, d.Value())
	}

	return result, nil // TODO 当收集接口类型的 Bean 时对于没有显式导出接口的 Bean 是否也需要收集？
}

// wireSliceItem 对 slice 的元素值进行注入
func (assembly *beanAssembly) wireSliceItem(v reflect.Value, d beanDefinition) error {
	bd := NewBean(v, d.getFile(), d.getLine())
	return assembly.wireBeanDefinition(bd, false)
}

// wireBeanDefinition 对特定的 bean.BeanDefinition 进行注入，onlyAutoWire 是否只注入而不进行属性绑定
func (assembly *beanAssembly) wireBeanDefinition(bd beanDefinition, onlyAutoWire bool) error {

	// Bean 是否已删除，已经删除的 Bean 不能再注入
	if bd.getStatus() == Deleted {
		return fmt.Errorf("bean:%q have been deleted", bd.BeanId())
	}

	// 如果刷新阶段已完成并且 Bean 已经注入则无需再次进行下面的步骤
	if assembly.ctx.state == 2 && bd.getStatus() == Wired {
		return nil
	}

	defer func() {
		if bd.getDestroy() != nil {
			assembly.destroys.Remove(assembly.destroys.Back())
		}
	}()

	// 如果有销毁函数则对其进行排序处理
	if bd.getDestroy() != nil {
		if curr, ok := bd.(*BeanDefinition); ok {
			de := assembly.ctx.destroyer(curr)
			if i := assembly.destroys.Back(); i != nil {
				prev := i.Value.(*BeanDefinition)
				de.After(prev)
			}
			assembly.destroys.PushBack(curr)
		} else {
			return errors.New("let me known when it happened")
		}
	}

	// Bean 是否已注入，已经注入的 Bean 无需再注入
	if bd.getStatus() == Wired {
		return nil
	}

	// 将当前 Bean 放入注入栈，以便检测循环依赖。
	assembly.stack.pushBack(bd)

	// 正在注入的 Bean 再次注入则说明出现了循环依赖
	if bd.getStatus() == Wiring {
		if bd.getFactory() != nil {
			return errors.New("found circle autowire")
		}
		return nil
	}

	bd.setStatus(Wiring)

	// 首先对当前 Bean 的间接依赖项进行自动注入
	for _, selector := range bd.getDependsOn() {
		b, err := assembly.ctx.FindBean(selector)
		if err != nil {
			return err
		}
		if n := len(b); n != 1 {
			return fmt.Errorf("found %d bean(s) for:%q", n, selector)
		}
		if err = assembly.wireBeanDefinition(b[0].(beanDefinition), false); err != nil {
			return err
		}
	}

	// 对当前 Bean 进行自动注入
	if bd.getFactory() == nil {
		if err := assembly.wireObjectBean(bd, onlyAutoWire); err != nil {
			return err
		}
	} else {
		if err := assembly.wireConstructorBean(bd); err != nil {
			return err
		}
	}

	// 如果用户设置了初始化函数则执行初始化函数
	if init := bd.getInit(); init != nil {
		if err := init.Run(assembly, bd.Value()); err != nil {
			return err
		}
	}

	// 设置为已注入状态
	bd.setStatus(Wired)

	// 删除保存的注入帧
	assembly.stack.popBack()
	return nil
}

// wireObjectBean 对原始对象进行注入
func (assembly *beanAssembly) wireObjectBean(bd beanDefinition, onlyAutoWire bool) error {
	st := bd.Type()
	switch sk := st.Kind(); sk {
	case reflect.Slice: // 对数组元素进行注入
		et := st.Elem()
		if ek := et.Kind(); ek == reflect.Struct { // 结构体数组
			v := bd.Value()
			for i := 0; i < v.Len(); i++ {
				iv := v.Index(i).Addr()
				if err := assembly.wireSliceItem(iv, bd); err != nil {
					return err
				}
			}
		} else if ek == reflect.Ptr {
			it := et.Elem()
			if ik := it.Kind(); ik == reflect.Struct { // 结构体指针数组
				v := bd.Value()
				for p := 0; p < v.Len(); p++ {
					pv := v.Index(p)
					if err := assembly.wireSliceItem(pv, bd); err != nil {
						return err
					}
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
						err := conf.BindValue(assembly.ctx.properties, fv, tag, conf.BindOption{Path: fieldName})
						if err != nil {
							return err
						}
					}
				}

				// 处理 autowire 标签，autowire 与 inject 等价
				if beanId, ok := ft.Tag.Lookup("autowire"); ok {
					if err := assembly.wireStructField(fv, beanId, sv, fieldName); err != nil {
						return err
					}
				}

				// 处理 inject 标签，inject 与 autowire 等价
				if beanId, ok := ft.Tag.Lookup("inject"); ok {
					if err := assembly.wireStructField(fv, beanId, sv, fieldName); err != nil {
						return err
					}
				}

				// 只处理结构体类型的字段，防止递归所以不支持指针结构体字段
				if ft.Type.Kind() == reflect.Struct {
					// 开放私有字段，但是不会更新其原有可见属性
					if fv0 := util.PatchValue(fv); fv0.CanSet() {
						// 对 Bean 的结构体进行递归注入
						b := NewBean(fv0.Addr(), bd.getFile(), bd.getLine())
						fbd := &fieldBeanDefinition{b, fieldName}
						if err := assembly.wireBeanDefinition(fbd, fieldOnlyAutoWire); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	return nil
}

func (assembly *beanAssembly) wireConstructorBean(bd beanDefinition) error {

	out, err := bd.getFactory().Call(assembly)
	if err != nil {
		return fmt.Errorf("ctor bean:%q return error: %v", bd.FileLine(), err)
	}

	// 构造函数的返回值为值类型时 bd.Type() 返回其指针类型。
	if val := out[0]; util.IsRefType(val.Kind()) {
		// 如果实现接口的是值类型，那么需要转换成指针类型然后再赋值给接口。
		if val.Kind() == reflect.Interface && util.IsValueType(val.Elem().Kind()) {
			v := reflect.New(val.Elem().Type())
			v.Elem().Set(val.Elem())
			bd.Value().Set(v)
		} else {
			bd.Value().Set(val)
		}
	} else {
		bd.Value().Elem().Set(val)
	}

	if bd.Value().IsNil() {
		return fmt.Errorf("ctor bean:%q return nil", bd.FileLine())
	}

	// 对函数的返回值进行自动注入
	var beanValue reflect.Value
	if bd.Type().Kind() == reflect.Interface {
		beanValue = bd.Value().Elem()
	} else {
		beanValue = bd.Value()
	}

	b := NewBean(beanValue, bd.getFile(), bd.getLine()).Name(bd.BeanName())
	return assembly.wireBeanDefinition(&fnValueBeanDefinition{BeanDefinition: b, f: bd}, false)
}

// WireValue 对结构体的字段进行绑定
func (assembly *beanAssembly) WireValue(v reflect.Value, tag string) error {
	return assembly.wireStructField(v, tag, reflect.Value{}, "")
}

func (assembly *beanAssembly) wireStructField(v reflect.Value, tag string, Parent reflect.Value, field string) error {

	// tag 预处理，Bean 名称可以通过属性值指定
	if strings.HasPrefix(tag, "${") {
		s := ""
		sv := reflect.ValueOf(&s).Elem()
		err := conf.BindValue(assembly.ctx.properties, sv, tag, conf.BindOption{})
		util.Panic(err).When(err != nil)
		tag = s
	}

	if CollectionMode(tag) { // 收集模式，绑定对象必须是数组
		if v.Type().Kind() != reflect.Slice {
			return fmt.Errorf("field: %s should be slice", field)
		}
		return assembly.collectBeans(v, ParseCollectionTag(tag), field)
	}
	return assembly.getBeanValue(v, parseSingletonTag(tag), Parent, field)
}

type fieldBeanDefinition struct {
	*BeanDefinition
	field string // 字段名称
}

// Description 返回 Bean 的详细描述
func (d *fieldBeanDefinition) Description() string {
	return fmt.Sprintf("%s field: %s %s", d.getClass(), d.field, d.FileLine())
}

type fnValueBeanDefinition struct {
	*BeanDefinition
	f beanDefinition // 函数 Bean 定义
}

// Description 返回 Bean 的详细描述
func (d *fnValueBeanDefinition) Description() string {
	return fmt.Sprintf("%s value %s", d.f.getClass(), d.f.FileLine())
}
