package core_test

import (
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/go-spring/spring-core/core"
	pkg2 "github.com/go-spring/spring-core/core/testdata/pkg/foo"
	"github.com/go-spring/spring-core/util"
)

//func TestParseSingletonTag(t *testing.T) {
//
//	data := map[string]SingletonTag{
//		"[]":     {"", "[]", false},
//		"[]?":    {"", "[]", true},
//		"i":      {"", "i", false},
//		"i?":     {"", "i", true},
//		":i":     {"", "i", false},
//		":i?":    {"", "i", true},
//		"int:i":  {"int", "i", false},
//		"int:i?": {"int", "i", true},
//		"int:":   {"int", "", false},
//		"int:?":  {"int", "", true},
//	}
//
//	for k, v := range data {
//		tag := parseSingletonTag(k)
//		util.AssertEqual(t, tag, v)
//	}
//}
//
//func TestParseBeanTag(t *testing.T) {
//
//	data := map[string]collectionTag{
//		"[]":  {[]SingletonTag{}, false},
//		"[]?": {[]SingletonTag{}, true},
//	}
//
//	for k, v := range data {
//		tag := ParseCollectionTag(k)
//		util.AssertEqual(t, tag, v)
//	}
//}

func TestIsFuncBeanType(t *testing.T) {

	type S struct{}
	type OptionFunc func(*S)

	data := map[reflect.Type]bool{
		reflect.TypeOf((func())(nil)):            false,
		reflect.TypeOf((func(int))(nil)):         false,
		reflect.TypeOf((func(int, int))(nil)):    false,
		reflect.TypeOf((func(int, ...int))(nil)): false,

		reflect.TypeOf((func() int)(nil)):          true,
		reflect.TypeOf((func() (int, int))(nil)):   false,
		reflect.TypeOf((func() (int, error))(nil)): true,

		reflect.TypeOf((func(int) int)(nil)):         true,
		reflect.TypeOf((func(int, int) int)(nil)):    true,
		reflect.TypeOf((func(int, ...int) int)(nil)): true,

		reflect.TypeOf((func(int) (int, error))(nil)):         true,
		reflect.TypeOf((func(int, int) (int, error))(nil)):    true,
		reflect.TypeOf((func(int, ...int) (int, error))(nil)): true,

		reflect.TypeOf((func() S)(nil)):          true,
		reflect.TypeOf((func() *S)(nil)):         true,
		reflect.TypeOf((func() (S, error))(nil)): true,

		reflect.TypeOf((func(OptionFunc) (*S, error))(nil)):    true,
		reflect.TypeOf((func(...OptionFunc) (*S, error))(nil)): true,
	}

	for k, v := range data {
		ok := core.IsFuncBeanType(k)
		util.AssertEqual(t, ok, v)
	}
}

func TestBeanDefinition_Match(t *testing.T) {

	data := []struct {
		bd       *core.BeanDefinition
		typeName string
		beanName string
		expect   bool
	}{
		{core.ObjBean(new(int)), "int", "*int", true},
		{core.ObjBean(new(int)), "", "*int", true},
		{core.ObjBean(new(int)), "int", "", true},
		{core.ObjBean(new(int)).WithName("i"), "int", "i", true},
		{core.ObjBean(new(int)).WithName("i"), "", "i", true},
		{core.ObjBean(new(int)).WithName("i"), "int", "", true},
		{core.ObjBean(new(pkg2.SamePkg)), "github.com/go-spring/spring-core/core/testdata/pkg/foo/pkg.SamePkg", "*pkg.SamePkg", true},
		{core.ObjBean(new(pkg2.SamePkg)), "", "*pkg.SamePkg", true},
		{core.ObjBean(new(pkg2.SamePkg)), "github.com/go-spring/spring-core/core/testdata/pkg/foo/pkg.SamePkg", "", true},
		{core.ObjBean(new(pkg2.SamePkg)).WithName("pkg2"), "github.com/go-spring/spring-core/core/testdata/pkg/foo/pkg.SamePkg", "pkg2", true},
		{core.ObjBean(new(pkg2.SamePkg)).WithName("pkg2"), "", "pkg2", true},
		{core.ObjBean(new(pkg2.SamePkg)).WithName("pkg2"), "github.com/go-spring/spring-core/core/testdata/pkg/foo/pkg.SamePkg", "pkg2", true},
	}

	for i, s := range data {
		if ok := s.bd.Match(s.typeName, s.beanName); ok != s.expect {
			t.Errorf("%d expect %v but %v", i, s.expect, ok)
		}
	}
}

type BeanZero struct {
	Int int
}

type BeanOne struct {
	Zero *BeanZero `autowire:""`
}

type BeanTwo struct {
	One *BeanOne `autowire:""`
}

func (t *BeanTwo) Group() {
}

type BeanThree struct {
	One *BeanTwo `autowire:""`
}

func (t *BeanThree) String() string {
	return ""
}

func TestObjectBean(t *testing.T) {

	t.Run("bean can't be nil", func(t *testing.T) {

		util.AssertPanic(t, func() {
			core.ObjBean(nil)
		}, "bean can't be nil")

		util.AssertPanic(t, func() {
			var i *int
			core.ObjBean(i)
		}, "bean can't be nil")

		util.AssertPanic(t, func() {
			var m map[string]string
			core.ObjBean(m)
		}, "bean can't be nil")
	})

	t.Run("bean must be ref type", func(t *testing.T) {

		data := []func(){
			func() { core.ObjBean([...]int{0}) },
			func() { core.ObjBean(false) },
			func() { core.ObjBean(3) },
			func() { core.ObjBean("3") },
			func() { core.ObjBean(BeanZero{}) },
			func() { core.ObjBean(pkg2.SamePkg{}) },
		}

		for _, fn := range data {
			util.AssertPanic(t, fn, "bean must be ref type")
		}
	})

	t.Run("valid bean", func(t *testing.T) {
		core.ObjBean(make(chan int))
		core.ObjBean(func() {})
		core.ObjBean(make(map[string]int))
		core.ObjBean(new(int))
		core.ObjBean(&BeanZero{})
		core.ObjBean(make([]int, 0))
	})

	t.Run("check name && typename", func(t *testing.T) {

		data := map[*core.BeanDefinition]struct {
			name     string
			typeName string
		}{
			core.ObjBean(io.Writer(os.Stdout)): {
				"*os.File", "os/os.File",
			},

			core.ObjBean(newHistoryTeacher("")): {
				"*core_test.historyTeacher",
				"github.com/go-spring/spring-core/core_test/core_test.historyTeacher",
			},

			core.ObjBean(new(int)): {
				"*int", "int",
			},

			core.ObjBean(new(int)).WithName("i"): {
				"i", "int",
			},

			core.ObjBean(new(pkg2.SamePkg)): {
				"*pkg.SamePkg",
				"github.com/go-spring/spring-core/core/testdata/pkg/foo/pkg.SamePkg",
			},

			core.ObjBean(new(pkg2.SamePkg)).WithName("pkg2"): {
				"pkg2",
				"github.com/go-spring/spring-core/core/testdata/pkg/foo/pkg.SamePkg",
			},
		}

		for bd, v := range data {
			util.AssertEqual(t, bd.Name(), v.name)
			util.AssertEqual(t, bd.TypeName(), v.typeName)
		}
	})
}

func TestConstructorBean(t *testing.T) {

	bd := core.CtorBean(NewStudent)
	util.AssertEqual(t, bd.Type().String(), "*core_test.Student")

	bd = core.CtorBean(NewPtrStudent)
	util.AssertEqual(t, bd.Type().String(), "*core_test.Student")

	mapFn := func() map[int]string { return make(map[int]string) }
	bd = core.CtorBean(mapFn)
	util.AssertEqual(t, bd.Type().String(), "map[int]string")

	sliceFn := func() []int { return make([]int, 1) }
	bd = core.CtorBean(sliceFn)
	util.AssertEqual(t, bd.Type().String(), "[]int")

	funcFn := func() func(int) { return nil }
	bd = core.CtorBean(funcFn)
	util.AssertEqual(t, bd.Type().String(), "func(int)")

	intFn := func() int { return 0 }
	bd = core.CtorBean(intFn)
	util.AssertEqual(t, bd.Type().String(), "*int")

	interfaceFn := func(name string) Teacher { return newHistoryTeacher(name) }
	bd = core.CtorBean(interfaceFn)
	util.AssertEqual(t, bd.Type().String(), "core_test.Teacher")

	util.AssertPanic(t, func() {
		bd = core.CtorBean(func() (*int, *int) { return nil, nil })
		util.AssertEqual(t, bd.Type().String(), "*int")
	}, "func bean must be func\\(...\\)bean or func\\(...\\)\\(bean, error\\)")

	bd = core.CtorBean(func() (*int, error) { return nil, nil })
	util.AssertEqual(t, bd.Type().String(), "*int")
}
