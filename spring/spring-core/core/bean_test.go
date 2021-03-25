package core_test

import (
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/go-spring/spring-core/assert"
	"github.com/go-spring/spring-core/core"
	pkg2 "github.com/go-spring/spring-core/core/testdata/pkg/foo"
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
//		util.Equal(t, tag, v)
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
//		util.Equal(t, tag, v)
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
		assert.Equal(t, ok, v)
	}
}

func TestBeanDefinition_Match(t *testing.T) {

	data := []struct {
		bd       *core.BeanDefinition
		typeName string
		beanName string
		expect   bool
	}{
		{core.Bean(new(int)), "int", "*int", true},
		{core.Bean(new(int)), "", "*int", true},
		{core.Bean(new(int)), "int", "", true},
		{core.Bean(new(int)).Name("i"), "int", "i", true},
		{core.Bean(new(int)).Name("i"), "", "i", true},
		{core.Bean(new(int)).Name("i"), "int", "", true},
		{core.Bean(new(pkg2.SamePkg)), "github.com/go-spring/spring-core/core/testdata/pkg/foo/pkg.SamePkg", "*pkg.SamePkg", true},
		{core.Bean(new(pkg2.SamePkg)), "", "*pkg.SamePkg", true},
		{core.Bean(new(pkg2.SamePkg)), "github.com/go-spring/spring-core/core/testdata/pkg/foo/pkg.SamePkg", "", true},
		{core.Bean(new(pkg2.SamePkg)).Name("pkg2"), "github.com/go-spring/spring-core/core/testdata/pkg/foo/pkg.SamePkg", "pkg2", true},
		{core.Bean(new(pkg2.SamePkg)).Name("pkg2"), "", "pkg2", true},
		{core.Bean(new(pkg2.SamePkg)).Name("pkg2"), "github.com/go-spring/spring-core/core/testdata/pkg/foo/pkg.SamePkg", "pkg2", true},
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

		assert.Panic(t, func() {
			core.Bean(nil)
		}, "bean can't be nil")

		assert.Panic(t, func() {
			var i *int
			core.Bean(i)
		}, "bean can't be nil")

		assert.Panic(t, func() {
			var m map[string]string
			core.Bean(m)
		}, "bean can't be nil")
	})

	t.Run("bean must be ref type", func(t *testing.T) {

		data := []func(){
			func() { core.Bean([...]int{0}) },
			func() { core.Bean(false) },
			func() { core.Bean(3) },
			func() { core.Bean("3") },
			func() { core.Bean(BeanZero{}) },
			func() { core.Bean(pkg2.SamePkg{}) },
		}

		for _, fn := range data {
			assert.Panic(t, fn, "bean must be ref type")
		}
	})

	t.Run("valid bean", func(t *testing.T) {
		core.Bean(make(chan int))
		core.Bean(reflect.ValueOf(func() {}))
		core.Bean(make(map[string]int))
		core.Bean(new(int))
		core.Bean(&BeanZero{})
		core.Bean(make([]int, 0))
	})

	t.Run("check name && typename", func(t *testing.T) {

		data := map[*core.BeanDefinition]struct {
			name     string
			typeName string
		}{
			core.Bean(io.Writer(os.Stdout)): {
				"*os.File", "os/os.File",
			},

			core.Bean(newHistoryTeacher("")): {
				"*core_test.historyTeacher",
				"github.com/go-spring/spring-core/core_test/core_test.historyTeacher",
			},

			core.Bean(new(int)): {
				"*int", "int",
			},

			core.Bean(new(int)).Name("i"): {
				"i", "int",
			},

			core.Bean(new(pkg2.SamePkg)): {
				"*pkg.SamePkg",
				"github.com/go-spring/spring-core/core/testdata/pkg/foo/pkg.SamePkg",
			},

			core.Bean(new(pkg2.SamePkg)).Name("pkg2"): {
				"pkg2",
				"github.com/go-spring/spring-core/core/testdata/pkg/foo/pkg.SamePkg",
			},
		}

		for bd, v := range data {
			assert.Equal(t, bd.BeanName(), v.name)
			assert.Equal(t, bd.TypeName(), v.typeName)
		}
	})
}

func TestConstructorBean(t *testing.T) {

	bd := core.Bean(NewStudent)
	assert.Equal(t, bd.Type().String(), "*core_test.Student")

	bd = core.Bean(NewPtrStudent)
	assert.Equal(t, bd.Type().String(), "*core_test.Student")

	mapFn := func() map[int]string { return make(map[int]string) }
	bd = core.Bean(mapFn)
	assert.Equal(t, bd.Type().String(), "map[int]string")

	sliceFn := func() []int { return make([]int, 1) }
	bd = core.Bean(sliceFn)
	assert.Equal(t, bd.Type().String(), "[]int")

	funcFn := func() func(int) { return nil }
	bd = core.Bean(funcFn)
	assert.Equal(t, bd.Type().String(), "func(int)")

	intFn := func() int { return 0 }
	bd = core.Bean(intFn)
	assert.Equal(t, bd.Type().String(), "*int")

	interfaceFn := func(name string) Teacher { return newHistoryTeacher(name) }
	bd = core.Bean(interfaceFn)
	assert.Equal(t, bd.Type().String(), "core_test.Teacher")

	assert.Panic(t, func() {
		_ = core.Bean(func() (*int, *int) { return nil, nil })
	}, "func bean must be func\\(...\\)bean or func\\(...\\)\\(bean, error\\)")

	bd = core.Bean(func() (*int, error) { return nil, nil })
	assert.Equal(t, bd.Type().String(), "*int")
}
