package util

import (
	"reflect"
)

// errorType error 的反射类型
var errorType = reflect.TypeOf((*error)(nil)).Elem()

// FuncType 是否是函数类型
func FuncType(fnType reflect.Type) bool {
	return fnType.Kind() == reflect.Func
}

// ReturnNothing 函数是否无返回值
func ReturnNothing(fnType reflect.Type) bool {
	return fnType.NumOut() == 0
}

// ReturnOnlyError 函数是否只返回错误值
func ReturnOnlyError(fnType reflect.Type) bool {
	return fnType.NumOut() == 1 && fnType.Out(0) == errorType
}

// WithReceiver 函数是否具有接收者
func WithReceiver(fnType reflect.Type, receiver reflect.Type) bool {
	return fnType.NumIn() >= 1 && fnType.In(0) == receiver
}
