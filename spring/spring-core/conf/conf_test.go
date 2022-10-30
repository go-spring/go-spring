/*
 * Copyright 2012-2019 the original author or authors.
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

package conf_test

import (
	"io"
	"syscall"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-core/conf"
)

func TestRegisterConverter(t *testing.T) {
	assert.Panic(t, func() {
		conf.RegisterConverter(func() {})
	}, "converter is func\\(string\\)\\(type,error\\)")
}

func TestLoad(t *testing.T) {

	_, err := conf.Load("nonexisting.yaml")
	assert.Error(t, err, "no such file or directory")

	_, err = conf.Load("testdata/application.xyz")
	assert.Error(t, err, "unsupported file type \\.xyz")

	_, err = conf.Load("testdata/error.yaml")
	assert.Error(t, err, "cannot unmarshal \\!!str `a:=b` into map\\[string]interface \\{}")

	_, err = conf.Load("testdata/application.yaml")
	assert.Nil(t, err)
}

func TestRead(t *testing.T) {

	_, err := conf.Read(util.FuncReader(func(p []byte) (n int, err error) {
		return 0, syscall.ENOENT
	}), ".yaml")
	assert.Error(t, err, "no such file or directory")

	_, err = conf.Read(util.FuncReader(func(p []byte) (n int, err error) {
		return copy(p, "a=b"), io.EOF
	}), ".xyz")
	assert.Error(t, err, "unsupported file type \\.xyz")

	_, err = conf.Read(util.FuncReader(func(p []byte) (n int, err error) {
		return copy(p, "a:=b"), io.EOF
	}), ".yaml")
	assert.Error(t, err, "cannot unmarshal \\!!str `a:=b` into map\\[string]interface \\{}")

	_, err = conf.Read(util.FuncReader(func(p []byte) (n int, err error) {
		return copy(p, "a=b"), io.EOF
	}), ".properties")
	assert.Nil(t, err)
}

func TestBytes(t *testing.T) {

	_, err := conf.Bytes([]byte("a=b"), ".xyz")
	assert.Error(t, err, "unsupported file type \\.xyz")

	_, err = conf.Bytes([]byte("a:=b"), ".yaml")
	assert.Error(t, err, "cannot unmarshal \\!!str `a:=b` into map\\[string]interface \\{}")

	_, err = conf.Bytes([]byte("a=b"), ".properties")
	assert.Nil(t, err)
}

func TestProperties(t *testing.T) {
	p := conf.Map(map[string]interface{}{
		"int":   1,
		"ints":  "1,2,3",
		"uint":  "1",
		"uints": []uint{1, 2, 3},
		"array": []int{1, 2, 3},
		"map": map[string]string{
			"1": "abc",
		},
	})
	err := p.Set("", "123")
	assert.Nil(t, err)
	assert.Equal(t, p.Copy(), p)
	assert.Equal(t, p.Keys(), []string{
		"array[0]",
		"array[1]",
		"array[2]",
		"int",
		"ints",
		"map.1",
		"uint",
		"uints[0]",
		"uints[1]",
		"uints[2]",
	})
	assert.Equal(t, p.Get("int"), "1")
	assert.Equal(t, p.Get("float", conf.Def("3.0")), "3.0")
}

func TestProperties_Merge(t *testing.T) {
	p := conf.Map(map[string]interface{}{
		"a": []int{1, 2, 3},
	})
	err := p.Merge(map[string]interface{}{
		"b": map[string]string{
			"c": "123",
		},
	})
	assert.Nil(t, err)
	err = p.Merge(map[string]interface{}{
		"a": map[string]string{
			"c": "123",
		},
	})
	assert.Error(t, err, "property 'a' is an array but 'a\\.c' wants other type")
}

////func TestProperties_Load(t *testing.T) {
////
////	p := conf.New()
////	err := p.Load("testdata/config/application.yaml")
////	assert.Nil(t, err)
////	err = p.Load("testdata/config/application.properties")
////	assert.Nil(t, err)
////
////	for _, k := range p.Keys() {
////		fmt.Println(k+":", p.Get(k))
////	}
////
////	assert.True(t, p.Has("yaml.list"))
////	assert.True(t, p.Has("properties.list"))
////}
//
//func TestProperties_Get(t *testing.T) {
//
//	t.Run("base", func(t *testing.T) {
//
//		p := conf.New()
//
//		err := p.Set("a.b.c", "3")
//		assert.Nil(t, err)
//		err = p.Set("a.b.d", []string{"3"})
//		assert.Nil(t, err)
//
//		v := p.Get("a.b.c")
//		assert.Equal(t, v, "3")
//		v = p.Get("a.b.d[0]")
//		assert.Equal(t, v, "3")
//
//		err = p.Set("Bool", true)
//		assert.Nil(t, err)
//		err = p.Set("Int", 3)
//		assert.Nil(t, err)
//		err = p.Set("Uint", 3)
//		assert.Nil(t, err)
//		err = p.Set("Float", 3.0)
//		assert.Nil(t, err)
//		err = p.Set("String", "3")
//		assert.Nil(t, err)
//		err = p.Set("Duration", "3s")
//		assert.Nil(t, err)
//		err = p.Set("StringSlice", []string{"3", "4"})
//		assert.Nil(t, err)
//		err = p.Set("Time", "2020-02-04 20:02:04 >> 2006-01-02 15:04:05")
//		assert.Nil(t, err)
//		err = p.Set("MapStringInterface", []interface{}{
//			map[interface{}]interface{}{
//				"1": 2,
//			},
//		})
//		assert.Nil(t, err)
//
//		assert.False(t, p.Has("NULL"))
//		assert.Equal(t, p.Get("NULL"), "")
//
//		v = p.Get("NULL", conf.Def("OK"))
//		assert.Equal(t, v, "OK")
//
//		v = p.Get("Int")
//		assert.Equal(t, v, "3")
//
//		var v2 int
//		err = p.Bind(&v2, conf.Key("Int"))
//		assert.Nil(t, err)
//		assert.Equal(t, v2, 3)
//
//		var u2 uint
//		err = p.Bind(&u2, conf.Key("Uint"))
//		assert.Nil(t, err)
//		assert.Equal(t, u2, uint(3))
//
//		var u3 uint32
//		err = p.Bind(&u3, conf.Key("Uint"))
//		assert.Nil(t, err)
//		assert.Equal(t, u3, uint32(3))
//
//		var f2 float32
//		err = p.Bind(&f2, conf.Key("Float"))
//		assert.Nil(t, err)
//		assert.Equal(t, f2, float32(3))
//
//		v = p.Get("Bool")
//		b := cast.ToBool(v)
//		assert.Equal(t, b, true)
//
//		var b2 bool
//		err = p.Bind(&b2, conf.Key("Bool"))
//		assert.Nil(t, err)
//		assert.Equal(t, b2, true)
//
//		v = p.Get("Int")
//		i := cast.ToInt64(v)
//		assert.Equal(t, i, int64(3))
//
//		v = p.Get("Uint")
//		u := cast.ToUint64(v)
//		assert.Equal(t, u, uint64(3))
//
//		v = p.Get("Float")
//		f := cast.ToFloat64(v)
//		assert.Equal(t, f, 3.0)
//
//		v = p.Get("String")
//		s := cast.ToString(v)
//		assert.Equal(t, s, "3")
//
//		assert.False(t, p.Has("string"))
//		assert.Equal(t, p.Get("string"), "")
//
//		v = p.Get("Duration")
//		d := cast.ToDuration(v)
//		assert.Equal(t, d, time.Second*3)
//
//		var ti time.Time
//		err = p.Bind(&ti, conf.Key("Time"))
//		assert.Nil(t, err)
//		assert.Equal(t, ti, time.Date(2020, 02, 04, 20, 02, 04, 0, time.UTC))
//
//		err = p.Bind(&ti, conf.Key("Duration"))
//		assert.Nil(t, err)
//		assert.Equal(t, ti, time.Date(1970, 01, 01, 00, 00, 03, 0, time.UTC).Local())
//
//		var ss2 []string
//		err = p.Bind(&ss2, conf.Key("StringSlice"))
//		assert.Nil(t, err)
//		assert.Equal(t, ss2, []string{"3", "4"})
//	})
//
//	t.Run("slice slice", func(t *testing.T) {
//		p := conf.Map(map[string]interface{}{
//			"a": []interface{}{
//				[]interface{}{1, 2},
//				[]interface{}{3, 4},
//				map[string]interface{}{
//					"b": "c",
//					"d": []interface{}{5, 6},
//				},
//			},
//		})
//		v := p.Get("a[0][0]")
//		assert.Equal(t, v, "1")
//		v = p.Get("a[0][1]")
//		assert.Equal(t, v, "2")
//		v = p.Get("a[1][0]")
//		assert.Equal(t, v, "3")
//		v = p.Get("a[1][1]")
//		assert.Equal(t, v, "4")
//		v = p.Get("a[2].b")
//		assert.Equal(t, v, "c")
//		v = p.Get("a[2].d[0]")
//		assert.Equal(t, v, "5")
//		v = p.Get("a[2].d[1]")
//		assert.Equal(t, v, "6")
//	})
//}
//
//func TestProperties_Ref(t *testing.T) {
//
//	type fileLog struct {
//		Dir             string `value:"${dir:=${app.dir}}"`
//		NestedDir       string `value:"${nested.dir:=${nested.app.dir:=./log}}"`
//		NestedEmptyDir  string `value:"${nested.dir:=${nested.app.dir:=}}"`
//		NestedNestedDir string `value:"${nested.dir:=${nested.app.dir:=${nested.nested.app.dir:=./log}}}"`
//	}
//
//	var mqLog struct{ fileLog }
//	var httpLog struct{ fileLog }
//
//	t.Run("not config", func(t *testing.T) {
//		p := conf.New()
//		err := p.Bind(&httpLog)
//		assert.Error(t, err, "property \\\"app.dir\\\" not exist")
//	})
//
//	t.Run("config", func(t *testing.T) {
//		p := conf.New()
//
//		appDir := "/home/log"
//		err := p.Set("app.dir", appDir)
//		assert.Nil(t, err)
//
//		err = p.Bind(&httpLog)
//		assert.Nil(t, err)
//		assert.Equal(t, httpLog.Dir, appDir)
//		assert.Equal(t, httpLog.NestedDir, "./log")
//		assert.Equal(t, httpLog.NestedEmptyDir, "")
//		assert.Equal(t, httpLog.NestedNestedDir, "./log")
//
//		err = p.Bind(&mqLog)
//		assert.Nil(t, err)
//		assert.Equal(t, mqLog.Dir, appDir)
//		assert.Equal(t, mqLog.NestedDir, "./log")
//		assert.Equal(t, mqLog.NestedEmptyDir, "")
//		assert.Equal(t, mqLog.NestedNestedDir, "./log")
//	})
//
//	t.Run("empty key", func(t *testing.T) {
//		p := conf.New()
//		var s struct {
//			KeyIsEmpty string `value:"${:=kie}"`
//		}
//		err := p.Bind(&s)
//		assert.Nil(t, err)
//		assert.Equal(t, s.KeyIsEmpty, "kie")
//	})
//}

func TestResolve(t *testing.T) {
	p := conf.New()

	_, err := p.Resolve("my name is ${name}")
	assert.Error(t, err, "resolve property \"name\" error; property \"name\" not exist")

	_ = p.Set("name", "Jim")
	_, err = p.Resolve("my name is ${name")
	assert.Error(t, err, "resolve string \"my name is \\${name\" error; invalid syntax")

	_, err = p.Resolve("my name is ${name} my name is ${name")
	assert.Error(t, err, "resolve string \" my name is \\${name\" error; invalid syntax")

	str, err := p.Resolve("my name is ${name}")
	assert.Nil(t, err)
	assert.Equal(t, str, "my name is Jim")

	str, err = p.Resolve("my name is ${name}${name}")
	assert.Nil(t, err)
	assert.Equal(t, str, "my name is JimJim")

	str, err = p.Resolve("my name is ${name} my name is ${name}")
	assert.Nil(t, err)
	assert.Equal(t, str, "my name is Jim my name is Jim")
}

//func TestProperties_Has(t *testing.T) {
//	p := conf.Map(map[string]interface{}{
//		"a.b.c": "3",
//		"a.b.d": []string{"7", "8"},
//	})
//	assert.True(t, p.Has("a"))
//	assert.False(t, p.Has("a[0]"))
//	assert.True(t, p.Has("a.b"))
//	assert.False(t, p.Has("a.c"))
//	assert.True(t, p.Has("a.b.c"))
//	assert.True(t, p.Has("a.b.d"))
//	assert.True(t, p.Has("a.b.d[0]"))
//	assert.True(t, p.Has("a.b.d[1]"))
//	assert.False(t, p.Has("a.b.d[2]"))
//	assert.False(t, p.Has("a.b.e"))
//}
//
//func TestProperties_Set(t *testing.T) {
//
//	t.Run("map nil", func(t *testing.T) {
//
//		p := conf.New()
//		err := p.Set("m", nil)
//		assert.Nil(t, err)
//		assert.True(t, p.Has("m"))
//		assert.Equal(t, p.Get("m"), "")
//
//		err = p.Set("m", map[string]string{"a": "b"})
//		assert.Nil(t, err)
//		assert.Equal(t, p.Keys(), []string{"m.a"})
//
//		err = p.Set("m", 1)
//		assert.Error(t, err, "property 'm' is a map but 'm' wants other type")
//
//		err = p.Set("m", []string{"b"})
//		assert.Error(t, err, "property 'm' is a map but 'm\\[0]' wants other type")
//	})
//
//	t.Run("map empty", func(t *testing.T) {
//		p := conf.New()
//		err := p.Set("m", map[string]string{})
//		assert.Nil(t, err)
//		assert.True(t, p.Has("m"))
//		assert.Equal(t, p.Get("m"), "")
//		err = p.Set("m", map[string]string{"a": "b"})
//		assert.Nil(t, err)
//		assert.Equal(t, p.Keys(), []string{"m.a"})
//	})
//
//	t.Run("list nil", func(t *testing.T) {
//		p := conf.New()
//		err := p.Set("a", nil)
//		assert.Nil(t, err)
//		assert.True(t, p.Has("a"))
//		assert.Equal(t, p.Get("a"), "")
//		err = p.Set("a", []string{"b"})
//		assert.Nil(t, err)
//		assert.Equal(t, p.Keys(), []string{"a[0]"})
//	})
//
//	t.Run("list empty", func(t *testing.T) {
//		p := conf.New()
//		err := p.Set("a", []string{})
//		assert.Nil(t, err)
//		assert.True(t, p.Has("a"))
//		assert.Equal(t, p.Get("a"), "")
//		err = p.Set("a", []string{"b"})
//		assert.Nil(t, err)
//		assert.Equal(t, p.Keys(), []string{"a[0]"})
//	})
//
//	t.Run("list", func(t *testing.T) {
//		p := conf.New()
//		err := p.Set("a", []string{"a", "aa", "aaa"})
//		assert.Nil(t, err)
//		err = p.Set("b", []int{1, 11, 111})
//		assert.Nil(t, err)
//		err = p.Set("c", []float32{1, 1.1, 1.11})
//		assert.Nil(t, err)
//		assert.Equal(t, p.Get("a[0]"), "a")
//		assert.Equal(t, p.Get("a[1]"), "aa")
//		assert.Equal(t, p.Get("a[2]"), "aaa")
//		assert.Equal(t, p.Get("b[0]"), "1")
//		assert.Equal(t, p.Get("b[1]"), "11")
//		assert.Equal(t, p.Get("b[2]"), "111")
//		assert.Equal(t, p.Get("c[0]"), "1")
//		assert.Equal(t, p.Get("c[1]"), "1.1")
//		assert.Equal(t, p.Get("c[2]"), "1.11")
//	})
//}
//

//
//func PointSplitter(str string) ([]string, error) {
//	var ret []string
//	var lastIndex int
//	for i, c := range str {
//		if c == ')' {
//			ret = append(ret, str[lastIndex:i+1])
//			lastIndex = i + 1
//		}
//	}
//	return ret, nil
//}
//
//func TestSplitter(t *testing.T) {
//	conf.RegisterConverter(PointConverter)
//	conf.RegisterSplitter("PointSplitter", PointSplitter)
//	var points []image.Point
//	err := conf.New().Bind(&points, conf.Tag("${:=(1,2)(3,4)}||PointSplitter"))
//	assert.Nil(t, err)
//	assert.Equal(t, points, []image.Point{{X: 1, Y: 2}, {X: 3, Y: 4}})
//}
