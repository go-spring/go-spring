package app

import (
	"github.com/go-spring/spring-core/core"
)

type builder struct {
	app *application
}

func New() *builder {
	return &builder{app: newApp()}
}

func (bu *builder) ApplicationContext() core.ApplicationContext {
	return bu.app.ApplicationContext
}

func (bu *builder) BannerMode(mode BannerMode) *builder {
	bu.app.bannerMode = mode
	return bu
}

func (bu *builder) ExpectSysProperties(pattern ...string) *builder {
	bu.app.expectSysProperties = pattern
	return bu
}

func (bu *builder) AfterPrepare(fn AfterPrepareFunc) *builder {
	bu.app.listOfAfterPrepare = append(bu.app.listOfAfterPrepare, fn)
	return bu
}

func (bu *builder) Run(cfgLocation ...string) {
	bu.app.Run(cfgLocation...)
}

// ShutDown 关闭执行器
func (bu *builder) ShutDown() {
	bu.app.ShutDown()
}

// Bean 注册 bean.BeanDefinition 对象。
func (bu *builder) Bean(bd *core.BeanDefinition) *builder {
	bu.app.RegisterBean(bd)
	return bu
}
