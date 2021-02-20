package bean

import (
	"errors"
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

// Status Bean 的状态值
type Status int

const (
	Default   = Status(0) // 默认状态
	Resolving = Status(1) // 正在决议
	Resolved  = Status(2) // 已决议
	Wiring    = Status(3) // 正在注入
	Wired     = Status(4) // 注入完成
	Deleted   = Status(5) // 已删除
)

type BeanFactory interface {
	BeanClass() string
	NewValue() reflect.Value
	BeanType() reflect.Type
}

type Definition interface {
	Bean() interface{}    // 源
	Type() reflect.Type   // 类型
	Value() reflect.Value // 值

	BeanId() string   // 返回 Bean 的唯一 ID
	BeanName() string // 返回 Bean 的名称
	TypeName() string // 原始类型的全限定名

	FileLine() string    // 返回 Bean 的注册点
	Description() string // 返回 Bean 的详细描述

	BeanFactory() BeanFactory
	GetStatus() Status        // 返回 Bean 的状态值
	GetDependsOn() []Selector // 返回 Bean 的间接依赖项
	GetInit() Runnable        // 返回 Bean 的初始化函数
	GetDestroy() Runnable     // 返回 Bean 的销毁函数
	GetFile() string          // 返回 Bean 注册点所在文件的名称
	GetLine() int             // 返回 Bean 注册点所在文件的行数

	SetValue(reflect.Value)  // 设置新的值
	SetStatus(status Status) // 设置 Bean 的状态值
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
