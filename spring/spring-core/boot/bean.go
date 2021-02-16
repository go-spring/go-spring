package boot

import (
	"github.com/go-spring/spring-core/core"
)

var beans = make([]*core.BeanDefinition, 0)

func addBean(bd *core.BeanDefinition) *core.BeanDefinition {
	beans = append(beans, bd)
	return bd
}

func ObjBean(i interface{}) *core.BeanDefinition {
	return addBean(core.ObjBean(i))
}

func CtorBean(fn interface{}, args ...core.Arg) *core.BeanDefinition {
	return addBean(core.CtorBean(fn, args...))
}

var configers = make([]*core.Configer, 0)

func Config(fn interface{}, args ...core.Arg) *core.Configer {
	c := core.Config(fn, args...)
	configers = append(configers, c)
	return c
}
