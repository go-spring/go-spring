package app

import (
	"github.com/go-spring/spring-core/core"
)

// ModuleContext
type ModuleContext interface {

	// ObjBean
	ObjBean(i interface{}) *core.BeanDefinition

	// CtorBean
	CtorBean(fn interface{}, args ...core.Arg) *core.BeanDefinition

	// Config
	Config(fn interface{}, args ...core.Arg) *core.Configer
}

type ModuleFunc func(ctx ModuleContext)

var modules = make([]ModuleFunc, 0)

func Module(f ModuleFunc) { modules = append(modules, f) }
