package bean

import (
	"reflect"

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

type Definition interface {
	Bean() interface{}    // 源
	Type() reflect.Type   // 类型
	Value() reflect.Value // 值

	BeanId() string   // 返回 Bean 的唯一 ID
	BeanName() string // 返回 Bean 的名称
	TypeName() string // 原始类型的全限定名

	FileLine() string    // 返回 Bean 的注册点
	Description() string // 返回 Bean 的详细描述
}

type Assembly interface {

	// ConditionContext 获取条件上下文
	ConditionContext() interface{}

	// BindValue 对结构体的字段进行属性绑定
	BindValue(v reflect.Value, str string) error

	// WireValue 对结构体的字段进行绑定
	WireValue(v reflect.Value, tag string) error
}

type Runnable interface {
	Run(assembly Assembly, receiver ...reflect.Value) error
}
