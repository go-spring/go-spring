/*
 * Copyright 2012-2019 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by bootlicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package gs

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"

	"github.com/go-spring/spring-boost/util"
	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/gs/arg"
	"github.com/go-spring/spring-core/gs/lib"
)

type bootstrap struct {

	// 应用上下文
	c *Container

	// 属性列表解析完成后的回调
	mapOfOnProperty map[string]interface{}

	PropertySources []lib.PropertySource `autowire:""`
}

func validOnProperty(fn interface{}) error {
	t := reflect.TypeOf(fn)
	if t.Kind() != reflect.Func {
		return errors.New("fn should be a func(value_type)")
	}
	if t.NumIn() != 1 || !util.IsValueType(t.In(0)) || t.NumOut() != 0 {
		return errors.New("fn should be a func(value_type)")
	}
	return nil
}

// OnProperty 当 key 对应的属性值准备好后发送一个通知。
func (boot *bootstrap) OnProperty(key string, fn interface{}) {
	err := validOnProperty(fn)
	util.Panic(err).When(err != nil)
	boot.mapOfOnProperty[key] = fn
}

// Property 设置 key 对应的属性值，如果 key 对应的属性值已经存在则 Set 方法会
// 覆盖旧值。Set 方法除了支持 string 类型的属性值，还支持 int、uint、bool 等
// 其他基础数据类型的属性值。特殊情况下，Set 方法也支持 slice 、map 与基础数据
// 类型组合构成的属性值，其处理方式是将组合结构层层展开，可以将组合结构看成一棵树，
// 那么叶子结点的路径就是属性的 key，叶子结点的值就是属性的值。
func (boot *bootstrap) Property(key string, value interface{}) {
	boot.c.Property(key, value)
}

// Object 注册对象形式的 bean ，需要注意的是该方法在注入开始后就不能再调用了。
func (boot *bootstrap) Object(i interface{}) *BeanDefinition {
	return boot.c.register(NewBean(reflect.ValueOf(i)))
}

// Provide 注册构造函数形式的 bean ，需要注意的是该方法在注入开始后就不能再调用了。
func (boot *bootstrap) Provide(ctor interface{}, args ...arg.Arg) *BeanDefinition {
	return boot.c.register(NewBean(ctor, args...))
}

func (boot *bootstrap) start(e *environment) error {

	boot.c.Object(boot)

	if err := boot.loadBootstrap(e); err != nil {
		return err
	}

	// 保存从环境变量和命令行解析的属性
	for _, k := range e.p.Keys() {
		boot.c.p.Set(k, e.p.Get(k))
	}

	for key, f := range boot.mapOfOnProperty {
		t := reflect.TypeOf(f)
		in := reflect.New(t.In(0)).Elem()
		err := boot.c.p.Bind(in, conf.Key(key))
		if err != nil {
			return err
		}
		reflect.ValueOf(f).Call([]reflect.Value{in})
	}

	return boot.c.Refresh()
}

func (boot *bootstrap) loadBootstrap(e *environment) error {
	if err := boot.loadConfigFile(e, "bootstrap"); err != nil {
		return err
	}
	if e.activeProfile == "" {
		return nil
	}
	return boot.loadConfigFile(e, "bootstrap-"+e.activeProfile)
}

func (boot *bootstrap) loadConfigFile(e *environment, filename string) error {
	for _, loc := range e.configLocations {
		for _, ext := range e.configExtensions {
			err := boot.c.Load(filepath.Join(loc, filename+ext))
			if err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

func (boot *bootstrap) sourceMap(e *environment) (map[string][]*conf.Properties, error) {
	sourceMap := make(map[string][]*conf.Properties)
	for _, ps := range boot.PropertySources {
		m, err := ps.Load(e)
		if err != nil {
			return nil, err
		}
		for k, p := range m {
			sourceMap[k] = append(sourceMap[k], p)
		}
	}
	return sourceMap, nil
}
