/*
 * Copyright 2025 The Go-Spring Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package gs_arg

import (
	"bytes"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"go-spring.org/gs-mock/gsmock"
	"go-spring.org/spring/gs/internal/gs"
	"go-spring.org/spring/gs/internal/gs_cond"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/testing/assert"
)

func TestTagArg(t *testing.T) {

	t.Run("empty tag", func(t *testing.T) {
		m := gsmock.NewManager()
		c := gs.NewArgContextMockImpl(m)
		c.MockBind().Handle(func(v reflect.Value, s string) error {
			v.SetString("default")
			return nil
		})

		tag := Tag("")
		v, err := tag.GetArgValue(c, reflect.TypeFor[string]())
		assert.That(t, err).Nil()
		assert.That(t, v.String()).Equal("default")
	})

	t.Run("bind success", func(t *testing.T) {
		m := gsmock.NewManager()
		c := gs.NewArgContextMockImpl(m)
		c.MockBind().Handle(func(v reflect.Value, s string) error {
			v.SetString("3")
			return nil
		})

		tag := Tag("${int:=3}")
		v, err := tag.GetArgValue(c, reflect.TypeFor[string]())
		assert.That(t, err).Nil()
		assert.That(t, v.String()).Equal("3")
	})

	t.Run("bind error", func(t *testing.T) {
		m := gsmock.NewManager()
		c := gs.NewArgContextMockImpl(m)
		c.MockBind().Handle(func(v reflect.Value, s string) error {
			return errutil.Explain(nil, "bind error")
		})

		tag := Tag("${int:=3}")
		_, err := tag.GetArgValue(c, reflect.TypeFor[string]())
		assert.Error(t, err).Matches("bind error")
	})

	t.Run("wire success", func(t *testing.T) {
		m := gsmock.NewManager()
		c := gs.NewArgContextMockImpl(m)
		c.MockWire().Handle(func(v reflect.Value, s string) error {
			v.Set(reflect.ValueOf(&http.Server{Addr: ":9090"}))
			return nil
		})

		tag := Tag("http-server")
		v, err := tag.GetArgValue(c, reflect.TypeFor[*http.Server]())
		assert.That(t, err).Nil()
		assert.That(t, v.Interface().(*http.Server).Addr).Equal(":9090")
	})

	t.Run("wire error", func(t *testing.T) {
		m := gsmock.NewManager()
		c := gs.NewArgContextMockImpl(m)
		c.MockWire().Handle(func(v reflect.Value, s string) error {
			return errutil.Explain(nil, "wire error")
		})

		tag := Tag("server")
		_, err := tag.GetArgValue(c, reflect.TypeFor[*bytes.Buffer]())
		assert.Error(t, err).Matches("wire error")
	})

	t.Run("unsupported type", func(t *testing.T) {
		tag := Tag("server")
		_, err := tag.GetArgValue(nil, reflect.TypeFor[*string]())
		assert.Error(t, err).Matches("unsupported argument type \\*string")
	})
}

func TestValueArg(t *testing.T) {

	t.Run("different types", func(t *testing.T) {
		tag := Value(42)
		v, err := tag.GetArgValue(nil, reflect.TypeFor[int]())
		assert.That(t, err).Nil()
		assert.That(t, v.Int()).Equal(int64(42))

		tag = Value(true)
		v, err = tag.GetArgValue(nil, reflect.TypeFor[bool]())
		assert.That(t, err).Nil()
		assert.That(t, v.Bool()).True()

		tag = Value(3.14)
		v, err = tag.GetArgValue(nil, reflect.TypeFor[float64]())
		assert.That(t, err).Nil()
		assert.That(t, v.Float()).Equal(3.14)
	})

	t.Run("slice value", func(t *testing.T) {
		slice := []string{"a", "b", "c"}
		tag := Value(slice)
		v, err := tag.GetArgValue(nil, reflect.TypeFor[[]string]())
		assert.That(t, err).Nil()
		result := v.Interface().([]string)
		assert.That(t, result).Equal(slice)
	})

	t.Run("index arg", func(t *testing.T) {
		arg := Index(0, Value(1))
		assert.That(t, arg.(IndexArg).Idx).Equal(0)
		assert.Panic(t, func() {
			_, _ = arg.GetArgValue(nil, reflect.TypeFor[int]())
		}, "unimplemented method")
	})

	t.Run("zero value", func(t *testing.T) {
		tag := Value(nil)
		v, err := tag.GetArgValue(nil, reflect.TypeFor[*http.Server]())
		assert.That(t, err).Nil()
		assert.That(t, v.Interface())
	})

	t.Run("assignable value", func(t *testing.T) {
		tag := Value(&http.Server{Addr: ":9090"})
		v, err := tag.GetArgValue(nil, reflect.TypeFor[*http.Server]())
		assert.That(t, err).Nil()
		assert.That(t, v.Interface().(*http.Server).Addr).Equal(":9090")
	})

	t.Run("incompatible types", func(t *testing.T) {
		tag := Value(new(int))
		_, err := tag.GetArgValue(nil, reflect.TypeFor[*http.Server]())
		assert.Error(t, err).Matches("cannot assign type \\*int to type \\*http.Server")
	})
}

func TestArgList_New(t *testing.T) {

	t.Run("invalid function type", func(t *testing.T) {
		fnType := reflect.TypeFor[int]()
		_, err := NewArgList(fnType, nil)
		assert.Error(t, err).Matches("expected function type, got int")
	})

	t.Run("mixed index and non-index args", func(t *testing.T) {
		fnType := reflect.TypeFor[func(a int, b string)]()
		args := []gs.Arg{
			Index(0, Value(1)),
			Value("test"),
		}
		_, err := NewArgList(fnType, args)
		assert.Error(t, err).Matches("arguments must be all indexed or non-indexed")
	})

	t.Run("mixed non-index and index args", func(t *testing.T) {
		fnType := reflect.TypeFor[func(a int, b string)]()
		args := []gs.Arg{
			Value(1),
			Index(1, Value("test")),
		}
		_, err := NewArgList(fnType, args)
		assert.Error(t, err).Matches("arguments must be all indexed or non-indexed")
	})

	t.Run("negative argument index", func(t *testing.T) {
		fnType := reflect.TypeFor[func(a int, b string)]()
		args := []gs.Arg{
			Index(-1, Value(1)),
		}
		_, err := NewArgList(fnType, args)
		assert.Error(t, err).Matches("invalid argument index -1")
	})

	t.Run("out of range argument index", func(t *testing.T) {
		fnType := reflect.TypeFor[func(a int, b string)]()
		args := []gs.Arg{
			Index(2, Value(1)),
		}
		_, err := NewArgList(fnType, args)
		assert.Error(t, err).Matches("invalid argument index 2")
	})

	t.Run("duplicate fixed argument index", func(t *testing.T) {
		fnType := reflect.TypeFor[func(a int, b string)]()
		args := []gs.Arg{
			Index(0, Value(1)),
			Index(0, Value(2)),
		}
		_, err := NewArgList(fnType, args)
		assert.Error(t, err).Matches("duplicate argument index 0")
	})

	t.Run("non-index args success", func(t *testing.T) {
		fnType := reflect.TypeFor[func(a int, b string)]()
		args := []gs.Arg{
			Value(1),
			Value("test"),
		}
		argList, err := NewArgList(fnType, args)
		assert.That(t, err).Nil()
		assert.That(t, argList).NotNil()
		assert.That(t, argList.args).Equal([]gs.Arg{
			Value(1),
			Value("test"),
		})
	})

	t.Run("index args success", func(t *testing.T) {
		fnType := reflect.TypeFor[func(a int, b string)]()
		args := []gs.Arg{
			Index(0, Value(1)),
			Index(1, Value("test")),
		}
		argList, err := NewArgList(fnType, args)
		assert.That(t, err).Nil()
		assert.That(t, argList).NotNil()
		assert.That(t, argList.args).Equal([]gs.Arg{
			Value(1),
			Value("test"),
		})
	})

	t.Run("variadic success with non-index args", func(t *testing.T) {
		fnType := reflect.TypeFor[func(a int, b ...string)]()
		args := []gs.Arg{
			Value(1),
			Value("test1"),
			Value("test2"),
		}
		argList, err := NewArgList(fnType, args)
		assert.That(t, err).Nil()
		assert.That(t, argList).NotNil()
		assert.That(t, argList.args).Equal([]gs.Arg{
			Value(1),
			Value("test1"),
			Value("test2"),
		})
	})

	t.Run("variadic success with indexed args", func(t *testing.T) {
		fnType := reflect.TypeFor[func(a int, b ...string)]()
		args := []gs.Arg{
			Index(0, Value(1)),
			Index(1, Value("test1")),
			Index(1, Value("test2")),
		}
		argList, err := NewArgList(fnType, args)
		assert.That(t, err).Nil()
		assert.That(t, argList).NotNil()
		assert.That(t, argList.args).Equal([]gs.Arg{
			Value(1),
			Value("test1"),
			Value("test2"),
		})
	})

	t.Run("variadic success with partial indexed args", func(t *testing.T) {
		fnType := reflect.TypeFor[func(a error, b ...string)]()
		args := []gs.Arg{
			Index(1, Value("test1")),
			Index(1, Value("test2")),
		}
		argList, err := NewArgList(fnType, args)
		assert.That(t, err).Nil()
		assert.That(t, argList).NotNil()
		assert.That(t, argList.args).Equal([]gs.Arg{
			Tag(""),
			Value("test1"),
			Value("test2"),
		})
	})

	t.Run("function with no parameters", func(t *testing.T) {
		fnType := reflect.TypeFor[func()]()
		var args []gs.Arg
		argList, err := NewArgList(fnType, args)
		assert.That(t, err).Nil()
		assert.That(t, argList).NotNil()
		assert.That(t, len(argList.args)).Equal(0)
	})

	t.Run("too many arguments for non-variadic function", func(t *testing.T) {
		fnType := reflect.TypeFor[func(a int, b string)]()
		args := []gs.Arg{
			Value(1),
			Value("test"),
			Value("extra"), // Extra argument
		}
		_, err := NewArgList(fnType, args)
		assert.Error(t, err).Matches("too many arguments")
	})
}

func TestArgList_Get(t *testing.T) {

	t.Run("success with non-variadic function", func(t *testing.T) {
		fnType := reflect.TypeFor[func(a int, b string)]()
		args := []gs.Arg{
			Value(1),
			Value("test"),
		}
		argList, err := NewArgList(fnType, args)
		assert.That(t, err).Nil()

		ctx := gs.NewArgContextMockImpl(nil)
		values, err := argList.get(ctx)
		assert.That(t, err).Nil()
		assert.That(t, 2).Equal(len(values))
		assert.That(t, 1).Equal(values[0].Interface().(int))
		assert.That(t, "test").Equal(values[1].Interface().(string))
	})

	t.Run("success with variadic function", func(t *testing.T) {
		fnType := reflect.TypeFor[func(a int, b ...string)]()
		args := []gs.Arg{
			Value(1),
			Value("test1"),
			Value("test2"),
		}
		argList, err := NewArgList(fnType, args)
		assert.That(t, err).Nil()

		ctx := gs.NewArgContextMockImpl(nil)
		values, err := argList.get(ctx)
		assert.That(t, err).Nil()
		assert.That(t, 3).Equal(len(values))
		assert.That(t, 1).Equal(values[0].Interface().(int))
		assert.That(t, "test1").Equal(values[1].Interface().(string))
		assert.That(t, "test2").Equal(values[2].Interface().(string))
	})

	t.Run("error when getting arg value", func(t *testing.T) {
		fnType := reflect.TypeFor[func(a int, b string)]()
		args := []gs.Arg{
			Value(1),
			Value(2),
		}
		argList, err := NewArgList(fnType, args)
		assert.That(t, err).Nil()

		ctx := gs.NewArgContextMockImpl(nil)
		_, err = argList.get(ctx)
		assert.Error(t, err).Matches("cannot assign type int to type string")
	})

	t.Run("variadic function with no extra args", func(t *testing.T) {
		fnType := reflect.TypeFor[func(a int, b ...string)]()
		args := []gs.Arg{
			Value(1),
			// No extra args
		}
		argList, err := NewArgList(fnType, args)
		assert.That(t, err).Nil()

		ctx := gs.NewArgContextMockImpl(nil)
		values, err := argList.get(ctx)
		assert.That(t, err).Nil()
		assert.That(t, 1).Equal(len(values))
		assert.That(t, 1).Equal(values[0].Interface().(int))
	})

	t.Run("function with any parameter", func(t *testing.T) {
		fnType := reflect.TypeFor[func(a any)]()
		args := []gs.Arg{
			Value("test"),
		}
		argList, err := NewArgList(fnType, args)
		assert.That(t, err).Nil()

		ctx := gs.NewArgContextMockImpl(nil)
		values, err := argList.get(ctx)
		assert.That(t, err).Nil()
		assert.That(t, 1).Equal(len(values))
		assert.That(t, "test").Equal(values[0].Interface())
	})
}

func TestCallable_New(t *testing.T) {

	t.Run("nil function", func(t *testing.T) {
		_, err := NewCallable(nil, nil)
		assert.Error(t, err).Matches("callable function cannot be nil")

		var fn func() string
		_, err = NewCallable(fn, nil)
		assert.Error(t, err).Matches("callable function value is nil")
	})

	t.Run("invalid function type", func(t *testing.T) {
		fn := "not a function"
		args := []gs.Arg{
			Value(1),
			Value("test"),
		}
		_, err := NewCallable(fn, args)
		assert.Error(t, err).Matches("callable function must be a function type, got string")
	})

	t.Run("error in argument processing", func(t *testing.T) {
		fn := func(a int, b string) string {
			return fmt.Sprintf("%d-%s", a, b)
		}
		args := []gs.Arg{
			Value(1),
			Value(2),
		}
		callable, err := NewCallable(fn, args)
		assert.That(t, err).Nil()

		ctx := gs.NewArgContextMockImpl(nil)
		_, err = callable.Call(ctx)
		assert.Error(t, err).Matches("cannot assign type int to type string")
	})
}

func TestCallable_Call(t *testing.T) {

	t.Run("error in get arg value", func(t *testing.T) {
		fn := func(a int, b string) (string, error) {
			return "", nil
		}
		args := []gs.Arg{
			Value(1),
			Value(2),
		}
		callable, err := NewCallable(fn, args)
		assert.That(t, err).Nil()

		ctx := gs.NewArgContextMockImpl(nil)
		_, err = callable.Call(ctx)
		assert.Error(t, err).Matches("cannot assign type int to type string")
	})

	t.Run("function return none", func(t *testing.T) {
		fn := func(a int, b string) {}
		args := []gs.Arg{
			Value(1),
			Value("test"),
		}
		callable, err := NewCallable(fn, args)
		assert.That(t, err).Nil()

		ctx := gs.NewArgContextMockImpl(nil)
		v, err := callable.Call(ctx)
		assert.That(t, err).Nil()
		assert.That(t, len(v)).Equal(0)
	})

	t.Run("function return error", func(t *testing.T) {
		fn := func(a int, b string) (string, error) {
			return "", errutil.Explain(nil, "execution error")
		}
		args := []gs.Arg{
			Value(1),
			Value("test"),
		}
		callable, err := NewCallable(fn, args)
		assert.That(t, err).Nil()

		ctx := gs.NewArgContextMockImpl(nil)
		v, err := callable.Call(ctx)
		assert.That(t, err).Nil()
		assert.That(t, 2).Equal(len(v))
		assert.That(t, "").Equal(v[0].Interface().(string))
		assert.Error(t, v[1].Interface().(error)).String("execution error")
	})

	t.Run("success with function with error", func(t *testing.T) {
		fn := func(a int, b string) (string, error) {
			return fmt.Sprintf("%d-%s", a, b), nil
		}
		args := []gs.Arg{
			Value(1),
			Value("test"),
		}
		callable, err := NewCallable(fn, args)
		assert.That(t, err).Nil()

		ctx := gs.NewArgContextMockImpl(nil)
		v, err := callable.Call(ctx)
		assert.That(t, err).Nil()
		assert.That(t, len(v)).Equal(2)
		assert.That(t, "1-test").Equal(v[0].Interface().(string))
	})

	t.Run("success with function without error", func(t *testing.T) {
		fn := func(a int, b string) string {
			return fmt.Sprintf("%d-%s", a, b)
		}
		args := []gs.Arg{
			Value(1),
			Value("test"),
		}
		callable, err := NewCallable(fn, args)
		assert.That(t, err).Nil()

		ctx := gs.NewArgContextMockImpl(nil)
		v, err := callable.Call(ctx)
		assert.That(t, err).Nil()
		assert.That(t, len(v)).Equal(1)
		assert.That(t, "1-test").Equal(v[0].Interface().(string))
	})

	t.Run("success with variadic function", func(t *testing.T) {
		fn := func(a int, b ...string) string {
			return fmt.Sprintf("%d-%s", a, strings.Join(b, ","))
		}
		args := []gs.Arg{
			Value(1),
			Value("test1"),
			Value("test2"),
		}
		callable, err := NewCallable(fn, args)
		assert.That(t, err).Nil()

		ctx := gs.NewArgContextMockImpl(nil)
		v, err := callable.Call(ctx)
		assert.That(t, err).Nil()
		assert.That(t, len(v)).Equal(1)
		assert.That(t, "1-test1,test2").Equal(v[0].Interface().(string))
	})

	t.Run("function with pointer receiver", func(t *testing.T) {
		type MyStruct struct {
			Value string
		}

		fn := func(s *MyStruct, suffix string) string {
			return s.Value + "-" + suffix
		}

		args := []gs.Arg{
			Value(&MyStruct{Value: "test"}),
			Value("suffix"),
		}

		callable, err := NewCallable(fn, args)
		assert.That(t, err).Nil()

		ctx := gs.NewArgContextMockImpl(nil)
		v, err := callable.Call(ctx)
		assert.That(t, err).Nil()
		assert.That(t, len(v)).Equal(1)
		assert.That(t, "test-suffix").Equal(v[0].Interface().(string))
	})

	t.Run("variadic function with no variadic args", func(t *testing.T) {
		fn := func(a int, b ...string) []string {
			return append([]string{fmt.Sprint(a)}, b...)
		}

		args := []gs.Arg{
			Value(1),
			// No variadic arguments
		}

		callable, err := NewCallable(fn, args)
		assert.That(t, err).Nil()

		ctx := gs.NewArgContextMockImpl(nil)
		v, err := callable.Call(ctx)
		assert.That(t, err).Nil()
		assert.That(t, len(v)).Equal(1)
		result := v[0].Interface().([]string)
		assert.That(t, result).Equal([]string{"1"})
	})
}

func TestBindArg_Bind(t *testing.T) {

	t.Run("nil function", func(t *testing.T) {
		assert.Panic(t, func() {
			Bind(nil)
		}, "callable function cannot be nil")

		var fn func() string
		assert.Panic(t, func() {
			Bind(fn)
		}, "callable function value is nil")
	})

	t.Run("function returning only error", func(t *testing.T) {
		fn := func(a int, b string) error {
			return nil
		}
		args := []gs.Arg{
			Value(1),
			Value("test"),
		}
		assert.Panic(t, func() {
			Bind(fn, args...)
		}, "invalid bind function signature: expected func\\(.*\\) error or func\\(.*\\) \\(T, error\\)")
	})

	t.Run("non-function type", func(t *testing.T) {
		fn := "not a function"
		assert.Panic(t, func() {
			Bind(fn)
		}, "callable function must be a function type, got string")
	})

	t.Run("function returning only error", func(t *testing.T) {
		fn := func(a int, b string) error {
			return nil
		}
		assert.Panic(t, func() {
			Bind(fn)
		}, "invalid bind function signature: expected func\\(.*\\) error or func\\(.*\\) \\(T, error\\)")
	})

	t.Run("function with invalid return types", func(t *testing.T) {
		fn := func(a int, b string) (string, bool) {
			return fmt.Sprintf("%d-%s", a, b), true
		}
		assert.Panic(t, func() {
			Bind(fn)
		}, "invalid bind function signature: expected func\\(.*\\) error or func\\(.*\\) \\(T, error\\)")
	})

	t.Run("function with too many return values", func(t *testing.T) {
		fn := func(a int, b string) ([]string, map[string]int, error) {
			return []string{b}, map[string]int{"value": a}, nil
		}
		args := []gs.Arg{
			Value(1),
			Value("test"),
		}
		assert.Panic(t, func() {
			Bind(fn, args...)
		}, "invalid bind function signature: expected func\\(.*\\) error or func\\(.*\\) \\(T, error\\)")
	})

	t.Run("error in argument processing", func(t *testing.T) {
		fn := func(a int, b string) string {
			return fmt.Sprintf("%d-%s", a, b)
		}
		args := []gs.Arg{
			Value(1),
			Index(1, Value("test")),
		}
		assert.Panic(t, func() {
			Bind(fn, args...)
		}, "arguments must be all indexed or non-indexed")
	})

	t.Run("success with returning single value", func(t *testing.T) {
		fn := func(a int, b string) string {
			return fmt.Sprintf("%d-%s", a, b)
		}
		args := []gs.Arg{
			Value(1),
			Value("test"),
		}
		arg := Bind(fn, args...)
		assert.String(t, arg.fileline).Matches("gs/internal/gs_arg/arg_test.go:.*")
	})

	t.Run("success with returning value and error", func(t *testing.T) {
		fn := func(a int, b string) (string, error) {
			return fmt.Sprintf("%d-%s", a, b), nil
		}
		args := []gs.Arg{
			Value(1),
			Value("test"),
		}
		arg := Bind(fn, args...)
		assert.String(t, arg.fileline).Matches("gs/internal/gs_arg/arg_test.go:.*")
	})
}

func TestBindArg_GetArgValue(t *testing.T) {

	t.Run("error in get arg value", func(t *testing.T) {
		fn := func(a int, b string) string {
			return fmt.Sprintf("%d-%s", a, b)
		}
		args := []gs.Arg{
			Value(1),
			Value(2),
		}
		arg := Bind(fn, args...)
		ctx := gs.NewArgContextMockImpl(nil)
		_, err := arg.GetArgValue(ctx, reflect.TypeFor[string]())
		assert.Error(t, err).Matches("cannot assign type int to type string")
	})

	t.Run("function returning slice", func(t *testing.T) {
		fn := func(prefix string, items ...int) []string {
			result := make([]string, len(items))
			for i, item := range items {
				result[i] = fmt.Sprintf("%s%d", prefix, item)
			}
			return result
		}
		args := []gs.Arg{
			Value("item"),
			Value(1),
			Value(2),
			Value(3),
		}
		arg := Bind(fn, args...)
		ctx := gs.NewArgContextMockImpl(nil)

		v, err := arg.GetArgValue(ctx, reflect.TypeFor[[]string]())
		assert.That(t, err).Nil()
		result := v.Interface().([]string)
		expected := []string{"item1", "item2", "item3"}
		assert.That(t, result).Equal(expected)
	})

	t.Run("success", func(t *testing.T) {
		fn := func(a int, b string) string {
			return fmt.Sprintf("%d-%s", a, b)
		}
		args := []gs.Arg{
			Value(1),
			Value("test"),
		}
		arg := Bind(fn, args...)
		ctx := gs.NewArgContextMockImpl(nil)
		v, err := arg.GetArgValue(ctx, reflect.TypeFor[string]())
		assert.That(t, err).Nil()
		assert.That(t, "1-test").Equal(v.Interface().(string))
	})

	t.Run("return value type mismatch", func(t *testing.T) {
		fn := func() string {
			return "test"
		}
		arg := Bind(fn)
		ctx := gs.NewArgContextMockImpl(nil)
		_, err := arg.GetArgValue(ctx, reflect.TypeFor[int]())
		assert.Error(t, err).Matches("cannot assign type string to type int")
	})

	t.Run("error in function execution", func(t *testing.T) {
		fn := func(a int, b string) (string, error) {
			return "", errutil.Explain(nil, "execution error")
		}
		args := []gs.Arg{
			Value(1),
			Value("test"),
		}
		arg := Bind(fn, args...)
		ctx := gs.NewArgContextMockImpl(nil)
		_, err := arg.GetArgValue(ctx, reflect.TypeFor[string]())
		assert.Error(t, err).Matches("execution error")
	})

	t.Run("no error in function execution", func(t *testing.T) {
		fn := func(a int, b string) (string, error) {
			return fmt.Sprintf("%d-%s", a, b), nil
		}
		args := []gs.Arg{
			Value(1),
			Value("test"),
		}
		arg := Bind(fn, args...)
		ctx := gs.NewArgContextMockImpl(nil)
		v, err := arg.GetArgValue(ctx, reflect.TypeFor[string]())
		assert.That(t, err).Nil()
		assert.That(t, "1-test").Equal(v.Interface().(string))
	})

	t.Run("success with variadic function", func(t *testing.T) {
		fn := func(a int, b ...string) string {
			return fmt.Sprintf("%d-%s", a, strings.Join(b, ","))
		}
		args := []gs.Arg{
			Value(1),
			Value("test1"),
			Value("test2"),
		}
		arg := Bind(fn, args...)
		ctx := gs.NewArgContextMockImpl(nil)
		v, err := arg.GetArgValue(ctx, reflect.TypeFor[string]())
		assert.That(t, err).Nil()
		assert.That(t, "1-test1,test2").Equal(v.Interface().(string))
	})

	t.Run("error in condition", func(t *testing.T) {
		fn := func(a int, b string) string {
			return fmt.Sprintf("%d-%s", a, b)
		}
		args := []gs.Arg{
			Value(1),
			Value("test"),
		}
		arg := Bind(fn, args...)
		arg.Condition(gs_cond.OnFunc(func(ctx gs.ConditionContext) (bool, error) {
			return false, errutil.Explain(nil, "condition error")
		}))

		m := gsmock.NewManager()
		c := gs.NewArgContextMockImpl(m)
		c.MockCheck().Handle(func(c gs.Condition) (bool, error) {
			ok, err := c.Matches(nil)
			return ok, err
		})

		_, err := arg.GetArgValue(c, reflect.TypeFor[string]())
		assert.Error(t, err).Matches("condition error")
	})

	t.Run("condition return false", func(t *testing.T) {
		fn := func(a int, b string) string {
			return fmt.Sprintf("%d-%s", a, b)
		}
		args := []gs.Arg{
			Value(1),
			Value("test"),
		}
		arg := Bind(fn, args...)
		arg.Condition(gs_cond.OnFunc(func(ctx gs.ConditionContext) (bool, error) {
			return false, nil
		}))

		m := gsmock.NewManager()
		c := gs.NewArgContextMockImpl(m)
		c.MockCheck().Handle(func(c gs.Condition) (bool, error) {
			ok, err := c.Matches(nil)
			return ok, err
		})

		v, err := arg.GetArgValue(c, reflect.TypeFor[string]())
		assert.That(t, err).Nil()
		assert.That(t, v.IsValid()).False()
	})

	t.Run("condition return true", func(t *testing.T) {
		fn := func(a int, b string) string {
			return fmt.Sprintf("%d-%s", a, b)
		}
		args := []gs.Arg{
			Value(1),
			Value("test"),
		}
		arg := Bind(fn, args...)
		arg.Condition(gs_cond.OnFunc(func(ctx gs.ConditionContext) (bool, error) {
			return true, nil
		}))

		m := gsmock.NewManager()
		c := gs.NewArgContextMockImpl(m)
		c.MockCheck().Handle(func(c gs.Condition) (bool, error) {
			ok, err := c.Matches(nil)
			return ok, err
		})

		v, err := arg.GetArgValue(c, reflect.TypeFor[string]())
		assert.That(t, err).Nil()
		assert.That(t, "1-test").Equal(v.Interface().(string))
	})
}

func TestBindArg_Condition(t *testing.T) {

	t.Run("nil condition", func(t *testing.T) {
		arg := Bind(func() string {
			return "ok"
		})
		assert.Panic(t, func() {
			arg.Condition(nil)
		}, "conditions cannot contains nil")
	})

	t.Run("multiple conditions - all true", func(t *testing.T) {
		fn := func(a int, b string) string {
			return fmt.Sprintf("%d-%s", a, b)
		}
		args := []gs.Arg{
			Value(1),
			Value("test"),
		}
		arg := Bind(fn, args...)

		arg.Condition(gs_cond.OnFunc(func(ctx gs.ConditionContext) (bool, error) {
			return true, nil
		}))
		arg.Condition(gs_cond.OnFunc(func(ctx gs.ConditionContext) (bool, error) {
			return true, nil
		}))

		m := gsmock.NewManager()
		c := gs.NewArgContextMockImpl(m)
		c.MockCheck().Handle(func(c gs.Condition) (bool, error) {
			ok, err := c.Matches(nil)
			return ok, err
		})

		v, err := arg.GetArgValue(c, reflect.TypeFor[string]())
		assert.That(t, err).Nil()
		assert.That(t, "1-test").Equal(v.Interface().(string))
	})

	t.Run("multiple conditions - one false", func(t *testing.T) {
		fn := func(a int, b string) string {
			return fmt.Sprintf("%d-%s", a, b)
		}
		args := []gs.Arg{
			Value(1),
			Value("test"),
		}
		arg := Bind(fn, args...)

		arg.Condition(gs_cond.OnFunc(func(ctx gs.ConditionContext) (bool, error) {
			return false, nil
		}))
		arg.Condition(gs_cond.OnFunc(func(ctx gs.ConditionContext) (bool, error) {
			return true, nil
		}))

		m := gsmock.NewManager()
		c := gs.NewArgContextMockImpl(m)
		c.MockCheck().Handle(func(c gs.Condition) (bool, error) {
			ok, err := c.Matches(nil)
			return ok, err
		})

		v, err := arg.GetArgValue(c, reflect.TypeFor[string]())
		assert.That(t, err).Nil()
		assert.That(t, v.IsValid()).False()
	})
}
