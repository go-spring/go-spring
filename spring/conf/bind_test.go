/*
 * Copyright 2024 The Go-Spring Authors.
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
	"image"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"go-spring.org/spring/conf"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/testing/assert"
	"github.com/spf13/cast"
)

func init() {
	conf.RegisterConverter(PointConverter)
}

type funcFilter func(i any, param conf.BindParam) (bool, error)

func (f funcFilter) Do(i any, param conf.BindParam) (bool, error) {
	return f(i, param)
}

func PointConverter(val string) (image.Point, error) {
	ss := strings.Split(val[1:len(val)-1], ",")
	x := cast.ToInt(ss[0])
	y := cast.ToInt(ss[1])
	return image.Point{X: x, Y: y}, nil
}

func TestConverter(t *testing.T) {
	var s struct {
		Time     time.Time     `value:"${time:=2025-02-01}"`
		Duration time.Duration `value:"${duration:=10s}"`
	}

	t.Run("built-in types", func(t *testing.T) {
		p := flatten.NewPropertiesStorage(flatten.NewProperties(nil))
		err := conf.Bind(p, &s)
		assert.That(t, err).Nil()
		assert.That(t, s.Time).Equal(time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC))
		assert.That(t, s.Duration).Equal(10 * time.Second)
	})

	t.Run("invalid time format", func(t *testing.T) {
		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"time": "2025-02-01M00:00:00",
		}))
		err := conf.Bind(p, &s)
		assert.Error(t, err).Matches("unable to parse date: 2025-02-01M00:00:00")
	})
}

func TestParseTag(t *testing.T) {

	t.Run("simple tag", func(t *testing.T) {
		tag, err := conf.ParseTag("${a}")
		assert.That(t, err).Nil()
		assert.That(t, tag.String()).Equal("${a}")
	})

	t.Run("with default", func(t *testing.T) {
		tag, err := conf.ParseTag("${a:=123}")
		assert.That(t, err).Nil()
		assert.That(t, tag.String()).Equal("${a:=123}")
	})

	t.Run("unmatched braces", func(t *testing.T) {
		_, err := conf.ParseTag("${a:=1,2,3")
		assert.Error(t, err).Matches("invalid syntax tag .*")
	})

	t.Run("missing dollar sign", func(t *testing.T) {
		_, err := conf.ParseTag("{a:=1,2,3}")
		assert.Error(t, err).Matches("invalid syntax tag .*")
	})

	t.Run("extra content outside tag", func(t *testing.T) {
		for _, s := range []string{"prefix${a}", "${a}suffix", "${a}${b}"} {
			_, err := conf.ParseTag(s)
			assert.Error(t, err).Matches("invalid syntax tag .*")
		}
	})

	t.Run("empty key with default", func(t *testing.T) {
		tag, err := conf.ParseTag("${:=default}")
		assert.That(t, err).Nil()
		assert.That(t, tag).Equal(conf.ParsedTag{
			Key:    "",
			Def:    "default",
			HasDef: true,
		})
	})

	t.Run("key with special chars", func(t *testing.T) {
		tag, err := conf.ParseTag("${key-with.dots_and_underscores:=value}")
		assert.That(t, err).Nil()
		assert.That(t, tag).Equal(conf.ParsedTag{
			Key:    "key-with.dots_and_underscores",
			Def:    "value",
			HasDef: true,
		})
	})
}

func TestBindParam(t *testing.T) {

	t.Run("root", func(t *testing.T) {
		var param conf.BindParam
		err := param.BindTag("${ROOT}", "")
		assert.That(t, err).Nil()
		assert.That(t, param).Equal(conf.BindParam{})
	})

	t.Run("normal", func(t *testing.T) {
		var param conf.BindParam
		err := param.BindTag("${a:=1,2,3}", "")
		assert.That(t, err).Nil()
		assert.That(t, param).Equal(conf.BindParam{
			Key:  "a",
			Path: "",
			Tag: conf.ParsedTag{
				Key:    "a",
				Def:    "1,2,3",
				HasDef: true,
			},
			Validate: "",
		})
	})

	t.Run("sub path", func(t *testing.T) {
		var param = conf.BindParam{
			Key:  "s",
			Path: "Struct",
		}
		err := param.BindTag("${a:=1,2,3}", "")
		assert.That(t, err).Nil()
		assert.That(t, param).Equal(conf.BindParam{
			Key:  "s.a",
			Path: "Struct",
			Tag: conf.ParsedTag{
				Key:    "a",
				Def:    "1,2,3",
				HasDef: true,
			},
			Validate: "",
		})
	})

	t.Run("default", func(t *testing.T) {
		var param conf.BindParam
		err := param.BindTag("${:=1,2,3}", "")
		assert.That(t, err).Nil()
		assert.That(t, param).Equal(conf.BindParam{
			Key:  "",
			Path: "",
			Tag: conf.ParsedTag{
				Key:    "",
				Def:    "1,2,3",
				HasDef: true,
			},
			Validate: "",
		})
	})

	t.Run("invalid format", func(t *testing.T) {
		var param conf.BindParam
		err := param.BindTag("a:=123", "")
		assert.Error(t, err).Matches("invalid syntax tag .*")
	})

	t.Run("empty tag", func(t *testing.T) {
		var param conf.BindParam
		err := param.BindTag("${}", "")
		assert.Error(t, err).Matches("invalid syntax tag .*")
	})

	t.Run("empty tag with default", func(t *testing.T) {
		var param conf.BindParam
		err := param.BindTag("${:=}", "")
		assert.Error(t, err).Nil()
	})

	t.Run("nested key", func(t *testing.T) {
		var param = conf.BindParam{
			Key:  "parent",
			Path: "Parent",
		}
		err := param.BindTag("${child.key:=value}", "")
		assert.That(t, err).Nil()
		assert.That(t, param.Key).Equal("parent.child.key")
	})
}

type DBConnection struct {
	UserName string `value:"${username}"`
	Password string `value:"${password}"`
	Url      string `value:"${url}"`
	Port     string `value:"${port}"`
}

type TaggedNestedDB struct {
	DBConnection `value:"${conn}"`
	DB           string `value:"${db}"`
}

type UntaggedNestedDB struct {
	DBConnection
	DB string `value:"${db}"`
}

type Extra struct {
	Bool     bool           `value:"${bool:=true}" expr:"$"`
	Int      int            `value:"${int:=4}" expr:"$==4"`
	Int8     int8           `value:"${int8:=8}" expr:"$==8"`
	Int16    int16          `value:"${int16:=16}" expr:"$==16"`
	Int32    int32          `value:"${int32:=32}" expr:"$==32"`
	Int64    int64          `value:"${int32:=64}" expr:"$==64"`
	Uint     uint           `value:"${uint:=4}" expr:"$==4"`
	Uint8    uint8          `value:"${uint8:=8}" expr:"$==8"`
	Uint16   uint16         `value:"${uint16:=16}" expr:"$==16"`
	Uint32   uint32         `value:"${uint32:=32}" expr:"$==32"`
	Uint64   uint64         `value:"${uint32:=64}" expr:"$==64"`
	Float32  float32        `value:"${float32:=3.2}" expr:"abs($-3.2)<0.000001"`
	Float64  float64        `value:"${float64:=6.4}" expr:"abs($-6.4)<0.000001"`
	String   string         `value:"${str:=xyz}" expr:"$==\"xyz\""`
	Duration time.Duration  `value:"${duration:=10s}"`
	IntsV0   []int          `value:"${intsV0:=}"`
	IntsV1   []int          `value:"${intsV1:=1,2,3}"`
	IntsV2   []int          `value:"${intsV2}"`
	MapV0    map[string]int `value:"${mapV0:=}"`
	MapV2    map[string]int `value:"${mapV2}"`
}

type DBConfig struct {
	DB0   []TaggedNestedDB   `value:"${tagged.db}"`
	DB1   []UntaggedNestedDB `value:"${db}"`
	Extra Extra              `value:"${extra}"`
}

type UnnamedDefault struct {
	Strs []string       `value:"${:=1,2,3}"`
	Ints []int          `value:"${:=}"`
	Map  map[string]int `value:"${:=}"`
}

type AdvancedTypes struct {
	BoolSlice   []bool          `value:"${boolSlice:=true,false,true}"`
	IntSlice    []int           `value:"${intSlice:=1,2,3}"`
	StringSlice []string        `value:"${stringSlice:=a,b,c}"`
	NestedMap   map[string]Data `value:"${nestedMap}"`
	EmptyStruct Data            `value:"${emptyStruct}"`
}

type Data struct {
	Name string `value:"${name}"`
	Age  int    `value:"${age}"`
}

func TestProperties_Bind(t *testing.T) {

	t.Run("nil storage", func(t *testing.T) {
		var v int
		err := conf.Bind(nil, &v, "${v:=1}")
		assert.Error(t, err).Matches("properties storage cannot be nil")
	})

	t.Run("unnamed default", func(t *testing.T) {
		var s UnnamedDefault
		err := conf.Bind(flatten.NewPropertiesStorage(flatten.NewProperties(nil)), &s)
		assert.That(t, err).Nil()
		assert.That(t, s).Equal(UnnamedDefault{
			Strs: []string{"1", "2", "3"},
			Ints: []int{},
			Map:  map[string]int{},
		})
	})

	t.Run("invalid tag", func(t *testing.T) {
		var i int
		err := conf.Bind(flatten.NewPropertiesStorage(flatten.NewProperties(nil)), &i, "$")
		assert.Error(t, err).Matches("invalid syntax tag '\\$'")
	})

	t.Run("non pointer target", func(t *testing.T) {
		err := conf.Bind(flatten.NewPropertiesStorage(flatten.NewProperties(nil)), 5)
		assert.Error(t, err).Matches("target should be a pointer to value type")
	})

	t.Run("nil pointer target", func(t *testing.T) {
		var i *int
		err := conf.Bind(flatten.NewPropertiesStorage(flatten.NewProperties(nil)), i)
		assert.Error(t, err).Matches("target should be a non-nil pointer to value type")
	})

	t.Run("pointer to pointer target", func(t *testing.T) {
		err := conf.Bind(flatten.NewPropertiesStorage(flatten.NewProperties(nil)), new(*int))
		assert.Error(t, err).Matches("target should be a value type")
	})

	t.Run("invalid reflect value target", func(t *testing.T) {
		err := conf.Bind(flatten.NewPropertiesStorage(flatten.NewProperties(nil)), reflect.Value{})
		assert.Error(t, err).Matches("target should be a settable value")
	})

	t.Run("unsettable reflect value target", func(t *testing.T) {
		err := conf.Bind(flatten.NewPropertiesStorage(flatten.NewProperties(nil)), reflect.ValueOf(0), "${:=1}")
		assert.Error(t, err).Matches("target should be a settable value")
	})

	t.Run("validate error", func(t *testing.T) {
		var s struct {
			Value int `value:"${v}" expr:"$>9"`
		}
		err := conf.Bind(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"v": "1",
		})), &s)
		assert.Error(t, err).Matches(".* expression evaluated to false")
	})

	t.Run("array error", func(t *testing.T) {
		err := conf.Bind(flatten.NewPropertiesStorage(flatten.NewProperties(nil)), new(struct {
			Arr [3]string `value:"${arr:=1,2,3}"`
		}))
		assert.Error(t, err).Matches("use slice instead of array")
	})

	t.Run("string to int error", func(t *testing.T) {
		var s struct {
			Value int `value:"${v}"`
		}
		err := conf.Bind(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"v": "abc",
		})), &s)
		assert.Error(t, err).Matches("strconv.ParseInt: parsing .*: invalid syntax")
	})

	t.Run("string to uint error", func(t *testing.T) {
		var s struct {
			Value uint `value:"${v}"`
		}
		err := conf.Bind(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"v": "abc",
		})), &s)
		assert.Error(t, err).Matches("strconv.ParseUint: parsing .*: invalid syntax")
	})

	t.Run("string to float error", func(t *testing.T) {
		var s struct {
			Value float32 `value:"${v}"`
		}
		err := conf.Bind(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"v": "abc",
		})), &s)
		assert.Error(t, err).Matches("strconv.ParseFloat: parsing .*: invalid syntax")
	})

	t.Run("string to bool error", func(t *testing.T) {
		var s struct {
			Value bool `value:"${v}"`
		}
		err := conf.Bind(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"v": "abc",
		})), &s)
		assert.Error(t, err).Matches("strconv.ParseBool: parsing .*: invalid syntax")
	})

	t.Run("slice error", func(t *testing.T) {
		var s struct {
			Value []int `value:"${v}"`
		}
		err := conf.Bind(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"v": []any{
				"1", "2", "a",
			},
		})), &s)
		assert.Error(t, err).Matches("strconv.ParseInt: parsing .*: invalid syntax")
	})

	t.Run("missing slice property", func(t *testing.T) {
		var s struct {
			Value []int `value:"${v}"`
		}
		err := conf.Bind(flatten.NewPropertiesStorage(flatten.NewProperties(nil)), &s)
		assert.Error(t, err).Matches("property \"v\" does not exist")
	})

	t.Run("missing converter for slice", func(t *testing.T) {
		var s struct {
			Value []image.Rectangle `value:"${v:={(1,2)(3,4)}"`
		}
		err := conf.Bind(flatten.NewPropertiesStorage(flatten.NewProperties(nil)), &s)
		assert.Error(t, err).Nil()
	})

	t.Run("map non empty default", func(t *testing.T) {
		var s struct {
			Value map[string]int `value:"${v:=a:b,1:2}"`
		}
		err := conf.Bind(flatten.NewPropertiesStorage(flatten.NewProperties(nil)), &s)
		assert.Error(t, err).Matches("map can't have a non-empty default value")
	})

	t.Run("map from slice", func(t *testing.T) {
		var s struct {
			Value map[string]int `value:"${v}"`
		}
		err := conf.Bind(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"v": []any{
				"1", "2", "3",
			},
		})), &s)
		assert.Error(t, err).Matches("map property \"v\" does not exist")
	})

	t.Run("map type conflict", func(t *testing.T) {
		var s struct {
			Value map[string]int `value:"${v}"`
		}
		err := conf.Bind(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"v": "a:b,1:2",
		})), &s)
		assert.Error(t, err).Matches("map property \"v\" does not exist")
	})

	t.Run("missing map property", func(t *testing.T) {
		var s struct {
			Value map[string]int `value:"${v}"`
		}
		err := conf.Bind(flatten.NewPropertiesStorage(flatten.NewProperties(nil)), &s)
		assert.Error(t, err).Matches("property \"v\" does not exist")
	})

	t.Run("struct non empty default", func(t *testing.T) {
		var s struct {
			Value struct {
				Int int
			} `value:"${v:={123}}"`
		}
		err := conf.Bind(flatten.NewPropertiesStorage(flatten.NewProperties(nil)), &s)
		assert.Error(t, err).Matches("struct can't have a non-empty default value")
	})

	t.Run("unexported field", func(t *testing.T) {
		var s struct {
			int `value:"${v}"`
		}
		err := conf.Bind(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"v": "123",
		})), &s)
		assert.That(t, err).Nil()
		assert.That(t, s.int).Equal(0)
	})

	t.Run("invalid struct tag", func(t *testing.T) {
		var s struct {
			Value int `value:"v"`
		}
		err := conf.Bind(flatten.NewPropertiesStorage(flatten.NewProperties(nil)), &s)
		assert.Error(t, err).Matches("invalid syntax tag 'v'")
	})

	t.Run("embedded interface", func(t *testing.T) {
		var s struct {
			io.Reader
		}
		err := conf.Bind(flatten.NewPropertiesStorage(flatten.NewProperties(nil)), &s)
		assert.That(t, err).Nil()
	})

	t.Run("reflect.Value", func(t *testing.T) {
		var i int
		v := reflect.ValueOf(&i).Elem()
		err := conf.Bind(flatten.NewPropertiesStorage(flatten.NewProperties(nil)), v, "${:=3}")
		assert.That(t, err).Nil()
		assert.That(t, i).Equal(3)
	})

	t.Run("success", func(t *testing.T) {
		expect := DBConfig{
			DB0: []TaggedNestedDB{
				{
					DBConnection: DBConnection{
						UserName: "root",
						Password: "123456",
						Url:      "1.1.1.1",
						Port:     "3306",
					},
					DB: "db1",
				},
				{
					DBConnection: DBConnection{
						UserName: "root",
						Password: "123456",
						Url:      "1.1.1.1",
						Port:     "3306",
					},
					DB: "db2",
				},
			},
			DB1: []UntaggedNestedDB{
				{
					DBConnection: DBConnection{
						UserName: "root",
						Password: "123456",
						Url:      "1.1.1.1",
						Port:     "3306",
					},
					DB: "db1",
				},
				{
					DBConnection: DBConnection{
						UserName: "root",
						Password: "123456",
						Url:      "1.1.1.1",
						Port:     "3306",
					},
					DB: "db2",
				},
			},
			Extra: Extra{
				Bool:     true,
				Int:      int(4),
				Int8:     int8(8),
				Int16:    int16(16),
				Int32:    int32(32),
				Int64:    int64(64),
				Uint:     uint(4),
				Uint8:    uint8(8),
				Uint16:   uint16(16),
				Uint32:   uint32(32),
				Uint64:   uint64(64),
				Float32:  float32(3.2),
				Float64:  6.4,
				String:   "xyz",
				Duration: time.Second * 10,
				IntsV0:   []int{},
				IntsV1:   []int{1, 2, 3},
				IntsV2:   []int{1, 2, 3},
				MapV0:    map[string]int{},
				MapV2:    map[string]int{"a": 1, "b": 2},
			},
		}

		p, err := conf.Load("./testdata/config/app.yaml")
		assert.That(t, err).Nil()

		//fileID := p.AddFile("bind_test.go")

		p.Set("extra.intsV0", "")
		p.Set("extra.intsV2", "1,2,3")
		p.Set("prefix.extra.intsV2", "1,2,3")

		p.Set("extra.mapV2.a", "1")
		p.Set("extra.mapV2.b", "2")
		p.Set("prefix.extra.mapV2.a", "1")
		p.Set("prefix.extra.mapV2.b", "2")

		var c DBConfig
		err = conf.Bind(flatten.NewPropertiesStorage(p), &c)
		assert.That(t, err).Nil()
		assert.That(t, c).Equal(expect)

		err = conf.Bind(flatten.NewPropertiesStorage(p), &c, "${prefix}")
		assert.That(t, err).Nil()
		assert.That(t, c).Equal(expect)
	})

	t.Run("advanced types", func(t *testing.T) {
		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"boolSlice":   "true,false,true",
			"intSlice":    "1,2,3",
			"stringSlice": "a,b,c",
			"nestedMap": map[string]any{
				"user1": map[string]any{
					"name": "Alice",
					"age":  25,
				},
				"user2": map[string]any{
					"name": "Bob",
					"age":  30,
				},
			},
			"emptyStruct": map[string]any{
				"name": "Empty",
				"age":  0,
			},
			"pointerField": map[string]any{
				"name": "Pointer",
				"age":  40,
			},
		}))

		var s AdvancedTypes
		err := conf.Bind(p, &s)
		assert.That(t, err).Nil()

		assert.That(t, s.BoolSlice).Equal([]bool{true, false, true})
		assert.That(t, s.IntSlice).Equal([]int{1, 2, 3})
		assert.That(t, s.StringSlice).Equal([]string{"a", "b", "c"})

		assert.That(t, s.NestedMap).Equal(map[string]Data{
			"user1": {Name: "Alice", Age: 25},
			"user2": {Name: "Bob", Age: 30},
		})

		assert.That(t, s.EmptyStruct).Equal(Data{Name: "Empty", Age: 0})
	})

	t.Run("empty collections with defaults", func(t *testing.T) {
		var s struct {
			EmptySlice []string       `value:"${emptySlice:=}"`
			EmptyMap   map[string]int `value:"${emptyMap:=}"`
		}
		err := conf.Bind(flatten.NewPropertiesStorage(flatten.NewProperties(nil)), &s)
		assert.That(t, err).Nil()
		assert.That(t, s.EmptySlice).Equal([]string{})
		assert.That(t, s.EmptyMap).Equal(map[string]int{})
	})

	t.Run("filter returns false", func(t *testing.T) {
		var param conf.BindParam
		err := param.BindTag("${ROOT}", "")
		assert.That(t, err).Nil()

		var s struct {
			Value int `value:"${v:=3}"`
		}

		v := reflect.ValueOf(&s).Elem()
		err = conf.BindValue(flatten.NewPropertiesStorage(flatten.NewProperties(nil)), v, v.Type(), param,
			funcFilter(func(i any, param conf.BindParam) (bool, error) {
				return false, nil
			}))
		assert.That(t, err).Nil()
		assert.That(t, s.Value).Equal(3)
	})

	t.Run("filter returns true", func(t *testing.T) {
		var param conf.BindParam
		err := param.BindTag("${ROOT}", "")
		assert.That(t, err).Nil()

		var s struct {
			Value int `value:"${v:=3}"`
		}

		v := reflect.ValueOf(&s).Elem()
		err = conf.BindValue(flatten.NewPropertiesStorage(flatten.NewProperties(nil)), v, v.Type(), param,
			funcFilter(func(i any, param conf.BindParam) (bool, error) {
				return true, nil
			}))
		assert.That(t, err).Nil()
		assert.That(t, s.Value).Equal(0)
	})

	t.Run("filter returns error", func(t *testing.T) {
		var param conf.BindParam
		err := param.BindTag("${ROOT}", "")
		assert.That(t, err).Nil()

		var s struct {
			Value int `value:"${v:=3}"`
		}

		v := reflect.ValueOf(&s).Elem()
		err = conf.BindValue(flatten.NewPropertiesStorage(flatten.NewProperties(nil)), v, v.Type(), param,
			funcFilter(func(i any, param conf.BindParam) (bool, error) {
				return false, errutil.Explain(nil, "filter error")
			}))
		assert.Error(t, err).Matches("filter error")
		assert.That(t, s.Value).Equal(0)
	})

	t.Run("bind value nil storage", func(t *testing.T) {
		var param conf.BindParam
		err := param.BindTag("${v:=1}", "")
		assert.That(t, err).Nil()

		var v int
		rv := reflect.ValueOf(&v).Elem()
		err = conf.BindValue(nil, rv, rv.Type(), param, nil)
		assert.Error(t, err).Matches("properties storage cannot be nil")
	})

	t.Run("property reference resolution", func(t *testing.T) {
		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"host": "localhost",
			"port": "8080",
			"url":  "http://${host}:${port}",
		}))

		var s struct {
			URL string `value:"${url}"`
		}

		err := conf.Bind(p, &s)
		assert.That(t, err).Nil()
		assert.That(t, s.URL).Equal("http://localhost:8080")
	})

	t.Run("nested property reference", func(t *testing.T) {
		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"protocol": "https",
			"host":     "example.com",
			"port":     "443",
			"path":     "api",
			"url":      "${protocol}://${host}:${port}/${path}",
		}))

		var s struct {
			URL string `value:"${url}"`
		}

		err := conf.Bind(p, &s)
		assert.That(t, err).Nil()
		assert.That(t, s.URL).Equal("https://example.com:443/api")
	})
}

func TestResolve(t *testing.T) {
	t.Run("unbalanced braces", func(t *testing.T) {
		_, err := conf.ParseTag("${key")
		assert.Error(t, err).Matches("invalid syntax tag .*")
	})

	t.Run("missing property", func(t *testing.T) {
		p := flatten.NewPropertiesStorage(flatten.NewProperties(nil))

		var s struct {
			Value string `value:"${missing}"`
		}

		err := conf.Bind(p, &s)
		assert.Error(t, err).Matches("property \"missing\" does not exist")
	})

	t.Run("missing property with default", func(t *testing.T) {
		p := flatten.NewPropertiesStorage(flatten.NewProperties(nil))

		var s struct {
			Value string `value:"${missing:=default}"`
		}

		err := conf.Bind(p, &s)
		assert.That(t, err).Nil()
		assert.That(t, s.Value).Equal("default")
	})
}

func TestMapBinding(t *testing.T) {
	t.Run("map success", func(t *testing.T) {
		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"config": map[string]any{
				"a": 1,
				"b": 2,
				"c": 3,
			},
		}))

		var s struct {
			Config map[string]int `value:"${config}"`
		}

		err := conf.Bind(p, &s)
		assert.That(t, err).Nil()
		assert.That(t, s.Config).Equal(map[string]int{
			"a": 1,
			"b": 2,
			"c": 3,
		})
	})

	t.Run("non string key returns error", func(t *testing.T) {
		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"config": map[string]any{
				"1": "one",
			},
		}))

		var s struct {
			Config map[int]string `value:"${config}"`
		}

		err := conf.Bind(p, &s)
		assert.Error(t, err).Matches("map key should be string")
	})

	//t.Run("empty map", func(t *testing.T) {
	//	p := flatten.MapProperties(map[string]any{
	//		"config": map[string]any{},
	//	})
	//
	//	var s struct {
	//		Config map[string]int `value:"${config}"`
	//	}
	//
	//	err := conf.Bind(p,&s)
	//	assert.That(t, err).Nil()
	//	assert.That(t, s.Config).Equal(map[string]int{})
	//})
}

func TestSliceBinding(t *testing.T) {
	t.Run("int slice from comma separated string", func(t *testing.T) {
		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"numbers": "1,2,3,4,5",
		}))

		var s struct {
			Numbers []int `value:"${numbers}"`
		}

		err := conf.Bind(p, &s)
		assert.That(t, err).Nil()
		assert.That(t, s.Numbers).Equal([]int{1, 2, 3, 4, 5})
	})

	t.Run("string slice with whitespace", func(t *testing.T) {
		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"values": " a , b , c ",
		}))

		var s struct {
			Values []string `value:"${values}"`
		}

		err := conf.Bind(p, &s)
		assert.That(t, err).Nil()
		assert.That(t, s.Values).Equal([]string{"a", "b", "c"})
	})

	t.Run("bool slice", func(t *testing.T) {
		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"flags": "true,false,true",
		}))

		var s struct {
			Flags []bool `value:"${flags}"`
		}

		err := conf.Bind(p, &s)
		assert.That(t, err).Nil()
		assert.That(t, s.Flags).Equal([]bool{true, false, true})
	})

	t.Run("comma separated string resolves references", func(t *testing.T) {
		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"a":       "1",
			"b":       "2",
			"numbers": "${a},${b}",
		}))

		var s struct {
			Numbers []int `value:"${numbers}"`
		}

		err := conf.Bind(p, &s)
		assert.That(t, err).Nil()
		assert.That(t, s.Numbers).Equal([]int{1, 2})
	})

	t.Run("indexed slice missing zero index", func(t *testing.T) {
		p := flatten.NewPropertiesStorage(flatten.NewProperties(map[string]string{
			"numbers[1]": "2",
		}))

		var s struct {
			Numbers []int `value:"${numbers}"`
		}

		err := conf.Bind(p, &s)
		assert.Error(t, err).Matches(`missing slice index 0`)
	})

	t.Run("indexed slice gap", func(t *testing.T) {
		p := flatten.NewPropertiesStorage(flatten.NewProperties(map[string]string{
			"numbers[0]": "1",
			"numbers[2]": "3",
		}))

		var s struct {
			Numbers []int `value:"${numbers}"`
		}

		err := conf.Bind(p, &s)
		assert.Error(t, err).Matches(`missing slice index 1`)
	})
}

func TestStructBinding(t *testing.T) {
	t.Run("nested struct", func(t *testing.T) {
		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"user": map[string]any{
				"name": "Alice",
				"age":  25,
			},
		}))

		var s struct {
			User Data `value:"${user}"`
		}

		err := conf.Bind(p, &s)
		assert.That(t, err).Nil()
		assert.That(t, s.User).Equal(Data{
			Name: "Alice",
			Age:  25,
		})
	})

	t.Run("embedded struct", func(t *testing.T) {
		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"name": "Bob",
			"age":  30,
		}))

		var s struct {
			Data
		}

		err := conf.Bind(p, &s)
		assert.That(t, err).Nil()
		assert.That(t, s.Data).Equal(Data{
			Name: "Bob",
			Age:  30,
		})
	})
}
