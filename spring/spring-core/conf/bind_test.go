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
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/conf"
)

type Point struct {
	X, Y int
}

func init() {
	conf.RegisterConverter(func(val string) (Point, error) {
		if !(strings.HasPrefix(val, "(") && strings.HasSuffix(val, ")")) {
			return Point{}, errors.New("illegal format")
		}
		ss := strings.Split(val[1:len(val)-1], ",")
		x, err := strconv.Atoi(ss[0])
		if err != nil {
			return Point{}, err
		}
		y, err := strconv.Atoi(ss[1])
		if err != nil {
			return Point{}, err
		}
		return Point{X: x, Y: y}, nil
	})
}

func TestParseTag(t *testing.T) {
	var testcases = []struct {
		Tag   string
		Error string
		Data  string
	}{
		{
			Tag:   "||",
			Error: `parse tag '\|\|' error: invalid syntax`,
		},
		{
			Tag:   "a||",
			Error: `parse tag 'a\|\|' error: invalid syntax`,
		},
		{
			Tag:   "a}||",
			Error: `parse tag 'a}\|\|' error: invalid syntax`,
		},
		{
			Tag:  "${}||",
			Data: "${}",
		},
		{
			Tag:  "${}||k",
			Data: "${}||k",
		},
		{
			Tag:  "${a}||",
			Data: "${a}",
		},
		{
			Tag:  "${a}||k",
			Data: "${a}||k",
		},
		{
			Tag:  "${a:=}||",
			Data: "${a:=}",
		},
		{
			Tag:  "${a:=}||k",
			Data: "${a:=}||k",
		},
		{
			Tag:  "${a:=b}||",
			Data: "${a:=b}",
		},
		{
			Tag:  "${a:=b}||k",
			Data: "${a:=b}||k",
		},
	}
	for _, c := range testcases {
		tag, err := conf.ParseTag(c.Tag)
		if c.Error != "" {
			assert.Error(t, err, c.Error)
			continue
		}
		assert.Nil(t, err)
		assert.Equal(t, tag.String(), c.Data)
		s, err := conf.ParseTag(tag.String())
		assert.Nil(t, err)
		assert.Equal(t, s, tag)
	}
}

func TestBindParam_BindTag(t *testing.T) {

	param := conf.BindParam{}
	err := param.BindTag("{}", "")
	assert.Error(t, err, "parse tag '\\{\\}' error: invalid syntax")

	param = conf.BindParam{}
	err = param.BindTag("${}", "")
	assert.Nil(t, err)
	assert.Equal(t, param, conf.BindParam{
		Key: "ANONYMOUS",
		Tag: conf.ParsedTag{
			Key: "ANONYMOUS",
		},
	})

	param = conf.BindParam{
		Key: "s",
	}
	err = param.BindTag("${}", "")
	assert.Nil(t, err)
	assert.Equal(t, param, conf.BindParam{
		Key: "s.ANONYMOUS",
		Tag: conf.ParsedTag{
			Key: "ANONYMOUS",
		},
	})

	param = conf.BindParam{}
	err = param.BindTag("${ROOT}", "")
	assert.Nil(t, err)
	assert.Equal(t, param, conf.BindParam{})

	param = conf.BindParam{}
	err = param.BindTag("${a:=b}", "")
	assert.Nil(t, err)
	assert.Equal(t, param, conf.BindParam{
		Key: "a",
		Tag: conf.ParsedTag{
			Key:    "a",
			Def:    "b",
			HasDef: true,
		},
	})

	param = conf.BindParam{
		Key: "s",
	}
	err = param.BindTag("${a:=b}", "")
	assert.Nil(t, err)
	assert.Equal(t, param, conf.BindParam{
		Key: "s.a",
		Tag: conf.ParsedTag{
			Key:    "a",
			Def:    "b",
			HasDef: true,
		},
	})
}

type PtrStruct struct {
	Int int `value:"${int}"`
}

type CommonStruct struct {
	Int      int           `value:"${int}"`
	Ints     []int         `value:"${ints}"`
	Uint     uint          `value:"${uint:=3}"`
	Uints    []uint        `value:"${uints:=1,2,3}"`
	Float    float64       `value:"${float:=3}"`
	Floats   []float64     `value:"${floats:=1,2,3}"`
	Bool     bool          `value:"${bool:=true}"`
	Bools    []bool        `value:"${bools:=true,false}"`
	String   string        `value:"${string:=abc}"`
	Strings  []string      `value:"${strings:=abc,def,ghi}"`
	Time     time.Time     `value:"${time:=2017-06-17 13:20:15 UTC}"`
	Duration time.Duration `value:"${duration:=5s}"`
}

type NestedStruct struct {
	*PtrStruct
	CommonStruct
	Struct   CommonStruct
	Nested   CommonStruct  `value:"${nested}"`
	Int      int           `value:"${int}"`
	Ints     []int         `value:"${ints}"`
	Uint     uint          `value:"${uint:=3}"`
	Uints    []uint        `value:"${uints:=1,2,3}"`
	Float    float64       `value:"${float:=3}"`
	Floats   []float64     `value:"${floats:=1,2,3}"`
	Bool     bool          `value:"${bool:=true}"`
	Bools    []bool        `value:"${bools:=true,false}"`
	String   string        `value:"${string:=abc}"`
	Strings  []string      `value:"${strings:=abc,def,ghi}"`
	Time     time.Time     `value:"${time:=2017-06-17 13:20:15 UTC}"`
	Duration time.Duration `value:"${duration:=5s}"`
}

func TestBind_InvalidValue(t *testing.T) {

	t.Run("invalid", func(t *testing.T) {
		var f float64
		err := conf.Map(nil).Bind(&f, conf.Tag("a:=b"))
		assert.Error(t, err, "parse tag 'a:=b' error: invalid syntax")
	})

	t.Run("int", func(t *testing.T) {
		var i int
		err := conf.Map(nil).Bind(i)
		assert.Error(t, err, "i should be a ptr")
	})

	t.Run("chan", func(t *testing.T) {
		c := make(chan int)
		key := conf.Key("chan")
		err := conf.Map(nil).Bind(&c, key)
		assert.Error(t, err, ".*:124 bind chan int error; target should be value type")
	})

	t.Run("array", func(t *testing.T) {
		var s [3]int
		key := conf.Key("array")
		err := conf.Map(nil).Bind(&s, key)
		assert.Error(t, err, ".*:134 bind \\[3\\]int error; use slice instead of array")
	})

	t.Run("complex", func(t *testing.T) {
		var c complex64
		tag := conf.Tag("${complex:=i+3}")
		err := conf.Map(nil).Bind(&c, tag)
		assert.Error(t, err, "bind complex64 error; unsupported bind type \"complex64\"")
	})

	t.Run("pointer", func(t *testing.T) {
		var s struct {
			PtrStruct *PtrStruct `value:"${ptr}"`
		}
		err := conf.Map(nil).Bind(&s)
		assert.Error(t, err, "bind .* error; target should be value type")
	})
}

func TestBind_SingleValue(t *testing.T) {

	t.Run("uint", func(t *testing.T) {
		var u uint

		key := conf.Key("uint")
		err := conf.Map(nil).Bind(&u, key)
		assert.Error(t, err, "resolve property \\\"uint\\\" error; property \\\"uint\\\" not exist")

		tag := conf.Tag("${uint:=3}")
		err = conf.Map(nil).Bind(&u, tag)
		assert.Nil(t, err)
		assert.Equal(t, u, uint(3))

		err = conf.Map(map[string]interface{}{
			"uint": 5,
		}).Bind(&u, tag)
		assert.Nil(t, err)
		assert.Equal(t, u, uint(5))

		err = conf.Map(map[string]interface{}{
			"uint": "abc",
		}).Bind(&u, tag)
		assert.Error(t, err, "bind uint error; strconv.ParseUint: parsing \\\"abc\\\": invalid syntax")
	})

	t.Run("int", func(t *testing.T) {
		var i int

		key := conf.Key("int")
		err := conf.Map(nil).Bind(&i, key)
		assert.Error(t, err, "resolve property \\\"int\\\" error; property \\\"int\\\" not exist")

		tag := conf.Tag("${int:=3}")
		err = conf.Map(nil).Bind(&i, tag)
		assert.Nil(t, err)
		assert.Equal(t, i, 3)

		err = conf.Map(map[string]interface{}{
			"int": 5,
		}).Bind(&i, tag)
		assert.Nil(t, err)
		assert.Equal(t, i, 5)

		err = conf.Map(map[string]interface{}{
			"int": "abc",
		}).Bind(&i, tag)
		assert.Error(t, err, "bind int error; strconv.ParseInt: parsing \\\"abc\\\": invalid syntax")
	})

	t.Run("float", func(t *testing.T) {
		var f float32

		key := conf.Key("float")
		err := conf.Map(nil).Bind(&f, key)
		assert.Error(t, err, "resolve property \\\"float\\\" error; property \\\"float\\\" not exist")

		tag := conf.Tag("${float:=3}")
		err = conf.Map(nil).Bind(&f, tag)
		assert.Nil(t, err)
		assert.Equal(t, f, float32(3))

		err = conf.Map(map[string]interface{}{
			"float": 5,
		}).Bind(&f, tag)
		assert.Nil(t, err)
		assert.Equal(t, f, float32(5))

		err = conf.Map(map[string]interface{}{
			"float": "abc",
		}).Bind(&f, tag)
		assert.Error(t, err, "bind float32 error; strconv.ParseFloat: parsing \\\"abc\\\": invalid syntax")
	})

	t.Run("bool", func(t *testing.T) {
		var b bool

		key := conf.Key("bool")
		err := conf.Map(nil).Bind(&b, key)
		assert.Error(t, err, "resolve property \\\"bool\\\" error; property \\\"bool\\\" not exist")

		tag := conf.Tag("${bool:=false}")
		err = conf.Map(nil).Bind(&b, tag)
		assert.Nil(t, err)
		assert.Equal(t, b, false)

		err = conf.Map(map[string]interface{}{
			"bool": true,
		}).Bind(&b, tag)
		assert.Nil(t, err)
		assert.Equal(t, b, true)

		err = conf.Map(map[string]interface{}{
			"bool": "abc",
		}).Bind(&b, tag)
		assert.Error(t, err, "bind bool error; strconv.ParseBool: parsing \\\"abc\\\": invalid syntax")
	})

	t.Run("string", func(t *testing.T) {
		var s string

		tag := conf.Tag("${string:=abc}")
		err := conf.Map(nil).Bind(&s, tag)
		assert.Nil(t, err)
		assert.Equal(t, s, "abc")

		err = conf.Map(map[string]interface{}{
			"string": "def",
		}).Bind(&s, tag)
		assert.Nil(t, err)
		assert.Equal(t, s, "def")
	})

	t.Run("struct", func(t *testing.T) {
		var s NestedStruct

		tag := conf.Tag("${struct:=abc,123}")
		err := conf.Map(nil).Bind(&s, tag)
		assert.Error(t, err, "bind .* error; struct can't have a non empty default value")

		tag = conf.Tag("${struct:=}")
		err = conf.Map(nil).Bind(&s, tag)
		assert.Error(t, err, "resolve property \"struct\\.int\" error; property \"struct\\.int\" not exist")

		tag = conf.Tag("${struct:=}")
		err = conf.Map(map[string]interface{}{
			"struct": map[string]interface{}{
				"int":  1,
				"ints": []int{1, 2, 3},
				"nested": map[string]interface{}{
					"int":  1,
					"ints": "1,2,3",
				},
			},
		}).Bind(&s, tag)
		assert.Error(t, err, "resolve property .* error; property \\\"struct.\\Struct\\.int\\\" not exist")

		m := map[string]interface{}{
			"struct": map[string]interface{}{
				"int":  1,
				"ints": []int{1, 2, 3},
				"nested": map[string]interface{}{
					"int":  1,
					"ints": "1,2,3",
				},
				"Struct": map[string]interface{}{
					"int":  1,
					"ints": "1,2,3",
				},
			},
		}

		expect := NestedStruct{
			CommonStruct: CommonStruct{
				Int:      1,
				Ints:     []int{1, 2, 3},
				Uint:     uint(3),
				Uints:    []uint{1, 2, 3},
				Float:    float64(3),
				Floats:   []float64{1, 2, 3},
				Bool:     true,
				Bools:    []bool{true, false},
				String:   "abc",
				Strings:  []string{"abc", "def", "ghi"},
				Time:     time.Date(2017, 6, 17, 13, 20, 15, 0, time.UTC),
				Duration: 5 * time.Second,
			},
			Struct: CommonStruct{
				Int:      1,
				Ints:     []int{1, 2, 3},
				Uint:     uint(3),
				Uints:    []uint{1, 2, 3},
				Float:    float64(3),
				Floats:   []float64{1, 2, 3},
				Bool:     true,
				Bools:    []bool{true, false},
				String:   "abc",
				Strings:  []string{"abc", "def", "ghi"},
				Time:     time.Date(2017, 6, 17, 13, 20, 15, 0, time.UTC),
				Duration: 5 * time.Second,
			},
			Nested: CommonStruct{
				Int:      1,
				Ints:     []int{1, 2, 3},
				Uint:     uint(3),
				Uints:    []uint{1, 2, 3},
				Float:    float64(3),
				Floats:   []float64{1, 2, 3},
				Bool:     true,
				Bools:    []bool{true, false},
				String:   "abc",
				Strings:  []string{"abc", "def", "ghi"},
				Time:     time.Date(2017, 6, 17, 13, 20, 15, 0, time.UTC),
				Duration: 5 * time.Second,
			},
			Int:      1,
			Ints:     []int{1, 2, 3},
			Uint:     uint(3),
			Uints:    []uint{1, 2, 3},
			Float:    float64(3),
			Floats:   []float64{1, 2, 3},
			Bool:     true,
			Bools:    []bool{true, false},
			String:   "abc",
			Strings:  []string{"abc", "def", "ghi"},
			Time:     time.Date(2017, 6, 17, 13, 20, 15, 0, time.UTC),
			Duration: 5 * time.Second,
		}

		tag = conf.Tag("${struct:=}")
		err = conf.Map(m).Bind(&s, tag)
		assert.Nil(t, err)
		assert.Equal(t, s, expect)

		tag = conf.Tag("${struct}")
		err = conf.Map(m).Bind(&s, tag)
		assert.Nil(t, err)
		assert.Equal(t, s, expect)

		err = conf.Map(m["struct"].(map[string]interface{})).Bind(&s)
		assert.Nil(t, err)
		assert.Equal(t, s, expect)
	})
}

func TestBind_SliceValue(t *testing.T) {

	t.Run("uints", func(t *testing.T) {
		var u []uint

		key := conf.Key("uints")
		err := conf.Map(nil).Bind(&u, key)
		assert.Error(t, err, "bind \\[\\]uint error; .* property \\\"uints\\\"; not exist")

		tag := conf.Tag("${uints:=1,2,3}")
		err = conf.Map(nil).Bind(&u, tag)
		assert.Nil(t, err)
		assert.Equal(t, u, []uint{1, 2, 3})

		err = conf.Map(map[string]interface{}{
			"uints": "",
		}).Bind(&u, tag)
		assert.Nil(t, err)
		assert.Equal(t, u, []uint{})

		err = conf.Map(map[string]interface{}{
			"uints": 5,
		}).Bind(&u, tag)
		assert.Nil(t, err)
		assert.Equal(t, u, []uint{5})

		err = conf.Map(map[string]interface{}{
			"uints": []uint{5, 6, 7},
		}).Bind(&u, tag)
		assert.Nil(t, err)
		assert.Equal(t, u, []uint{5, 6, 7})

		err = conf.Map(map[string]interface{}{
			"uints": "5, 6, 7",
		}).Bind(&u, tag)
		assert.Nil(t, err)
		assert.Equal(t, u, []uint{5, 6, 7})

		err = conf.Map(map[string]interface{}{
			"uints": "abc",
		}).Bind(&u, tag)
		assert.Error(t, err, "bind \\[\\]uint\\[0\\] error; strconv.ParseUint: parsing \\\"abc\\\": invalid syntax")
	})

	t.Run("ints", func(t *testing.T) {
		var i []int

		key := conf.Key("ints")
		err := conf.Map(nil).Bind(&i, key)
		assert.Error(t, err, "bind \\[\\]int error; .* property \\\"ints\\\"; not exist")

		tag := conf.Tag("${ints:=1,2,3}")
		err = conf.Map(nil).Bind(&i, tag)
		assert.Nil(t, err)
		assert.Equal(t, i, []int{1, 2, 3})

		err = conf.Map(map[string]interface{}{
			"ints": "",
		}).Bind(&i, tag)
		assert.Nil(t, err)
		assert.Equal(t, i, []int{})

		err = conf.Map(map[string]interface{}{
			"ints": 5,
		}).Bind(&i, tag)
		assert.Nil(t, err)
		assert.Equal(t, i, []int{5})

		err = conf.Map(map[string]interface{}{
			"ints": []int{5, 6, 7},
		}).Bind(&i, tag)
		assert.Nil(t, err)
		assert.Equal(t, i, []int{5, 6, 7})

		err = conf.Map(map[string]interface{}{
			"ints": "5, 6, 7",
		}).Bind(&i, tag)
		assert.Nil(t, err)
		assert.Equal(t, i, []int{5, 6, 7})

		err = conf.Map(map[string]interface{}{
			"ints": "abc",
		}).Bind(&i, tag)
		assert.Error(t, err, "bind \\[\\]int\\[0\\] error; strconv.ParseInt: parsing \"abc\": invalid syntax")
	})

	t.Run("floats", func(t *testing.T) {
		var f []float32

		key := conf.Key("floats")
		err := conf.Map(nil).Bind(&f, key)
		assert.Error(t, err, "bind \\[\\]float32 error; .* property \\\"floats\\\"; not exist")

		tag := conf.Tag("${floats:=1,2,3}")
		err = conf.Map(nil).Bind(&f, tag)
		assert.Nil(t, err)
		assert.Equal(t, f, []float32{1, 2, 3})

		err = conf.Map(map[string]interface{}{
			"floats": "",
		}).Bind(&f, tag)
		assert.Nil(t, err)
		assert.Equal(t, f, []float32{})

		err = conf.Map(map[string]interface{}{
			"floats": 5,
		}).Bind(&f, tag)
		assert.Nil(t, err)
		assert.Equal(t, f, []float32{5})

		err = conf.Map(map[string]interface{}{
			"floats": []float32{5, 6, 7},
		}).Bind(&f, tag)
		assert.Nil(t, err)
		assert.Equal(t, f, []float32{5, 6, 7})

		err = conf.Map(map[string]interface{}{
			"floats": "5, 6, 7",
		}).Bind(&f, tag)
		assert.Nil(t, err)
		assert.Equal(t, f, []float32{5, 6, 7})

		err = conf.Map(map[string]interface{}{
			"floats": "abc",
		}).Bind(&f, tag)
		assert.Error(t, err, "bind \\[\\]float32\\[0\\] error; strconv.ParseFloat: parsing \\\"abc\\\": invalid syntax")
	})

	t.Run("bools", func(t *testing.T) {
		var b []bool

		key := conf.Key("bools")
		err := conf.Map(nil).Bind(&b, key)
		assert.Error(t, err, "bind \\[\\]bool error; .* property \\\"bools\\\"; not exist")

		tag := conf.Tag("${bools:=false,true,false}")
		err = conf.Map(nil).Bind(&b, tag)
		assert.Nil(t, err)
		assert.Equal(t, b, []bool{false, true, false})

		err = conf.Map(map[string]interface{}{
			"bools": "",
		}).Bind(&b, tag)
		assert.Nil(t, err)
		assert.Equal(t, b, []bool{})

		err = conf.Map(map[string]interface{}{
			"bools": true,
		}).Bind(&b, tag)
		assert.Nil(t, err)
		assert.Equal(t, b, []bool{true})

		err = conf.Map(map[string]interface{}{
			"bools": []bool{true, false, true},
		}).Bind(&b, tag)
		assert.Nil(t, err)
		assert.Equal(t, b, []bool{true, false, true})

		err = conf.Map(map[string]interface{}{
			"bools": "true, false, true",
		}).Bind(&b, tag)
		assert.Nil(t, err)
		assert.Equal(t, b, []bool{true, false, true})

		err = conf.Map(map[string]interface{}{
			"bools": "abc",
		}).Bind(&b, tag)
		assert.Error(t, err, "bind \\[\\]bool\\[0\\] error; strconv.ParseBool: parsing \\\"abc\\\": invalid syntax")
	})

	t.Run("strings", func(t *testing.T) {
		var s []string

		tag := conf.Tag("${strings:=abc,cde,def}")
		err := conf.Map(nil).Bind(&s, tag)
		assert.Nil(t, err)
		assert.Equal(t, s, []string{"abc", "cde", "def"})

		err = conf.Map(map[string]interface{}{
			"strings": "",
		}).Bind(&s, tag)
		assert.Nil(t, err)
		assert.Equal(t, s, []string{})

		err = conf.Map(map[string]interface{}{
			"strings": "def",
		}).Bind(&s, tag)
		assert.Nil(t, err)
		assert.Equal(t, s, []string{"def"})

		err = conf.Map(map[string]interface{}{
			"strings": []string{"def", "efg", "ghi"},
		}).Bind(&s, tag)
		assert.Nil(t, err)
		assert.Equal(t, s, []string{"def", "efg", "ghi"})
	})

	t.Run("structs", func(t *testing.T) {
		var s []CommonStruct

		tag := conf.Tag("${structs:=abc,cde,def}")
		err := conf.Map(nil).Bind(&s, tag)
		assert.Error(t, err, "bind .* error; .* slice can't have a non empty default value")

		tag = conf.Tag("${structs:=}")
		err = conf.Map(nil).Bind(&s, tag)
		assert.Nil(t, err)
		assert.Equal(t, s, []CommonStruct{})

		tag = conf.Tag("${structs}")
		err = conf.Map(nil).Bind(&s, tag)
		assert.Error(t, err, "bind .* error; .* property \"structs\"; not exist")

		err = conf.Map(map[string]interface{}{
			"structs[0]": map[string]interface{}{
				"int":  3,
				"ints": "1,2,3",
			},
			"structs[2]": "",
		}).Bind(&s, tag)
		assert.Nil(t, err)
		assert.Equal(t, s, []CommonStruct{
			{
				Int:      3,
				Ints:     []int{1, 2, 3},
				Uint:     uint(3),
				Uints:    []uint{1, 2, 3},
				Float:    float64(3),
				Floats:   []float64{1, 2, 3},
				Bool:     true,
				Bools:    []bool{true, false},
				String:   "abc",
				Strings:  []string{"abc", "def", "ghi"},
				Time:     time.Date(2017, 6, 17, 13, 20, 15, 0, time.UTC),
				Duration: 5 * time.Second,
			},
		})

		err = conf.Map(map[string]interface{}{
			"structs": []interface{}{
				map[string]interface{}{
					"int":  3,
					"ints": "1,2,3",
				},
				map[string]interface{}{
					"int":  3,
					"ints": "1,2,3",
				},
			},
		}).Bind(&s, tag)
		assert.Nil(t, err)
		assert.Equal(t, s, []CommonStruct{
			{
				Int:      3,
				Ints:     []int{1, 2, 3},
				Uint:     uint(3),
				Uints:    []uint{1, 2, 3},
				Float:    float64(3),
				Floats:   []float64{1, 2, 3},
				Bool:     true,
				Bools:    []bool{true, false},
				String:   "abc",
				Strings:  []string{"abc", "def", "ghi"},
				Time:     time.Date(2017, 6, 17, 13, 20, 15, 0, time.UTC),
				Duration: 5 * time.Second,
			},
			{
				Int:      3,
				Ints:     []int{1, 2, 3},
				Uint:     uint(3),
				Uints:    []uint{1, 2, 3},
				Float:    float64(3),
				Floats:   []float64{1, 2, 3},
				Bool:     true,
				Bools:    []bool{true, false},
				String:   "abc",
				Strings:  []string{"abc", "def", "ghi"},
				Time:     time.Date(2017, 6, 17, 13, 20, 15, 0, time.UTC),
				Duration: 5 * time.Second,
			},
		})
	})
}

func TestBind_MapValue(t *testing.T) {

	t.Run("error#1", func(t *testing.T) {
		var m map[string]uint
		tag := conf.Tag("${map}")
		err := conf.Map(map[string]interface{}{
			"map": "abc",
		}).Bind(&m, tag)
		assert.Error(t, err, "bind .* error; property 'map' is value")
	})

	t.Run("error#2", func(t *testing.T) {
		var m map[string]uint
		tag := conf.Tag("${map}")
		err := conf.Map(map[string]interface{}{
			"map": map[string]interface{}{
				"a": "1",
				"b": "abc",
			},
		}).Bind(&m, tag)
		assert.Error(t, err, "bind .* error; strconv.ParseUint: parsing \\\"abc\\\": invalid syntax")
	})

	t.Run("error#3", func(t *testing.T) {
		var m map[string]uint
		tag := conf.Tag("${map}")
		err := conf.Map(map[string]interface{}{
			"map": []uint{1, 2, 3},
		}).Bind(&m, tag)
		assert.Error(t, err, "bind .* error; .* resolve property \\\"map.0\\\" error; property \\\"map.0\\\" not exist")
	})

	t.Run("uint", func(t *testing.T) {
		var m map[string]uint

		tag := conf.Tag("${map:=abc,123}")
		err := conf.Map(nil).Bind(&m, tag)
		assert.Error(t, err, "bind map\\[string\\]uint error; map can't have a non empty default value")

		tag = conf.Tag("${map:=}")
		err = conf.Map(nil).Bind(&m, tag)
		assert.Nil(t, err)
		assert.Equal(t, m, map[string]uint{})

		tag = conf.Tag("${map:=}")
		err = conf.Map(map[string]interface{}{
			"map": map[string]uint{
				"abc": 1,
				"def": 2,
			},
		}).Bind(&m, tag)
		assert.Nil(t, err)
		assert.Equal(t, m, map[string]uint{
			"abc": 1,
			"def": 2,
		})

		tag = conf.Tag("${map}")
		err = conf.Map(map[string]interface{}{
			"map": map[string]uint{
				"abc": 1,
				"def": 2,
			},
		}).Bind(&m, tag)
		assert.Nil(t, err)
		assert.Equal(t, m, map[string]uint{
			"abc": 1,
			"def": 2,
		})
	})

	t.Run("int", func(t *testing.T) {
		var m map[string]int

		tag := conf.Tag("${map:=abc,123}")
		err := conf.Map(nil).Bind(&m, tag)
		assert.Error(t, err, "bind map\\[string\\]int error; map can't have a non empty default value")

		tag = conf.Tag("${map:=}")
		err = conf.Map(nil).Bind(&m, tag)
		assert.Nil(t, err)
		assert.Equal(t, m, map[string]int{})

		tag = conf.Tag("${map:=}")
		err = conf.Map(map[string]interface{}{
			"map": map[string]int{
				"abc": 1,
				"def": 2,
			},
		}).Bind(&m, tag)
		assert.Nil(t, err)
		assert.Equal(t, m, map[string]int{
			"abc": 1,
			"def": 2,
		})

		tag = conf.Tag("${map}")
		err = conf.Map(map[string]interface{}{
			"map": map[string]int{
				"abc": 1,
				"def": 2,
			},
		}).Bind(&m, tag)
		assert.Nil(t, err)
		assert.Equal(t, m, map[string]int{
			"abc": 1,
			"def": 2,
		})
	})

	t.Run("float", func(t *testing.T) {
		var m map[string]float32

		tag := conf.Tag("${map:=abc,123}")
		err := conf.Map(nil).Bind(&m, tag)
		assert.Error(t, err, "bind map\\[string\\]float32 error; map can't have a non empty default value")

		tag = conf.Tag("${map:=}")
		err = conf.Map(nil).Bind(&m, tag)
		assert.Nil(t, err)
		assert.Equal(t, m, map[string]float32{})

		tag = conf.Tag("${map:=}")
		err = conf.Map(map[string]interface{}{
			"map": map[string]float32{
				"abc": 1,
				"def": 2,
			},
		}).Bind(&m, tag)
		assert.Nil(t, err)
		assert.Equal(t, m, map[string]float32{
			"abc": 1,
			"def": 2,
		})

		tag = conf.Tag("${map}")
		err = conf.Map(map[string]interface{}{
			"map": map[string]float32{
				"abc": 1,
				"def": 2,
			},
		}).Bind(&m, tag)
		assert.Nil(t, err)
		assert.Equal(t, m, map[string]float32{
			"abc": 1,
			"def": 2,
		})
	})

	t.Run("string", func(t *testing.T) {
		var m map[string]string

		tag := conf.Tag("${map:=abc,123}")
		err := conf.Map(nil).Bind(&m, tag)
		assert.Error(t, err, "bind map\\[string\\]string error; map can't have a non empty default value")

		tag = conf.Tag("${map:=}")
		err = conf.Map(nil).Bind(&m, tag)
		assert.Nil(t, err)
		assert.Equal(t, m, map[string]string{})

		tag = conf.Tag("${map:=}")
		err = conf.Map(map[string]interface{}{
			"map": map[string]float32{
				"abc": 1,
				"def": 2,
			},
		}).Bind(&m, tag)
		assert.Nil(t, err)
		assert.Equal(t, m, map[string]string{
			"abc": "1",
			"def": "2",
		})

		tag = conf.Tag("${map}")
		err = conf.Map(map[string]interface{}{
			"map": map[string]float32{
				"abc": 1,
				"def": 2,
			},
		}).Bind(&m, tag)
		assert.Nil(t, err)
		assert.Equal(t, m, map[string]string{
			"abc": "1",
			"def": "2",
		})
	})

	t.Run("struct", func(t *testing.T) {
		var m map[string]CommonStruct

		tag := conf.Tag("${map:=abc,123}")
		err := conf.Map(nil).Bind(&m, tag)
		assert.Error(t, err, "bind .* error; map can't have a non empty default value")

		tag = conf.Tag("${map:=}")
		err = conf.Map(nil).Bind(&m, tag)
		assert.Nil(t, err)
		assert.Equal(t, m, map[string]CommonStruct{})

		input := map[string]interface{}{
			"map": map[string]interface{}{
				"a": map[string]interface{}{
					"int":  3,
					"ints": "1,2,3",
				},
			},
		}

		expect := map[string]CommonStruct{
			"a": {
				Int:      3,
				Ints:     []int{1, 2, 3},
				Uint:     uint(3),
				Uints:    []uint{1, 2, 3},
				Float:    float64(3),
				Floats:   []float64{1, 2, 3},
				Bool:     true,
				Bools:    []bool{true, false},
				String:   "abc",
				Strings:  []string{"abc", "def", "ghi"},
				Time:     time.Date(2017, 6, 17, 13, 20, 15, 0, time.UTC),
				Duration: 5 * time.Second,
			},
		}

		tag = conf.Tag("${map:=}")
		err = conf.Map(input).Bind(&m, tag)
		assert.Nil(t, err)
		assert.Equal(t, m, expect)

		tag = conf.Tag("${map}")
		err = conf.Map(input).Bind(&m, tag)
		assert.Nil(t, err)
		assert.Equal(t, m, expect)
	})
}

func TestBind_Validate(t *testing.T) {

	t.Run("uint", func(t *testing.T) {
		var v struct {
			Uint uint `value:"${uint:=2}" expr:"$>=3"`
		}

		tag := conf.Key("s")
		err := conf.Map(nil).Bind(&v, tag)
		assert.Error(t, err, "bind .* error; validate failed on \"\\$\\>\\=3\" for value 2")

		err = conf.Map(map[string]interface{}{
			"s": map[string]interface{}{
				"uint": 1,
			},
		}).Bind(&v, tag)
		assert.Error(t, err, "bind .* error; validate failed on \"\\$\\>\\=3\" for value 1")

		err = conf.Map(map[string]interface{}{
			"s": map[string]interface{}{
				"uint": 3,
			},
		}).Bind(&v, tag)
		assert.Nil(t, err)
		assert.Equal(t, v.Uint, uint(3))
	})

	t.Run("int", func(t *testing.T) {
		var v struct {
			Int int `value:"${int:=2}" expr:"$>=3"`
		}

		tag := conf.Key("s")
		err := conf.Map(nil).Bind(&v, tag)
		assert.Error(t, err, "bind .* error; validate failed on \"\\$\\>\\=3\" for value 2")

		err = conf.Map(map[string]interface{}{
			"s": map[string]interface{}{
				"int": 1,
			},
		}).Bind(&v, tag)
		assert.Error(t, err, "bind .* error; validate failed on \"\\$\\>\\=3\" for value 1")

		err = conf.Map(map[string]interface{}{
			"s": map[string]interface{}{
				"int": 3,
			},
		}).Bind(&v, tag)
		assert.Nil(t, err)
		assert.Equal(t, v.Int, 3)
	})

	t.Run("float", func(t *testing.T) {
		var v struct {
			Float float32 `value:"${float:=2}" expr:"$>=3"`
		}

		tag := conf.Key("s")
		err := conf.Map(nil).Bind(&v, tag)
		assert.Error(t, err, "bind .* error; validate failed on \"\\$\\>\\=3\" for value 2")

		err = conf.Map(map[string]interface{}{
			"s": map[string]interface{}{
				"float": 1,
			},
		}).Bind(&v, tag)
		assert.Error(t, err, "bind .* error; validate failed on \"\\$\\>\\=3\" for value 1")

		err = conf.Map(map[string]interface{}{
			"s": map[string]interface{}{
				"float": 3,
			},
		}).Bind(&v, tag)
		assert.Nil(t, err)
		assert.Equal(t, v.Float, float32(3))
	})

	t.Run("string", func(t *testing.T) {
		var v struct {
			String string `value:"${string:=123}" expr:"len($)>=6"`
		}

		tag := conf.Key("s")
		err := conf.Map(nil).Bind(&v, tag)
		assert.Error(t, err, "bind .* error; validate failed on \\\"len\\(\\$\\)\\>\\=6\\\" for value 123")

		err = conf.Map(map[string]interface{}{
			"s": map[string]interface{}{
				"string": "abc",
			},
		}).Bind(&v, tag)
		assert.Error(t, err, "bind .* error; validate failed on \\\"len\\(\\$\\)\\>\\=6\\\" for value abc")

		err = conf.Map(map[string]interface{}{
			"s": map[string]interface{}{
				"string": "123456",
			},
		}).Bind(&v, tag)
		assert.Nil(t, err)
		assert.Equal(t, v.String, "123456")
	})
}

func TestBind_StructValue(t *testing.T) {

	t.Run("unexported", func(t *testing.T) {
		var s struct {
			value int `value:"${a:=3}"`
		}
		err := conf.Map(nil).Bind(&s)
		assert.Nil(t, err)
		assert.Equal(t, s.value, 3)
	})

	t.Run("error_tag", func(t *testing.T) {
		var s struct {
			value int `value:"a:=3"`
		}
		err := conf.Map(nil).Bind(&s)
		assert.Error(t, err, "bind .* error; parse tag 'a:=3' error: invalid syntax")
	})
}

func TestBind_StructFilter(t *testing.T) {

	t.Run("error", func(t *testing.T) {
		var s struct {
			Uint uint `value:"${uint:=3}"`
		}
		p := conf.Map(nil)
		v := reflect.ValueOf(&s).Elem()
		param := conf.BindParam{
			Path: v.Type().String(),
		}
		err := param.BindTag("${ROOT}", "")
		assert.Nil(t, err)
		filter := func(i interface{}, param conf.BindParam) (bool, error) {
			return false, errors.New("this is an error")
		}
		err = conf.BindValue(p, v, v.Type(), param, filter)
		assert.Error(t, err, "bind .* error; this is an error")
	})

	t.Run("filtered", func(t *testing.T) {
		var s struct {
			Uint uint `value:"${uint:=3}"`
		}
		p := conf.Map(nil)
		v := reflect.ValueOf(&s).Elem()
		param := conf.BindParam{
			Path: v.Type().String(),
		}
		err := param.BindTag("${ROOT}", "")
		assert.Nil(t, err)
		filter := func(i interface{}, param conf.BindParam) (bool, error) {
			reflect.ValueOf(i).Elem().SetUint(3)
			return true, nil
		}
		err = conf.BindValue(p, v, v.Type(), param, filter)
		assert.Nil(t, err)
		assert.Equal(t, s.Uint, uint(3))
	})

	t.Run("default", func(t *testing.T) {
		var s struct {
			Uint uint `value:"${uint:=3}"`
		}
		p := conf.Map(nil)
		v := reflect.ValueOf(&s).Elem()
		param := conf.BindParam{
			Path: v.Type().String(),
		}
		err := param.BindTag("${ROOT}", "")
		assert.Nil(t, err)
		filter := func(i interface{}, param conf.BindParam) (bool, error) {
			return false, nil
		}
		err = conf.BindValue(p, v, v.Type(), param, filter)
		assert.Nil(t, err)
		assert.Equal(t, s.Uint, uint(3))
	})
}

func TestBind_Splitter(t *testing.T) {

	t.Run("nil", func(t *testing.T) {
		name := "splitter"
		conf.RegisterSplitter(name, nil)
		defer conf.RemoveSplitter(name)
		var s []int
		err := conf.Map(map[string]interface{}{
			"s": "1;2;3",
		}).Bind(&s, conf.Tag("${s}||splitter"))
		assert.Error(t, err, "bind \\[\\]int error; .* error splitter 'splitter'")
	})

	t.Run("error", func(t *testing.T) {
		name := "splitter"
		conf.RegisterSplitter(name, func(s string) ([]string, error) {
			return nil, errors.New("this is an error")
		})
		defer conf.RemoveSplitter(name)
		var s []int
		err := conf.Map(map[string]interface{}{
			"s": "1;2;3",
		}).Bind(&s, conf.Tag("${s}||splitter"))
		assert.Error(t, err, "bind \\[\\]int error; .* split error; this is an error")
	})

	t.Run("success", func(t *testing.T) {
		name := "splitter"
		conf.RegisterSplitter(name, func(s string) ([]string, error) {
			return strings.Split(s, ";"), nil
		})
		defer conf.RemoveSplitter(name)
		var s []int
		err := conf.Map(map[string]interface{}{
			"s": "1;2;3",
		}).Bind(&s, conf.Tag("${s}||splitter"))
		assert.Nil(t, err)
		assert.Equal(t, s, []int{1, 2, 3})
	})
}

func TestBind_Converter(t *testing.T) {

	t.Run("error", func(t *testing.T) {
		var s struct {
			Point Point `value:"${point}"`
		}
		err := conf.Map(map[string]interface{}{
			"point": "[1,2]",
		}).Bind(&s)
		assert.Error(t, err, "bind .* error; illegal format")
	})

	t.Run("success", func(t *testing.T) {
		var s struct {
			Point Point `value:"${point}"`
		}
		err := conf.Map(map[string]interface{}{
			"point": "(1,2)",
		}).Bind(&s)
		assert.Nil(t, err)
		assert.Equal(t, s.Point, Point{X: 1, Y: 2})
	})
}

func TestBind_ReflectValue(t *testing.T) {

	assert.Panic(t, func() {
		var i int
		v := reflect.ValueOf(i)
		_ = conf.Map(map[string]interface{}{
			"int": 1,
		}).Bind(v, conf.Key("int"))
	}, "reflect: reflect.Value.SetInt using unaddressable value")

	t.Run("error", func(t *testing.T) {
		var i int
		v := reflect.ValueOf(&i)
		err := conf.Map(map[string]interface{}{
			"int": 1,
		}).Bind(v, conf.Key("int"))
		assert.Error(t, err, "bind \\*int error; target should be value type")
	})

	t.Run("success", func(t *testing.T) {
		var i int
		v := reflect.ValueOf(&i).Elem()
		err := conf.Map(map[string]interface{}{
			"int": 1,
		}).Bind(v, conf.Key("int"))
		assert.Nil(t, err)
		assert.Equal(t, i, 1)
	})
}
