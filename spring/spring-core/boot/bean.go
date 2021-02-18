package boot

import (
	"github.com/go-spring/spring-core/arg"
	"github.com/go-spring/spring-core/core"
)

func ObjBean(i interface{}) *core.BeanDefinition {
	bd := core.ObjBean(i)
	gApp.Bean(bd)
	return bd
}

func CtorBean(fn interface{}, args ...arg.Arg) *core.BeanDefinition {
	bd := core.CtorBean(fn, args...)
	gApp.Bean(bd)
	return bd
}

func Config(fn interface{}, args ...arg.Arg) *core.Configer {
	c := core.Config(fn, args...)
	gApp.Configer(c)
	return c
}
