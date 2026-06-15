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
		p, err := conf.Load("./testdata/config/app.properties")
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
		_, err := conf.Load("./testdata/config/xxx.yml")
		assert.Error(t, err).Matches("no such file or directory")
	})

	t.Run("unsupported ext", func(t *testing.T) {
		_, err := conf.Load("./testdata/config/app.unknown")
		assert.Error(t, err).Matches("unsupported file type .unknown")
	})

	t.Run("syntax error", func(t *testing.T) {
		_, err := conf.Load("./testdata/config/err.yaml")
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

	//t.Run("array property as string", func(t *testing.T) {
	//	p := flatten.MapProperties(map[string]any{
	//		"a.b.c": []string{"3"},
	//	})
	//	_, err := conf.Resolve(p,"${a.b.c}")
	//	assert.Error(t, err).Matches("property \"a.b.c\" isn't simple value")
	//})

	t.Run("missing bracket", func(t *testing.T) {
		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"a.b.c": []string{"3"},
		}))
		_, err := conf.Resolve(p, "${a.b.c")
		assert.Error(t, err).Matches("invalid syntax: unmatched braces in '\\${a.b.c'")
	})

	//t.Run("invalid expression", func(t *testing.T) {
	//	p := flatten.MapProperties(map[string]any{
	//		"a.b.c": []string{"3"},
	//	})
	//	_, err := conf.Resolve(p,"${a.b.c[0]}==${a.b.c}")
	//	assert.Error(t, err).Matches("property \"a.b.c\" isn't simple value")
	//})
}

func TestProperties_CopyTo(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		//p := flatten.MapProperties(map[string]any{
		//	"a.b.c": []string{"3"},
		//})
		//assert.That(t, p.Keys()).Equal([]string{
		//	"a.b.c[0]",
		//})

		//assert.That(t, p.Exists("a.b.c")).True()
		//assert.That(t, p.Exists("a.b.c[0]")).True()
		//assert.That(t, p.Get("a.b.c[0]")).Equal("3")
		//assert.That(t, p.Data()).Equal(map[string]string{
		//	"a.b.c[0]": "3",
		//})

		//s := flatten.MapProperties(map[string]any{
		//	"a.b.c": []string{"4", "5"},
		//})
		//assert.That(t, s.Keys()).Equal([]string{
		//	"a.b.c[0]",
		//	"a.b.c[1]",
		//})

		//assert.That(t, s.Exists("a.b.c")).True()
		//assert.That(t, s.Exists("a.b.c[0]")).True()
		//assert.That(t, s.Exists("a.b.c[1]")).True()
		//assert.That(t, s.Data()).Equal(map[string]string{
		//	"a.b.c[0]": "4",
		//	"a.b.c[1]": "5",
		//})

		//err := p.CopyTo(s)
		//assert.That(t, err).Nil()
		//assert.That(t, s.Data()).Equal(map[string]string{
		//	"a.b.c[0]": "3",
		//	"a.b.c[1]": "5",
		//})
	})

	t.Run("type conflict", func(t *testing.T) {
		p := flatten.MapProperties(map[string]any{
			"a.b.c": []string{"3"},
		})
		assert.That(t, p.Data()).Equal(map[string]string{
			"a.b.c[0]": "3",
		})

		//s := flatten.MapProperties(map[string]any{
		//	"a.b.c": "3",
		//})
		//assert.That(t, s.Get("a.b.c")).Equal("3")

		//err := p.CopyTo(s)
		//assert.Error(t, err).Matches("path a.b.c\\[0\\] conflicts with existing structure")
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
