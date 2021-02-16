package boot

import (
	"context"
	"fmt"

	"github.com/go-spring/spring-core/core"
)

var listOfAppCtx = make([]core.ApplicationContext, 0)

func addAppCtx(appCtx core.ApplicationContext) {
	for _, c := range listOfAppCtx {
		if c == appCtx {
			return
		}
	}
	listOfAppCtx = append(listOfAppCtx, appCtx)
}

func delAppCtx(appCtx core.ApplicationContext) {
	index := -1
	for i, c := range listOfAppCtx {
		if c == appCtx {
			index = i
			break
		}
	}
	if index >= 0 {
		listOfAppCtx = append(listOfAppCtx[:index], listOfAppCtx[index+1:]...)
	}
}

func checkAppCtxCount() {
	if n := len(listOfAppCtx); n != 1 {
		panic(fmt.Errorf("found %d ApplicationContext", n))
	}
}

func ApplicationContext() core.ApplicationContext {
	checkAppCtxCount()
	return listOfAppCtx[0]
}

// GetProfile 返回运行环境
func GetProfile() string {
	return ApplicationContext().GetProfile()
}

func GetProperty(key string) interface{} {
	return ApplicationContext().GetProperty(key)
}

func WireBean(i interface{}) {
	ApplicationContext().WireBean(i)
}

// Beans 获取所有 Bean 的定义，不能保证解析和注入，请谨慎使用该函数!
func Beans() []*core.BeanInstance {
	return ApplicationContext().Beans()
}

func GetBean(i interface{}, selector ...core.BeanSelector) bool {
	return ApplicationContext().GetBean(i, selector...)
}

func FindBean(selector core.BeanSelector) (*core.BeanInstance, bool) {
	return ApplicationContext().FindBean(selector)
}

func CollectBeans(i interface{}, selectors ...core.BeanSelector) bool {
	return ApplicationContext().CollectBeans(i, selectors...)
}

func Invoke(fn interface{}, args ...core.Arg) error {
	return ApplicationContext().Invoke(fn, args...)
}

type GoFuncWithContext func(context.Context)

func Go(fn GoFuncWithContext) {
	appCtx := ApplicationContext()
	appCtx.Go(func() { fn(appCtx.Context()) })
}
