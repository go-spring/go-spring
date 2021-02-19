package bean

import (
	"errors"
	"reflect"

	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/util"
)

// ValidBean 返回是否是合法的 Bean 及其类型
func ValidBean(v reflect.Value) (reflect.Type, bool) {
	if v.IsValid() {
		if beanType := v.Type(); util.IsRefType(beanType.Kind()) {
			return beanType, true
		}
	}
	return nil, false
}

// Selector Bean 选择器，可以是 BeanId 字符串，可以是 reflect.Type
// 对象或者形如 (*error)(nil) 的对象指针，还可以是 *BeanDefinition 对象。
type Selector interface{}

// TypeOrPtr 可以是 reflect.Type 对象或者形如 (*error)(nil) 的对象指针。
type TypeOrPtr interface{}

// TypeName 返回原始类型的全限定名，Go 语言允许不同的路径下存在相同的包，因此有全限定名
// 的需求，形如 "github.com/go-spring/spring-core/SpringCore.BeanDefinition"。
func TypeName(typOrPtr TypeOrPtr) string {

	if typOrPtr == nil {
		panic(errors.New("shouldn't be nil"))
	}

	var typ reflect.Type

	switch t := typOrPtr.(type) {
	case reflect.Type:
		typ = t
	default:
		typ = reflect.TypeOf(t)
	}

	for { // 去掉指针和数组的包装，以获得原始类型
		if k := typ.Kind(); k == reflect.Ptr || k == reflect.Slice {
			typ = typ.Elem()
		} else {
			break
		}
	}

	if pkgPath := typ.PkgPath(); pkgPath != "" {
		return pkgPath + "/" + typ.String()
	} else { // 内置类型的路径为空
		return typ.String()
	}
}

type Definition interface {
	Bean() interface{}    // 源
	Type() reflect.Type   // 类型
	Value() reflect.Value // 值
	TypeName() string     // 原始类型的全限定名
	Name() string         // 返回 Bean 的名称
	BeanId() string       // 返回 Bean 的唯一 ID
	FileLine() string     // 返回 Bean 的注册点
	Description() string  // 返回 Bean 的详细描述
}

type Assembly interface {

	// ConditionContext 获取条件上下文
	ConditionContext() interface{}

	// BindValue 对结构体的字段进行属性绑定
	BindValue(v reflect.Value, str string, opt conf.BindOption) error

	// WireStructField 对结构体的字段进行绑定
	WireStructField(v reflect.Value, tag string, parent reflect.Value, field string)
}

type Runnable interface {
	Run(assembly Assembly, receiver ...reflect.Value) error
}
