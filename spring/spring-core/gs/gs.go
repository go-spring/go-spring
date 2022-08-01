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

// Package gs 实现了 go-spring 的核心骨架，包含 IoC 容器、基于 IoC 容器的 App
// 以及全局 App 对象封装三个部分，可以应用于多种使用场景。
package gs

import (
	"bytes"
	"container/list"
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/gs/arg"
	"github.com/go-spring/spring-core/gs/cond"
	"github.com/go-spring/spring-core/gs/gsutil"
)

type refreshState int

const (
	Unrefreshed = refreshState(iota) // 未刷新
	RefreshInit                      // 准备刷新
	Refreshing                       // 正在刷新
	Refreshed                        // 已刷新
)

var (
	loggerType  = reflect.TypeOf((*log.Logger)(nil))
	contextType = reflect.TypeOf((*Context)(nil)).Elem()
)

type Container interface {
	Context() context.Context
	Property(key string, value interface{})
	Object(i interface{}) *BeanDefinition
	Provide(ctor interface{}, args ...arg.Arg) *BeanDefinition
	Refresh() error
	Go(fn func(ctx context.Context))
	Close()
}

// Context 提供了一些在 IoC 容器启动后基于反射获取和使用 property 与 bean 的接
// 口。因为很多人会担心在运行时大量使用反射会降低程序性能，所以命名为 Context，取
// 其诱人但危险的含义。事实上，这些在 IoC 容器启动后使用属性绑定和依赖注入的方案，
// 都可以转换为启动阶段的方案以提高程序的性能。
// 另一方面，为了统一 Container 和 App 两种启动方式下这些方法的使用方式，需要提取
// 出一个可共用的接口来，也就是说，无论程序是 Container 方式启动还是 App 方式启动，
// 都可以在需要使用这些方法的地方注入一个 Context 对象而不是 Container 对象或者
// App 对象，从而实现使用方式的统一。
type Context interface {
	Context() context.Context
	Keys() []string
	Has(key string) bool
	Prop(key string, opts ...conf.GetOption) string
	Bind(i interface{}, opts ...conf.BindOption) error
	Get(i interface{}, selectors ...gsutil.BeanSelector) error
	Wire(objOrCtor interface{}, ctorArgs ...arg.Arg) (interface{}, error)
	Invoke(fn interface{}, args ...arg.Arg) ([]interface{}, error)
	Go(fn func(ctx context.Context))
}

// ContextAware injects the Context into a struct as the field GSContext.
type ContextAware struct {
	GSContext Context `autowire:""`
}

type tempContainer struct {
	p               *conf.Properties
	beans           []*BeanDefinition
	beansByName     map[string][]*BeanDefinition
	beansByType     map[reflect.Type][]*BeanDefinition
	mapOfOnProperty map[string]interface{}
}

// container 是 go-spring 框架的基石，实现了 Martin Fowler 在 << Inversion
// of Control Containers and the Dependency Injection pattern >> 一文中
// 提及的依赖注入的概念。但原文的依赖注入仅仅是指对象之间的依赖关系处理，而有些 IoC
// 容器在实现时比如 Spring 还引入了对属性 property 的处理。通常大家会用依赖注入统
// 述上面两种概念，但实际上使用属性绑定来描述对 property 的处理会更加合适，因此
// go-spring 严格区分了这两种概念，在描述对 bean 的处理时要么单独使用依赖注入或属
// 性绑定，要么同时使用依赖注入和属性绑定。
type container struct {
	*tempContainer
	logger                  *log.Logger
	ctx                     context.Context
	cancel                  context.CancelFunc
	destroyers              []func()
	state                   refreshState
	wg                      sync.WaitGroup
	ContextAware            bool
	AllowCircularReferences bool `value:"${spring.main.allow-circular-references:=false}"`
}

// New 创建 IoC 容器。
func New() Container {
	ctx, cancel := context.WithCancel(context.Background())
	return &container{
		ctx:    ctx,
		cancel: cancel,
		tempContainer: &tempContainer{
			p:               conf.New(),
			beansByName:     make(map[string][]*BeanDefinition),
			beansByType:     make(map[reflect.Type][]*BeanDefinition),
			mapOfOnProperty: make(map[string]interface{}),
		},
	}
}

// Context 返回 IoC 容器的 ctx 对象。
func (c *container) Context() context.Context {
	return c.ctx
}

func validOnProperty(fn interface{}) error {
	t := reflect.TypeOf(fn)
	if t.Kind() != reflect.Func {
		return errors.New("fn should be a func(value_type)")
	}
	if t.NumIn() != 1 || !conf.IsValueType(t.In(0)) || t.NumOut() != 0 {
		return errors.New("fn should be a func(value_type)")
	}
	return nil
}

// OnProperty 当 key 对应的属性值准备好后发送一个通知。
func (c *container) OnProperty(key string, fn interface{}) {
	err := validOnProperty(fn)
	util.Panic(err).When(err != nil)
	c.mapOfOnProperty[key] = fn
}

// Property 设置 key 对应的属性值，如果 key 对应的属性值已经存在则 Set 方法会
// 覆盖旧值。Set 方法除了支持 string 类型的属性值，还支持 int、uint、bool 等
// 其他基础数据类型的属性值。特殊情况下，Set 方法也支持 slice 、map 与基础数据
// 类型组合构成的属性值，其处理方式是将组合结构层层展开，可以将组合结构看成一棵树，
// 那么叶子结点的路径就是属性的 key，叶子结点的值就是属性的值。
func (c *container) Property(key string, value interface{}) {
	c.p.Set(key, value)
}

func (c *container) Accept(b *BeanDefinition) *BeanDefinition {
	if c.state >= Refreshing {
		panic(errors.New("should call before Refresh"))
	}
	c.beans = append(c.beans, b)
	return b
}

// Object 注册对象形式的 bean ，需要注意的是该方法在注入开始后就不能再调用了。
func (c *container) Object(i interface{}) *BeanDefinition {
	return c.Accept(NewBean(reflect.ValueOf(i)))
}

// Provide 注册构造函数形式的 bean ，需要注意的是该方法在注入开始后就不能再调用了。
func (c *container) Provide(ctor interface{}, args ...arg.Arg) *BeanDefinition {
	return c.Accept(NewBean(ctor, args...))
}

// destroyer 保存具有销毁函数的 bean 以及销毁函数的调用顺序。
type destroyer struct {
	current *BeanDefinition
	earlier []*BeanDefinition
}

func (d *destroyer) foundEarlier(b *BeanDefinition) bool {
	for _, c := range d.earlier {
		if c == b {
			return true
		}
	}
	return false
}

// after 添加一个需要在该 bean 的销毁函数执行之前调用销毁函数的 bean 。
func (d *destroyer) after(b *BeanDefinition) {
	if d.foundEarlier(b) {
		return
	}
	d.earlier = append(d.earlier, b)
}

// getBeforeDestroyers 获取排在 i 前面的 destroyer，用于 sort.Triple 排序。
func getBeforeDestroyers(destroyers *list.List, i interface{}) *list.List {
	d := i.(*destroyer)
	result := list.New()
	for e := destroyers.Front(); e != nil; e = e.Next() {
		c := e.Value.(*destroyer)
		if d.foundEarlier(c.current) {
			result.PushBack(c)
		}
	}
	return result
}

type lazyField struct {
	v    reflect.Value
	path string
	tag  string
}

// wiringStack 记录 bean 的注入路径。
type wiringStack struct {
	logger       *log.Logger
	destroyers   *list.List
	destroyerMap map[string]*destroyer
	beans        []*BeanDefinition
	lazyFields   []lazyField
}

func newWiringStack(logger *log.Logger) *wiringStack {
	return &wiringStack{
		logger:       logger,
		destroyers:   list.New(),
		destroyerMap: make(map[string]*destroyer),
	}
}

// pushBack 添加一个即将注入的 bean 。
func (s *wiringStack) pushBack(b *BeanDefinition) {
	s.logger.Tracef("push %s %s", b, getStatusString(b.status))
	s.beans = append(s.beans, b)
}

// popBack 删除一个已经注入的 bean 。
func (s *wiringStack) popBack() {
	n := len(s.beans)
	b := s.beans[n-1]
	s.beans = s.beans[:n-1]
	s.logger.Tracef("pop %s %s", b, getStatusString(b.status))
}

// path 返回 bean 的注入路径。
func (s *wiringStack) path() (path string) {
	for _, b := range s.beans {
		path += fmt.Sprintf("=> %s ↩\n", b)
	}
	return path[:len(path)-1]
}

// saveDestroyer 记录具有销毁函数的 bean ，因为可能有多个依赖，因此需要排重处理。
func (s *wiringStack) saveDestroyer(b *BeanDefinition) *destroyer {
	d, ok := s.destroyerMap[b.ID()]
	if !ok {
		d = &destroyer{current: b}
		s.destroyerMap[b.ID()] = d
	}
	return d
}

// sortDestroyers 对具有销毁函数的 bean 按照销毁函数的依赖顺序进行排序。
func (s *wiringStack) sortDestroyers() []func() {

	destroy := func(v reflect.Value, fn interface{}) func() {
		return func() {
			if fn == nil {
				v.Interface().(BeanDestroy).OnDestroy()
			} else {
				fnValue := reflect.ValueOf(fn)
				out := fnValue.Call([]reflect.Value{v})
				if len(out) > 0 && !out[0].IsNil() {
					s.logger.Error(out[0].Interface().(error))
				}
			}
		}
	}

	destroyers := list.New()
	for _, d := range s.destroyerMap {
		destroyers.PushBack(d)
	}
	destroyers = util.TripleSort(destroyers, getBeforeDestroyers)

	var ret []func()
	for e := destroyers.Front(); e != nil; e = e.Next() {
		d := e.Value.(*destroyer).current
		ret = append(ret, destroy(d.Value(), d.destroy))
	}
	return ret
}

func (c *container) clear() {
	c.tempContainer = nil
}

// Refresh 刷新容器的内容，对 bean 进行有效性判断以及完成属性绑定和依赖注入。
func (c *container) Refresh() error {
	return c.refresh(true)
}

func (c *container) refresh(autoClear bool) (err error) {

	if c.state != Unrefreshed {
		return errors.New("container already refreshed")
	}
	c.state = RefreshInit

	start := time.Now()
	c.Object(c).Export((*Context)(nil))
	c.logger = log.GetLogger(util.TypeName(c))

	for key, f := range c.mapOfOnProperty {
		t := reflect.TypeOf(f)
		in := reflect.New(t.In(0)).Elem()
		if err = c.p.Bind(in, conf.Key(key)); err != nil {
			return err
		}
		reflect.ValueOf(f).Call([]reflect.Value{in})
	}

	c.state = Refreshing

	for _, b := range c.beans {
		c.registerBean(b)
	}

	for _, b := range c.beans {
		if err = c.resolveBean(b); err != nil {
			return err
		}
	}

	beansById := make(map[string]*BeanDefinition)
	{
		for _, b := range c.beans {
			if b.status == Deleted {
				continue
			}
			if b.status != Resolved {
				return fmt.Errorf("unexpected status %d", b.status)
			}
			beanID := b.ID()
			if d, ok := beansById[beanID]; ok {
				return fmt.Errorf("found duplicate beans [%s] [%s]", b, d)
			}
			beansById[beanID] = b
		}
	}

	stack := newWiringStack(c.logger)

	defer func() {
		if err != nil || len(stack.beans) > 0 {
			err = fmt.Errorf("%s ↩\n%s", err, stack.path())
			c.logger.Error(err)
		}
	}()

	// 按照 bean id 升序注入，保证注入过程始终一致。
	{
		var keys []string
		for s := range beansById {
			keys = append(keys, s)
		}
		sort.Strings(keys)
		for _, s := range keys {
			b := beansById[s]
			if err = c.wireBean(b, stack); err != nil {
				return err
			}
		}
	}

	if c.AllowCircularReferences {
		// 处理被标记为延迟注入的那些 bean 字段
		for _, f := range stack.lazyFields {
			tag := strings.TrimSuffix(f.tag, ",lazy")
			if err := c.wireByTag(f.v, tag, stack); err != nil {
				return fmt.Errorf("%q wired error: %s", f.path, err.Error())
			}
		}
	} else if len(stack.lazyFields) > 0 {
		return errors.New("remove the dependency cycle between beans")
	}

	c.destroyers = stack.sortDestroyers()
	c.state = Refreshed

	cost := time.Now().Sub(start)
	c.logger.Infof("refresh %d beans cost %v", len(beansById), cost)

	if autoClear && !c.ContextAware {
		c.clear()
	}

	c.logger.Info("container refreshed successfully")
	return nil
}

func (c *container) registerBean(b *BeanDefinition) {
	c.logger.Debugf("register %s name:%q type:%q %s", b.getClass(), b.BeanName(), b.Type(), b.FileLine())
	c.beansByName[b.name] = append(c.beansByName[b.name], b)
	c.beansByType[b.Type()] = append(c.beansByType[b.Type()], b)
	for _, t := range b.exports {
		c.logger.Debugf("register %s name:%q type:%q %s", b.getClass(), b.BeanName(), t, b.FileLine())
		c.beansByType[t] = append(c.beansByType[t], b)
	}
}

// resolveBean 判断 bean 的有效性，如果 bean 是无效的则被标记为已删除。
func (c *container) resolveBean(b *BeanDefinition) error {

	if b.status >= Resolving {
		return nil
	}

	b.status = Resolving

	// method bean 先确定 parent bean 是否存在
	if b.method {
		selector, ok := b.f.Arg(0)
		if !ok || selector == "" {
			selector, _ = b.f.In(0)
		}
		parents, err := c.findBean(selector)
		if err != nil {
			return err
		}
		n := len(parents)
		if n > 1 {
			msg := fmt.Sprintf("found %d parent beans, bean:%q type:%q [", n, selector, b.t.In(0))
			for _, b := range parents {
				msg += "( " + b.String() + " ), "
			}
			msg = msg[:len(msg)-2] + "]"
			return errors.New(msg)
		} else if n == 0 {
			b.status = Deleted
			return nil
		}
	}

	if b.cond != nil {
		if ok, err := b.cond.Matches(c); err != nil {
			return err
		} else if !ok {
			b.status = Deleted
			return nil
		}
	}

	b.status = Resolved
	return nil
}

// wireTag 注入语法的 tag 分解式，字符串形式的完整格式为 TypeName:BeanName? 。
// 注入语法的字符串表示形式分为三个部分，TypeName 是原始类型的全限定名，BeanName
// 是 bean 注册时设置的名称，? 表示注入结果允许为空。
type wireTag struct {
	typeName string
	beanName string
	nullable bool
}

func parseWireTag(str string) (tag wireTag) {

	if str == "" {
		return
	}

	if n := len(str) - 1; str[n] == '?' {
		tag.nullable = true
		str = str[:n]
	}

	i := strings.Index(str, ":")
	if i < 0 {
		tag.beanName = str
		return
	}

	tag.typeName = str[:i]
	tag.beanName = str[i+1:]
	return
}

func (tag wireTag) String() string {
	b := bytes.NewBuffer(nil)
	if tag.typeName != "" {
		b.WriteString(tag.typeName)
		b.WriteString(":")
	}
	b.WriteString(tag.beanName)
	if tag.nullable {
		b.WriteString("?")
	}
	return b.String()
}

func toWireTag(selector gsutil.BeanSelector) wireTag {
	switch s := selector.(type) {
	case string:
		return parseWireTag(s)
	case BeanDefinition:
		return parseWireTag(s.ID())
	case *BeanDefinition:
		return parseWireTag(s.ID())
	default:
		return parseWireTag(gsutil.TypeName(s) + ":")
	}
}

func toWireString(tags []wireTag) string {
	var buf bytes.Buffer
	for i, tag := range tags {
		buf.WriteString(tag.String())
		if i < len(tags)-1 {
			buf.WriteByte(',')
		}
	}
	return buf.String()
}

// findBean 查找符合条件的 bean 对象，注意该函数只能保证返回的 bean 是有效的，
// 即未被标记为删除的，而不能保证已经完成属性绑定和依赖注入。
func (c *container) findBean(selector gsutil.BeanSelector) ([]*BeanDefinition, error) {

	finder := func(fn func(*BeanDefinition) bool) ([]*BeanDefinition, error) {
		var result []*BeanDefinition
		for _, b := range c.beans {
			if b.status == Resolving || b.status == Deleted || !fn(b) {
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

	var t reflect.Type
	switch st := selector.(type) {
	case string, BeanDefinition, *BeanDefinition:
		tag := toWireTag(selector)
		return finder(func(b *BeanDefinition) bool {
			return b.Match(tag.typeName, tag.beanName)
		})
	case reflect.Type:
		t = st
	default:
		t = reflect.TypeOf(st)
	}

	if t.Kind() == reflect.Ptr {
		if e := t.Elem(); e.Kind() == reflect.Interface {
			t = e // 指 (*error)(nil) 形式的 bean 选择器
		}
	}

	return finder(func(b *BeanDefinition) bool {
		if b.Type() == t {
			return true
		}
		for _, typ := range b.exports {
			if typ == t {
				return true
			}
		}
		return false
	})
}

// wireBean 对 bean 进行属性绑定和依赖注入，同时追踪其注入路径。如果 bean 有初始
// 化函数，则在注入完成之后执行其初始化函数。如果 bean 依赖了其他 bean，则首先尝试
// 实例化被依赖的 bean 然后对它们进行注入。
func (c *container) wireBean(b *BeanDefinition, stack *wiringStack) error {

	if b.status == Deleted {
		return fmt.Errorf("bean:%q have been deleted", b.ID())
	}

	// 运行时 Get 或者 Wire 会出现下面这种情况。
	if c.state == Refreshed && b.status == Wired {
		return nil
	}

	haveDestroy := false

	defer func() {
		if haveDestroy {
			stack.destroyers.Remove(stack.destroyers.Back())
		}
	}()

	// 记录注入路径上的销毁函数及其执行的先后顺序。
	if _, ok := b.Interface().(BeanDestroy); ok || b.destroy != nil {
		haveDestroy = true
		d := stack.saveDestroyer(b)
		if i := stack.destroyers.Back(); i != nil {
			d.after(i.Value.(*BeanDefinition))
		}
		stack.destroyers.PushBack(b)
	}

	stack.pushBack(b)

	if b.status == Creating && b.f != nil {
		prev := stack.beans[len(stack.beans)-2]
		if prev.status == Creating {
			return errors.New("found circle autowire")
		}
	}

	if b.status >= Creating {
		stack.popBack()
		return nil
	}

	b.status = Creating

	// 对当前 bean 的间接依赖项进行注入。
	for _, s := range b.depends {
		beans, err := c.findBean(s)
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

	b.status = Created

	t := v.Type()
	for _, typ := range b.exports {
		if !t.Implements(typ) {
			return fmt.Errorf("%s doesn't implement interface %s", b, typ)
		}
	}

	err = c.wireBeanValue(v, t, stack)
	if err != nil {
		return err
	}

	if b.init != nil {
		fnValue := reflect.ValueOf(b.init)
		out := fnValue.Call([]reflect.Value{b.Value()})
		if len(out) > 0 && !out[0].IsNil() {
			return out[0].Interface().(error)
		}
	}

	if f, ok := b.Interface().(BeanInit); ok {
		if err = f.OnInit(c); err != nil {
			return err
		}
	}

	b.status = Wired
	stack.popBack()
	return nil
}

type argContext struct {
	c     *container
	stack *wiringStack
}

func (a *argContext) Matches(c cond.Condition) (bool, error) {
	return c.Matches(a.c)
}

func (a *argContext) Bind(v reflect.Value, tag string) error {
	return a.c.p.Bind(v, conf.Tag(tag))
}

func (a *argContext) Wire(v reflect.Value, tag string) error {
	return a.c.wireByTag(v, tag, a.stack)
}

// getBeanValue 获取 bean 的值，如果是构造函数 bean 则执行其构造函数然后返回执行结果。
func (c *container) getBeanValue(b *BeanDefinition, stack *wiringStack) (reflect.Value, error) {

	if b.f == nil {
		return b.Value(), nil
	}

	out, err := b.f.Call(&argContext{c: c, stack: stack})
	if err != nil {
		return reflect.Value{}, err /* fmt.Errorf("%s:%s return error: %v", b.getClass(), b.ID(), err) */
	}

	// 构造函数的返回值为值类型时 b.Type() 返回其指针类型。
	if val := out[0]; gsutil.IsBeanType(val.Type()) {
		// 如果实现接口的是值类型，那么需要转换成指针类型然后再赋值给接口。
		if !val.IsNil() && val.Kind() == reflect.Interface && conf.IsValueType(val.Elem().Type()) {
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
		return reflect.Value{}, fmt.Errorf("%s:%q return nil", b.getClass(), b.FileLine())
	}

	v := b.Value()
	// 结果以接口类型返回时需要将原始值取出来才能进行注入。
	if b.Type().Kind() == reflect.Interface {
		v = v.Elem()
	}
	return v, nil
}

// wireBeanValue 对 v 进行属性绑定和依赖注入，v 在传入时应该是一个已经初始化的值。
func (c *container) wireBeanValue(v reflect.Value, t reflect.Type, stack *wiringStack) error {

	if v.Kind() == reflect.Ptr {
		v = v.Elem()
		t = t.Elem()
	}

	// 如整数指针类型的 bean 是无需注入的。
	if v.Kind() != reflect.Struct {
		return nil
	}

	typeName := t.Name()
	if typeName == "" { // 简单类型没有名字
		typeName = t.String()
	}

	param := conf.BindParam{Type: t, Path: typeName}
	return c.wireStruct(v, param, stack)
}

// wireStruct 对结构体进行依赖注入，需要注意的是这里不需要进行属性绑定。
func (c *container) wireStruct(v reflect.Value, opt conf.BindParam, stack *wiringStack) error {

	for i := 0; i < opt.Type.NumField(); i++ {
		ft := opt.Type.Field(i)
		fv := v.Field(i)

		if !fv.CanInterface() {
			fv = util.PatchValue(fv)
			if !fv.CanInterface() {
				continue
			}
		}

		fieldPath := opt.Path + "." + ft.Name

		tag, ok := ft.Tag.Lookup("logger")
		if ok {
			if ft.Type != loggerType {
				return fmt.Errorf("field expects type *log.Logger")
			}
			l := log.GetLogger(util.TypeName(v))
			fv.Set(reflect.ValueOf(l))
			continue
		}

		// 支持 autowire 和 inject 两个标签。
		tag, ok = ft.Tag.Lookup("autowire")
		if !ok {
			tag, ok = ft.Tag.Lookup("inject")
		}
		if ok {
			if strings.HasSuffix(tag, ",lazy") {
				f := lazyField{v: fv, path: fieldPath, tag: tag}
				stack.lazyFields = append(stack.lazyFields, f)
			} else {
				if ft.Type == contextType {
					c.ContextAware = true
				}
				if err := c.wireByTag(fv, tag, stack); err != nil {
					return fmt.Errorf("%q wired error: %w", fieldPath, err)
				}
			}
			continue
		}

		subParam := conf.BindParam{
			Type: ft.Type,
			Key:  opt.Key,
			Path: fieldPath,
		}

		if tag, ok = ft.Tag.Lookup("value"); ok {
			if err := subParam.BindTag(tag); err != nil {
				return err
			}
			if ft.Anonymous {
				if err := c.wireStruct(fv, subParam, stack); err != nil {
					return err
				}
			} else {
				if err := conf.BindValue(c.p, fv, subParam); err != nil {
					return err
				}
			}
			continue
		}

		if ft.Anonymous && ft.Type.Kind() == reflect.Struct {
			if err := c.wireStruct(fv, subParam, stack); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *container) wireByTag(v reflect.Value, tag string, stack *wiringStack) error {

	// tag 预处理，可能通过属性值进行指定。
	if strings.HasPrefix(tag, "${") {
		s, err := c.p.Resolve(tag)
		if err != nil {
			return err
		}
		tag = s
	}

	if tag == "" {
		return c.autowire(v, nil, stack)
	}

	var tags []wireTag
	for _, s := range strings.Split(tag, ",") {
		tags = append(tags, toWireTag(s))
	}
	return c.autowire(v, tags, stack)
}

func (c *container) autowire(v reflect.Value, tags []wireTag, stack *wiringStack) error {
	switch v.Kind() {
	case reflect.Map, reflect.Slice, reflect.Array:
		return c.collectBeans(v, tags, stack)
	default:
		var tag wireTag
		if len(tags) > 0 {
			tag = tags[0]
		}
		return c.getBean(v, tag, stack)
	}
}

// getBean 获取 tag 对应的 bean 然后赋值给 v，因此 v 应该是一个未初始化的值。
func (c *container) getBean(v reflect.Value, tag wireTag, stack *wiringStack) error {

	if !v.IsValid() {
		return fmt.Errorf("receiver must be ref type, bean:%q", tag)
	}

	t := v.Type()
	if !gsutil.IsBeanReceiver(t) {
		return fmt.Errorf("%s is not valid receiver type", t.String())
	}

	var foundBeans []*BeanDefinition

	for _, b := range c.beansByType[t] {
		if b.status == Deleted {
			continue
		}
		if !b.Match(tag.typeName, tag.beanName) {
			continue
		}
		foundBeans = append(foundBeans, b)
	}

	// 指定 bean 名称时通过名称获取，防止未通过 Export 方法导出接口。
	if t.Kind() == reflect.Interface && tag.beanName != "" {
		for _, b := range c.beansByName[tag.beanName] {
			if b.status == Deleted {
				continue
			}
			if !b.Type().AssignableTo(t) {
				continue
			}
			if !b.Match(tag.typeName, tag.beanName) {
				continue
			}

			found := false // 对结果排重
			for _, r := range foundBeans {
				if r == b {
					found = true
					break
				}
			}
			if !found {
				foundBeans = append(foundBeans, b)
				c.logger.Warnf("you should call Export() on %s", b)
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

// filterBean 返回 tag 对应的 bean 在数组中的索引，找不到返回 -1。
func filterBean(beans []*BeanDefinition, tag wireTag, t reflect.Type) (int, error) {

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

func (c *container) collectBeans(v reflect.Value, tags []wireTag, stack *wiringStack) error {

	t := v.Type()
	if t.Kind() != reflect.Slice && t.Kind() != reflect.Map {
		return fmt.Errorf("should be slice or map in collection mode")
	}

	et := t.Elem()
	if !gsutil.IsBeanReceiver(et) {
		return fmt.Errorf("%s is not valid receiver type", t.String())
	}

	beans := c.beansByType[et]
	{
		var arr []*BeanDefinition
		for _, b := range beans {
			if b.status == Deleted {
				continue
			}
			arr = append(arr, b)
		}
		beans = arr
	}
	if len(tags) > 0 {

		var (
			anyBeans  []*BeanDefinition
			afterAny  []*BeanDefinition
			beforeAny []*BeanDefinition
		)

		foundAny := false
		for _, item := range tags {

			// 是否遇到了"无序"标记
			if item.beanName == "*" {
				if foundAny {
					return fmt.Errorf("more than one * in collection %q", tags)
				}
				foundAny = true
				continue
			}

			index, err := filterBean(beans, item, et)
			if err != nil {
				return err
			}
			if index < 0 {
				continue
			}

			if foundAny {
				afterAny = append(afterAny, beans[index])
			} else {
				beforeAny = append(beforeAny, beans[index])
			}

			tmpBeans := append([]*BeanDefinition{}, beans[:index]...)
			beans = append(tmpBeans, beans[index+1:]...)
		}

		if foundAny {
			anyBeans = append(anyBeans, beans...)
		}

		n := len(beforeAny) + len(anyBeans) + len(afterAny)
		arr := make([]*BeanDefinition, 0, n)
		arr = append(arr, beforeAny...)
		arr = append(arr, anyBeans...)
		arr = append(arr, afterAny...)
		beans = arr
	}

	if len(beans) == 0 {
		if len(tags) == 0 {
			return fmt.Errorf("no beans collected for %q", toWireString(tags))
		}
		for _, tag := range tags {
			if !tag.nullable {
				return fmt.Errorf("no beans collected for %q", toWireString(tags))
			}
		}
		return nil
	}

	for _, b := range beans {
		if err := c.wireBean(b, stack); err != nil {
			return err
		}
	}

	var ret reflect.Value
	switch t.Kind() {
	case reflect.Slice:
		sort.Sort(byOrder(beans))
		ret = reflect.MakeSlice(t, 0, 0)
		for _, b := range beans {
			ret = reflect.Append(ret, b.Value())
		}
	case reflect.Map:
		ret = reflect.MakeMap(t)
		for _, b := range beans {
			ret.SetMapIndex(reflect.ValueOf(b.name), b.Value())
		}
	}
	v.Set(ret)
	return nil
}

// Close 关闭容器，此方法必须在 Refresh 之后调用。该方法会触发 ctx 的 Done 信
// 号，然后等待所有 goroutine 结束，最后按照被依赖先销毁的原则执行所有的销毁函数。
func (c *container) Close() {

	c.cancel()
	c.wg.Wait()

	c.logger.Info("goroutines exited")

	for _, f := range c.destroyers {
		f()
	}

	c.logger.Info("container closed")
}

// Go 创建安全可等待的 goroutine，fn 要求的 ctx 对象由 IoC 容器提供，当 IoC 容
// 器关闭时 ctx会 发出 Done 信号， fn 在接收到此信号后应当立即退出。
func (c *container) Go(fn func(ctx context.Context)) {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				c.logger.Panic(r)
			}
		}()
		fn(c.ctx)
	}()
}
