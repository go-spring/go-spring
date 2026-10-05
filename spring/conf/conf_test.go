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
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"go-spring.org/spring/conf"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/testing/assert"
)

func TestProperties_Load(t *testing.T) {

	t.Run("success", func(t *testing.T) {
		p, err := conf.Load(context.Background(), "./testdata/config/app.properties")
		assert.That(t, err).Nil()
		assert.That(t, p.Data()).Equal(map[string]string{
			"properties.list[0]":          "1",
			"properties.list[1]":          "2",
			"properties.obj.list[0].age":  "4",
			"properties.obj.list[0].name": "tom",
			"properties.obj.list[1].age":  "2",
			"properties.obj.list[1].name": "jerry",
		})
	})

	t.Run("file not exist", func(t *testing.T) {
		_, err := conf.Load(context.Background(), "./testdata/config/xxx.yml")
		assert.Error(t, err).Matches("no such file or directory")
	})

	t.Run("unsupported ext", func(t *testing.T) {
		_, err := conf.Load(context.Background(), "./testdata/config/app.unknown")
		assert.Error(t, err).Matches("unsupported config format")
	})

	t.Run("syntax error", func(t *testing.T) {
		_, err := conf.Load(context.Background(), "./testdata/config/err.yaml")
		assert.Error(t, err).Matches("did not find expected node content")
	})
}

func TestProperties_Resolve(t *testing.T) {

	t.Run("nil storage", func(t *testing.T) {
		_, err := conf.Resolve(nil, "${a.b.c}")
		assert.Error(t, err).Matches("p cannot be nil")
	})

	t.Run("success", func(t *testing.T) {
		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"a.b.c": []string{"3"},
		}))

		s, err := conf.Resolve(p, "${a.b.c[0]}")
		assert.That(t, err).Nil()
		assert.That(t, s).Equal("3")
	})

	t.Run("success with default", func(t *testing.T) {
		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"a.b.c": []string{"3"},
		}))
		s, err := conf.Resolve(p, "${x:=${a.b.c[0]}}")
		assert.That(t, err).Nil()
		assert.That(t, s).Equal("3")
	})

	t.Run("key with default", func(t *testing.T) {
		p := flatten.NewPropertiesStorage(flatten.NewProperties(nil))
		s, err := conf.Resolve(p, "${a.b.c:=123}")
		assert.That(t, err).Nil()
		assert.That(t, s).Equal("123")
	})

	t.Run("key not exist", func(t *testing.T) {
		p := flatten.NewPropertiesStorage(flatten.NewProperties(nil))
		_, err := conf.Resolve(p, "${a.b.c}")
		assert.Error(t, err).Matches("property \"a.b.c\" does not exist")
	})

	t.Run("circular reference", func(t *testing.T) {
		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"a": "${b}",
			"b": "${a}",
		}))
		_, err := conf.Resolve(p, "${a}")
		assert.Error(t, err).Matches("circular property reference \"a\"")
	})

	t.Run("same reference repeated", func(t *testing.T) {
		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"a": "1",
		}))
		s, err := conf.Resolve(p, "${a}-${a}")
		assert.That(t, err).Nil()
		assert.That(t, s).Equal("1-1")
	})

	t.Run("reference depth exceeded", func(t *testing.T) {
		m := make(map[string]any)
		for i := range 120 {
			m[fmt.Sprintf("a%d", i)] = fmt.Sprintf("${a%d}", i+1)
		}
		m["a120"] = "ok"
		p := flatten.NewPropertiesStorage(flatten.MapProperties(m))
		_, err := conf.Resolve(p, "${a0}")
		assert.Error(t, err).Matches("property reference depth exceeds 100")
	})

	t.Run("missing bracket", func(t *testing.T) {
		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"a.b.c": []string{"3"},
		}))
		_, err := conf.Resolve(p, "${a.b.c")
		assert.Error(t, err).Matches("invalid syntax: unmatched braces in '\\${a.b.c'")
	})
}

// TestPropertiesStorage verifies the flatten.Storage seam that conf binds
// against: hierarchical data is flattened into indexed leaf keys, prefix keys
// report Exists, and map/slice discovery returns the child structure.
func TestPropertiesStorage(t *testing.T) {
	p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
		"a.b.c": []string{"4", "5"},
		"a.b.d": "x",
	}))

	assert.That(t, p.Data()).Equal(map[string]string{
		"a.b.c[0]": "4",
		"a.b.c[1]": "5",
		"a.b.d":    "x",
	})

	t.Run("exists", func(t *testing.T) {
		assert.That(t, p.Exists("a")).True()     // prefix of deeper keys
		assert.That(t, p.Exists("a.b")).True()   // prefix of deeper keys
		assert.That(t, p.Exists("a.b.c")).True() // prefix of indexed keys
		assert.That(t, p.Exists("a.b.c[0]")).True()
		assert.That(t, p.Exists("a.b.d")).True()  // exact leaf key
		assert.That(t, p.Exists("a.b.e")).False() // no such key
	})

	t.Run("value", func(t *testing.T) {
		v, ok := p.Value("a.b.c[0]")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("4")

		_, ok = p.Value("a.b.c") // slice node, not a leaf
		assert.That(t, ok).False()
	})

	t.Run("map keys", func(t *testing.T) {
		keys := make(map[string]struct{})
		assert.That(t, p.MapKeys("a.b", keys)).True()
		assert.That(t, keys).Equal(map[string]struct{}{"c": {}, "d": {}})

		keys = make(map[string]struct{})
		assert.That(t, p.MapKeys("a.b.e", keys)).False()
		assert.That(t, len(keys)).Equal(0)
	})

	t.Run("slice entries", func(t *testing.T) {
		entries := make(map[string]string)
		assert.That(t, p.SliceEntries("a.b.c", entries)).True()
		assert.That(t, entries).Equal(map[string]string{
			"a.b.c[0]": "4",
			"a.b.c[1]": "5",
		})

		entries = make(map[string]string)
		assert.That(t, p.SliceEntries("a.b.d", entries)).False() // leaf, not a slice
		assert.That(t, len(entries)).Equal(0)
	})
}

type bindEachConfig struct {
	Port int `value:"${port:=0}"`
}

func TestBindEach(t *testing.T) {

	t.Run("empty map", func(t *testing.T) {
		p := flatten.NewPropertiesStorage(flatten.NewProperties(nil))
		calls := 0
		err := conf.BindEach(p, "${servers:=}", func(name string, c bindEachConfig) error {
			calls++
			return nil
		})
		assert.That(t, err).Nil()
		assert.That(t, calls).Equal(0)
	})

	t.Run("multiple entries", func(t *testing.T) {
		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"servers": map[string]any{
				"a": map[string]any{"port": 1},
				"b": map[string]any{"port": 2},
			},
		}))
		got := make(map[string]int)
		err := conf.BindEach(p, "${servers}", func(name string, c bindEachConfig) error {
			got[name] = c.Port
			return nil
		})
		assert.That(t, err).Nil()
		assert.That(t, got).Equal(map[string]int{"a": 1, "b": 2})
	})

	t.Run("bind error propagates", func(t *testing.T) {
		p := flatten.NewPropertiesStorage(flatten.NewProperties(nil))
		err := conf.BindEach(p, "${servers}", func(name string, c bindEachConfig) error {
			return nil
		})
		assert.Error(t, err).Matches(`map property "servers" does not exist`)
	})

	t.Run("fn error aborts the loop", func(t *testing.T) {
		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"servers": map[string]any{
				"a": map[string]any{"port": 1},
				"b": map[string]any{"port": 2},
			},
		}))
		calls := 0
		err := conf.BindEach(p, "${servers}", func(name string, c bindEachConfig) error {
			calls++
			return fmt.Errorf("boom")
		})
		assert.Error(t, err).Matches(`entry "\w+" failed: boom`)
		assert.That(t, calls).Equal(1) // first error aborts, remaining entries are skipped
	})
}

func BenchmarkResolve(b *testing.B) {
	const src = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

	data := make([]byte, 2000)
	for i := range len(data) {
		data[i] = src[rand.Intn(len(src))]
	}
	s := string(data)

	b.Run("contains", func(b *testing.B) {
		for b.Loop() {
			_ = strings.Contains(s, "${")
		}
	})

	p := flatten.NewPropertiesStorage(flatten.NewProperties(nil))
	b.Run("resolve", func(b *testing.B) {
		for b.Loop() {
			_, _ = conf.Resolve(p, s)
		}
	})
}

type nilConverterTarget struct{}

func TestRegisterNilConverter(t *testing.T) {
	var fn conf.Converter[nilConverterTarget]
	assert.Panic(t, func() {
		conf.RegisterConverter(fn)
	}, "converter for type .* cannot be nil")
}

func TestRegisterNilValidateFunc(t *testing.T) {
	const name = "nilValidateFuncForTest"
	var fn conf.ValidateFunc[int]
	assert.Panic(t, func() {
		conf.RegisterValidateFunc(name, fn)
	}, "validate function nilValidateFuncForTest cannot be nil")
}

func TestRegisterValidateFuncEmptyName(t *testing.T) {
	assert.Panic(t, func() {
		conf.RegisterValidateFunc("", func(int) (bool, error) { return true, nil })
	}, "validate function name can't be empty")
}

func TestRegisterValidateFuncDuplicateName(t *testing.T) {
	const name = "dupValidateFuncForTest"
	conf.RegisterValidateFunc(name, func(int) (bool, error) { return true, nil })
	assert.Panic(t, func() {
		conf.RegisterValidateFunc(name, func(int) (bool, error) { return true, nil })
	}, "validate function "+name+" already exists")
}

type dupConverterTarget struct{}

func TestRegisterDuplicateConverter(t *testing.T) {
	conf.RegisterConverter(func(string) (dupConverterTarget, error) { return dupConverterTarget{}, nil })

	assert.Panic(t, func() {
		conf.RegisterConverter(func(string) (dupConverterTarget, error) { return dupConverterTarget{}, nil })
	}, "converter for type .* already exists")
}
