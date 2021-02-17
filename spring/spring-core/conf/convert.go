package conf

import (
	"errors"
	"reflect"
	"time"

	"github.com/go-spring/spring-core/bean"
	"github.com/spf13/cast"
)

var converters = map[reflect.Type]interface{}{}

func init() {

	// 注册时长转换函数 string -> time.Duration converter
	// time units are "ns", "us" (or "µs"), "ms", "s", "m", "h"。
	Convert(func(s string) (time.Duration, error) { return cast.ToDurationE(s) })

	// 注册日期转换函数 string -> time.Time converter
	// 支持非常多的日期格式，参见 cast.StringToDate。
	Convert(func(s string) (time.Time, error) { return cast.ToTimeE(s) })
}

// validConverter 返回是否是合法的类型转换器。
func validConverter(t reflect.Type) bool {

	if t.Kind() != reflect.Func || t.NumIn() != 1 || t.NumOut() != 2 {
		return false
	}

	if t.In(0).Kind() != reflect.String {
		return false
	}

	return bean.IsValueType(t.Out(0).Kind()) && t.Out(1) == errorType
}

// Convert 添加类型转换器，函数原型 func(string)(type,error)
func Convert(fn interface{}) {
	if t := reflect.TypeOf(fn); validConverter(t) {
		converters[t.Out(0)] = fn
		return
	}
	panic(errors.New("fn must be func(string)(type,error)"))
}
