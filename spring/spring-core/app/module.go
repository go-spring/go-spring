package app

import (
	"github.com/go-spring/spring-core/arg"
	"github.com/go-spring/spring-core/core"
)

// ModuleContext
type ModuleContext interface {

	// ObjBean
	ObjBean(i interface{}) *core.BeanDefinition

	// CtorBean
	CtorBean(fn interface{}, args ...arg.Arg) *core.BeanDefinition

	// Config
	Config(fn interface{}, args ...arg.Arg) *core.Configer
}

type ModuleFunc func(ctx ModuleContext)

var modules = make([]ModuleFunc, 0)

func Module(f ModuleFunc) { modules = append(modules, f) }
