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

// Package gs 实现了 go-spring 框架的基础骨架，包含 IoC 容器、基于 IoC 容器
// 的 App 以及全局 App 对象封装三个部分。
package gs

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"sort"
	"strings"
	"sync"

	"github.com/go-spring/spring-core/arg"
	"github.com/go-spring/spring-core/bean"
	"github.com/go-spring/spring-core/cond"
	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/log"
	"github.com/go-spring/spring-core/util"
)

type refreshState int

const (
	Unrefreshed = refreshState(0) // 未刷新
	Refreshing  = refreshState(1) // 正在刷新
	Refreshed   = refreshState(2) // 已刷新
)

// Container 是 go-spring 框架的基石，实现了 Martin Fowler 在 << Inversion
// of Control Containers and the Dependency Injection pattern >> 一文滥
// 觞的依赖注入的概念。原文的依赖注入仅仅是指对象(在 Java 中是指结构体实例)之间的
// 依赖关系处理，而有些 IoC 容器在实现时比如 Spring 还引入了对属性 property 的处
// 理，通常大家会用依赖注入统述上面两种概念，但实际上使用属性绑定来描述对 property
// 的处理更合适，因此 go-spring 在描述对 bean 的处理时要么单独使用依赖注入或属性
// 绑定，要么同时使用依赖注入和属性绑定。
type Container struct {
	p *conf.Properties

	state refreshState

	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	beans       []*BeanDefinition
	beansById   map[string]*BeanDefinition
	beansByName map[string][]*BeanDefinition
	beansByType map[reflect.Type][]*BeanDefinition

	configers  []*Configer
	destroyers []destroyer0
}

// New 创建 IoC 容器。
func New() *Container {

	ctx, cancel := context.WithCancel(context.Background())
	c := &Container{
		p:           conf.New(),
		ctx:         ctx,
		cancel:      cancel,
		beansById:   make(map[string]*BeanDefinition),
		beansByName: make(map[string][]*BeanDefinition),
		beansByType: make(map[reflect.Type][]*BeanDefinition),
	}

	c.Object(&pandora{c}).Export((*Pandora)(nil))
	return c
}

// callBeforeRefreshing 有些方法只能在 Refresh 开始前调用，比如 Object 。
func (c *Container) callBeforeRefreshing() {
	if c.state != Unrefreshed {
		panic(errors.New("should call before Refreshing"))
	}
}

// callAfterRefreshing 有些方法必须在 Refresh 开始后才能调用，比如 Wire 。
func (c *Container) callAfterRefreshing() {
	if c.state == Unrefreshed {
		panic(errors.New("should call after Refreshing"))
	}
}

// Load 从文件读取属性列表。
func (c *Container) Load(filename string) error {
	c.callBeforeRefreshing()
	return c.p.Load(filename)
}

// Property 设置 key 对应的属性值。
func (c *Container) Property(key string, value interface{}) {
	c.callBeforeRefreshing()
	c.p.Set(key, value)
}

// Object 注册对象形式的 bean ，包括接口、指针、channel、function 及其集合。
func (c *Container) Object(i interface{}) *BeanDefinition {
	return c.register(NewBean(reflect.ValueOf(i)))
}

// Provide 注册构造函数形式的 bean ，这里的构造函数泛指所有可以返回 bean 对象
// 的函数或方法( golang 里面方法是指带有接收者的函数)，当使用方法定义的构造函数
// 时，接收者被当做函数的第一个参数。另外，ctor 只能返回一个 bean 对象，当然也
// 支持返回一个额外的 error 对象。
func (c *Container) Provide(ctor interface{}, args ...arg.Arg) *BeanDefinition {
	return c.register(NewBean(ctor, args...))
}

func (c *Container) register(b *BeanDefinition) *BeanDefinition {
	c.callBeforeRefreshing()
	c.beans = append(c.beans, b)
	return b
}

// Config 注册配置函数，配置函数是指可以接受一些 bean 作为入参且没有返回值的函
// 数，使用场景大多是在 bean 初始化之后对 bean 进行二次配置，该机制可以作为框架
// 配置能力的有效补充，但是一定要慎用！
func (c *Container) Config(fn interface{}, args ...arg.Arg) *Configer {
	return c.config(NewConfiger(fn, args...))
}

func (c *Container) config(configer *Configer) *Configer {
	c.callBeforeRefreshing()
	c.configers = append(c.configers, configer)
	return configer
}

// find 查找符合条件的 bean，注意该函数只能保证返回的 bean 是有效的(即未被标
// 记为删除)的，而不能保证已经完成属性绑定和依赖注入。
func (c *Container) find(selector bean.Selector) ([]*BeanDefinition, error) {
	c.callAfterRefreshing()

	finder := func(fn func(*BeanDefinition) bool) ([]*BeanDefinition, error) {
		var result []*BeanDefinition
		for _, b := range c.beansById {
			if b.status == Resolving || !fn(b) {
				continue
			}
			if err := c.resolveBean(b); err != nil {
				return nil, err
			}
			if b.status == Deleted {
				continue
			}
			result = append(result, b)
		}
		return result, nil
	}

	t := reflect.TypeOf(selector)

	if t.Kind() == reflect.String {
		tag := parseSingletonTag(selector.(string))
		return finder(func(b *BeanDefinition) bool {
			return b.Match(tag.typeName, tag.beanName)
		})
	}

	if t.Kind() == reflect.Ptr {
		if e := t.Elem(); e.Kind() == reflect.Interface {
			t = e // 指 (*error)(nil) 形式的 bean 选择器
		}
	}

	return finder(func(b *BeanDefinition) bool {
		if !b.Type().AssignableTo(t) {
			return false
		}
		if t.Kind() != reflect.Interface {
			return true
		}
		_, ok := b.exports[t]
		return ok
	})
}

// Refresh 决议和组装所有的 configer 与 bean 。
func (c *Container) Refresh() {

	if c.state != Unrefreshed {
		panic(errors.New("container already refreshed"))
	}

	c.state = Refreshing

	err := c.resolveBeans()
	util.Panic(err).When(err != nil)

	stack := newWiringStack()

	defer func() {
		if len(stack.beans) > 0 {
			log.Infof("wiring path %s", stack.path())
		}
	}()

	err = c.runConfigers(stack)
	util.Panic(err).When(err != nil)

	err = c.wireBeans(stack)
	util.Panic(err).When(err != nil)

	c.destroyers = stack.sortDestroyers()
	c.state = Refreshed

	log.Info("container refreshed successfully")
}

func (c *Container) resolveBeans() error {
	for _, b := range c.beans {
		if err := c.resolveBean(b); err != nil {
			return err
		}
	}
	return nil
}

// resolveBean 决议 bean 是否是有效的，如果 bean 是无效的则呗标记为已删除。
func (c *Container) resolveBean(b *BeanDefinition) error {

	if b.status >= Resolving {
		return nil
	}

	b.status = Resolving

	if b.cond != nil {
		if ok, err := b.cond.Matches(&pandora{c}); err != nil {
			return err
		} else if !ok {
			b.status = Deleted
			return nil
		}
	}

	if d, ok := c.beansById[b.ID()]; ok {
		return fmt.Errorf("found duplicate beans [%s] [%s]", b, d)
	}

	log.Debugf("register %s name:%q type:%q %s", b.getClass(), b.Name(), b.Type(), b.FileLine())

	c.beansById[b.ID()] = b
	c.beansByName[b.name] = append(c.beansByName[b.name], b)
	c.beansByType[b.Type()] = append(c.beansByType[b.Type()], b)

	if err := c.export(b.Type(), b); err != nil {
		return err
	}

	for t := range b.exports {
		if !b.Type().Implements(t) {
			return fmt.Errorf("%s doesn't implement %s interface", b, t)
		}
		log.Debugf("register %s name:%q type:%q %s", b.getClass(), b.Name(), t, b.FileLine())
		c.beansByType[t] = append(c.beansByType[t], b)
	}

	b.status = Resolved
	return nil
}

// export 导出结构体指针类型 bean 使用 export 语法导出的接口。
func (c *Container) export(t reflect.Type, b *BeanDefinition) error {

	t = util.Indirect(t)
	if t.Kind() != reflect.Struct {
		return nil
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)

		_, export := f.Tag.Lookup("export")
		_, inject := f.Tag.Lookup("inject")
		_, autowire := f.Tag.Lookup("autowire")

		if !export {
			if f.Anonymous && !inject && !autowire {
				if err := c.export(f.Type, b); err != nil {
					return err
				}
			}
			continue
		}

		// 有 export 标签的必须是接口类型。
		if f.Type.Kind() != reflect.Interface {
			return errors.New("export can only use on interface")
		}

		// 不能导出需要注入的接口。
		if inject || autowire {
			return errors.New("inject or autowire can't use with export")
		}

		if err := b.export(f.Type); err != nil {
			return err
		}
	}
	return nil
}

func (c *Container) runConfigers(stack *wiringStack) error {

	configerList := list.New()
	for _, g := range c.configers {
		if g.cond != nil {
			if ok, err := g.cond.Matches(&pandora{c}); err != nil {
				return err
			} else if !ok {
				continue
			}
		}
		configerList.PushBack(g)
	}

	configerList = util.TripleSort(configerList, getBeforeList)
	for e := configerList.Front(); e != nil; e = e.Next() {
		g := e.Value.(*Configer)
		ctx := newArgContext(c, stack)
		if _, err := g.fn.Call(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (c *Container) wireBeans(stack *wiringStack) error {
	for _, b := range c.beansById {
		if err := c.wireBean(b, stack); err != nil {
			return err
		}
	}
	return nil
}

// Close 关闭容器，此方法必须在 Refresh 之后调用。该方法会触发 ctx 的 Done 信
// 号，然后等待所有 goroutine 结束，最后按照被依赖先销毁的原则执行所有的销毁函数。
func (c *Container) Close() {
	c.callAfterRefreshing()

	c.cancel()
	c.wg.Wait()

	log.Info("goroutines exited")

	c.runDestroyers()

	log.Info("container closed")
}

func (c *Container) runDestroyers() {
	for _, d := range c.destroyers {
		fnValue := reflect.ValueOf(d.fn)
		out := fnValue.Call([]reflect.Value{d.v})
		if len(out) > 0 && !out[0].IsNil() {
			log.Error(out[0].Interface().(error))
		}
	}
}

// Go 创建安全可等待的 goroutine，fn 要求的 ctx 对象由 Container 提供，当
// Container 关闭时 ctx 发出 Done 信号， fn 在接收到此信号后应当立即退出。
func (c *Container) Go(fn func(ctx context.Context)) {

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		defer func() {
			if r := recover(); r != nil {
				log.Errorf("%v, %s", r, debug.Stack())
			}
		}()

		fn(c.ctx)
	}()
}

type wiringStack struct {
	beans        []*BeanDefinition
	destroyers   *list.List
	destroyerMap map[string]*destroyer
}

func newWiringStack() *wiringStack {
	return &wiringStack{
		beans:        make([]*BeanDefinition, 0),
		destroyers:   list.New(),
		destroyerMap: make(map[string]*destroyer),
	}
}

// pushBack 添加一个即将注入的 bean 。
func (s *wiringStack) pushBack(b *BeanDefinition) {
	log.Tracef("wiring %s", b)
	s.beans = append(s.beans, b)
}

// popBack 删除一个已经注入的 bean 。
func (s *wiringStack) popBack() {
	n := len(s.beans)
	b := s.beans[n-1]
	s.beans = s.beans[:n-1]
	log.Tracef("wired %s", b)
}

// path 返回注入路径。
func (s *wiringStack) path() (path string) {
	for _, b := range s.beans {
		path += fmt.Sprintf("=> %s ↩\n", b)
	}
	return path[:len(path)-1]
}

// saveDestroyer 某个 Bean 可能会被多个 Bean 依赖，因此需要排重处理。
func (s *wiringStack) saveDestroyer(b *BeanDefinition) *destroyer {
	d, ok := s.destroyerMap[b.ID()]
	if !ok {
		d = &destroyer{current: b}
		s.destroyerMap[b.ID()] = d
	}
	return d
}

// sortDestroyers 对销毁函数进行排序
func (s *wiringStack) sortDestroyers() (ret []destroyer0) {
	destroyers := list.New()
	for _, d := range s.destroyerMap {
		destroyers.PushBack(d)
	}
	destroyers = util.TripleSort(destroyers, getBeforeDestroyers)
	for e := destroyers.Front(); e != nil; e = e.Next() {
		d := e.Value.(*destroyer).current
		ret = append(ret, destroyer0{d.destroy, d.Value()})
	}
	return ret
}

// getBean 获取 tag 对应的 bean 然后赋值给 v，因此 v 应该是一个未初始化的值。
func (c *Container) getBean(v reflect.Value, tag singletonTag, stack *wiringStack) error {

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

	cache := c.beansByType[t]
	for i := 0; i < len(cache); i++ {
		b := cache[i]
		if b.Match(tag.typeName, tag.beanName) {
			foundBeans = append(foundBeans, b)
		}
	}

	// 指定 bean 名称时通过名称获取，防止未通过 Export 方法导出接口。
	if t.Kind() == reflect.Interface && tag.beanName != "" {
		cache = c.beansByName[tag.beanName]
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
					log.Warnf("you should call Export() on %s", b)
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
			msg += "( " + b.String() + " ), "
		}
		msg = msg[:len(msg)-2] + "]"
		return errors.New(msg)
	}

	if len(primaryBeans) == 0 && len(foundBeans) > 1 {
		msg := fmt.Sprintf("found %d beans, bean:%q type:%q [", len(foundBeans), tag, t)
		for _, b := range foundBeans {
			msg += "( " + b.String() + " ), "
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
	err := c.wireBean(result, stack)
	if err != nil {
		return err
	}

	v.Set(result.Value())
	return nil
}

// wireBean 对 bean 进行注入，同时追踪其注入路径。如果 bean 有初始化函数，则在注入完成之后
// 执行其初始化函数。如果 bean 依赖了其他 bean，则首先尝试获取这些 bean 然后对它们进行注入。
func (c *Container) wireBean(b *BeanDefinition, stack *wiringStack) error {

	if b.status == Deleted {
		return fmt.Errorf("bean:%q have been deleted", b.ID())
	}

	if c.state == Refreshed && b.status == Wired {
		return nil
	}

	defer func() {
		if b.destroy != nil {
			stack.destroyers.Remove(stack.destroyers.Back())
		}
	}()

	// 对注入路径上的销毁函数进行排序。
	if b.destroy != nil {
		d := stack.saveDestroyer(b)
		if i := stack.destroyers.Back(); i != nil {
			d.after(i.Value.(*BeanDefinition))
		}
		stack.destroyers.PushBack(b)
	}

	if b.status == Wired {
		return nil
	}

	// 将当前 bean 放入注入栈，以便检测循环依赖。
	stack.pushBack(b)

	if b.status == Wiring {
		if b.f != nil { // 构造函数 bean 出现循环依赖。
			return errors.New("found circle autowire")
		}
		return nil
	}

	b.status = Wiring

	// 对当前 bean 的间接依赖项进行注入。
	for _, s := range b.dependsOn {
		beans, err := c.find(s)
		if err != nil {
			return err
		}
		for _, d := range beans {
			err = c.wireBean(d, stack)
			if err != nil {
				return err
			}
		}
	}

	v, err := c.getBeanValue(b, stack)
	if err != nil {
		return err
	}

	err = c.wireBeanValue(v, stack)
	if err != nil {
		return err
	}

	// 执行 bean 的初始化函数。
	if b.init != nil {
		fnValue := reflect.ValueOf(b.init)
		out := fnValue.Call([]reflect.Value{b.Value()})
		if len(out) > 0 && !out[0].IsNil() {
			return out[0].Interface().(error)
		}
	}

	b.status = Wired
	stack.popBack()
	return nil
}

// getBeanValue 获取 bean 的值，如果是构造函数 bean 则执行其构造函数然后返回执行结果。
func (c *Container) getBeanValue(b *BeanDefinition, stack *wiringStack) (reflect.Value, error) {

	if b.f == nil {
		return b.Value(), nil
	}

	out, err := b.f.Call(newArgContext(c, stack))
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
func (c *Container) wireBeanValue(v reflect.Value, stack *wiringStack) error {

	t := v.Type()

	// 数组 bean 的每个元素单独注入。
	if t.Kind() == reflect.Slice {
		for i := 0; i < v.Len(); i++ {
			err := c.wireBeanValue(v.Index(i), stack)
			if err != nil {
				return err
			}
		}
		return nil
	}

	if t.Kind() == reflect.Map {
		iter := v.MapRange()
		for iter.Next() {
			err := c.wireBeanValue(iter.Value(), stack)
			if err != nil {
				return err
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
	err := c.p.Bind(ev)
	if err != nil {
		return err
	}

	return c.wireStruct(ev, stack)
}

// wireStruct 对结构体进行依赖注入，需要注意的是这里不需要进行属性绑定。
func (c *Container) wireStruct(v reflect.Value, stack *wiringStack) error {

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
			err := c.autowire(fv, tag, stack)
			if err != nil {
				fieldName := typeName + "." + ft.Name
				return fmt.Errorf("%q wired error: %s", fieldName, err.Error())
			}
		}

		// 递归处理结构体字段，指针字段不可以因为可能出现无限循环。
		if ft.Type.Kind() == reflect.Struct {
			err := c.wireStruct(fv, stack)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// autowire 根据 tag 的内容自动判断注入模式，是单例模式，还是收集模式。
func (c *Container) autowire(v reflect.Value, tag string, stack *wiringStack) error {

	// tag 预处理，可以通过属性值进行指定。
	if strings.HasPrefix(tag, "${") {
		s := ""
		err := c.p.Bind(&s, conf.Tag(tag))
		if err != nil {
			return err
		}
		tag = s
	}

	if !collectionMode(tag) {
		return c.getBean(v, parseSingletonTag(tag), stack)
	}
	return c.collectBeans(v, parseCollectionTag(tag), stack)
}

// filterBean 返回 tag 对应的 bean 在数组中的索引，找不到返回 -1。
func filterBean(beans []*BeanDefinition, tag singletonTag, t reflect.Type) (int, error) {

	var found []int
	for i, b := range beans {
		if b.Match(tag.typeName, tag.beanName) {
			found = append(found, i)
		}
	}

	if len(found) > 1 {
		msg := fmt.Sprintf("found %d beans, bean:%q type:%q [", len(found), tag, t)
		for _, i := range found {
			msg += "( " + beans[i].String() + " ), "
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

type byOrder []*BeanDefinition

func (b byOrder) Len() int           { return len(b) }
func (b byOrder) Less(i, j int) bool { return b[i].order < b[j].order }
func (b byOrder) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }

func (c *Container) collectBeans(v reflect.Value, tag collectionTag, stack *wiringStack) error {

	t := v.Type()
	if t.Kind() != reflect.Slice && t.Kind() != reflect.Map {
		return fmt.Errorf("should be slice or map in collection mode")
	}

	et := t.Elem()
	if !util.IsBeanType(et) {
		return errors.New("item in collection mode should be ref type")
	}

	tmp := make([]*BeanDefinition, 0)

	mapType := reflect.MapOf(reflect.TypeOf(""), et)
	cache := c.beansByType[mapType]
	for i := 0; i < len(cache); i++ {
		tmp = append(tmp, cache[i])
	}

	sliceType := reflect.SliceOf(et)
	cache = c.beansByType[sliceType]
	for i := 0; i < len(cache); i++ {
		tmp = append(tmp, cache[i])
	}

	cache = c.beansByType[et]
	for i := 0; i < len(cache); i++ {
		tmp = append(tmp, cache[i])
	}

	var beans []*BeanDefinition

	if len(tag.beanTags) == 0 {
		beans = tmp
	} else {
		for _, item := range tag.beanTags {
			index, err := filterBean(tmp, item, et)
			if err != nil {
				return err
			}
			if index >= 0 {
				beans = append(beans, tmp[index])
			}
		}
	}

	if len(beans) == 0 {
		if tag.nullable {
			return nil
		}
		return fmt.Errorf("no beans collected for %q", tag)
	}

	switch t.Kind() {
	case reflect.Slice:
		ret := reflect.MakeSlice(t, 0, 0)
		sort.Sort(byOrder(beans))
		for _, b := range beans {
			err := c.wireBean(b, stack)
			if err != nil {
				return err
			}
			beanValue := b.Value()
			switch b.Type().Kind() {
			case reflect.Map:
				iter := beanValue.MapRange()
				for iter.Next() {
					ret = reflect.Append(ret, iter.Value())
				}
			case reflect.Slice:
				for i := 0; i < beanValue.Len(); i++ {
					ret = reflect.Append(ret, beanValue.Index(i))
				}
			default:
				ret = reflect.Append(ret, beanValue)
			}
		}
		v.Set(ret)
	case reflect.Map:
		ret := reflect.MakeMap(t)
		for _, b := range beans {
			err := c.wireBean(b, stack)
			if err != nil {
				return err
			}
			beanValue := b.Value()
			switch b.Type().Kind() {
			case reflect.Map:
				iter := beanValue.MapRange()
				for iter.Next() {
					key := b.name + "#" + iter.Key().Interface().(string)
					ret.SetMapIndex(reflect.ValueOf(key), iter.Value())
				}
			case reflect.Slice:
				for i := 0; i < beanValue.Len(); i++ {
					key := fmt.Sprintf("%s#%d", b.name, i)
					ret.SetMapIndex(reflect.ValueOf(key), beanValue.Index(i))
				}
			default:
				ret.SetMapIndex(reflect.ValueOf(b.name), beanValue)
			}
		}
		v.Set(ret)
	}
	return nil
}

type ArgContext struct {
	c     *Container
	stack *wiringStack
}

func newArgContext(c *Container, stack *wiringStack) *ArgContext {
	return &ArgContext{c: c, stack: stack}
}

// Matches 条件成立返回 true，否则返回 false。
func (c *ArgContext) Matches(cond cond.Condition) (bool, error) {
	return cond.Matches(&pandora{c.c})
}

// Bind 根据 tag 的内容进行属性绑定。
func (c *ArgContext) Bind(v reflect.Value, tag string) error {
	return c.c.p.Bind(v, conf.Tag(tag))
}

// Wire 根据 tag 的内容自动判断注入模式，是单例模式，还是收集模式。
func (c *ArgContext) Wire(v reflect.Value, tag string) error {
	return c.c.autowire(v, tag, c.stack)
}
