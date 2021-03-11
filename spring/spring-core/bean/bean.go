package bean

import (
	"reflect"
)

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
