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

package gs_dync

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"go-spring.org/spring/conf"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/testing/assert"
)

func TestValue(t *testing.T) {
	var v Value[int]
	assert.That(t, v.Value()).Equal(0)

	refresh := func(prop flatten.Storage) error {
		return v.onRefresh(prop, conf.BindParam{Key: "key", Path: "$"}, true)
	}

	err := refresh(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
		"key": "42",
	})))
	assert.That(t, err).Nil()
	assert.That(t, v.Value()).Equal(42)

	err = refresh(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
		"key": map[string]any{
			"value": "42",
		},
	})))
	assert.Error(t, err).Matches("failed to resolve value at path \\$: property \"key\" is not a simple value")

	time.Sleep(50 * time.Millisecond)
	err = refresh(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
		"key": 59,
	})))
	assert.That(t, err).Nil()

	b, err := json.Marshal(map[string]any{"key": &v})
	assert.That(t, err).Nil()
	assert.String(t, string(b)).JSONEqual(`{"key":59}`)
}

func TestValue_DifferentTypes(t *testing.T) {

	t.Run("string", func(t *testing.T) {
		var v Value[string]
		err := v.onRefresh(
			flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{"key": "hello"})),
			conf.BindParam{Key: "key"},
			true,
		)
		assert.That(t, err).Nil()
		assert.That(t, v.Value()).Equal("hello")
	})

	t.Run("bool", func(t *testing.T) {
		var v Value[bool]
		err := v.onRefresh(
			flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{"key": "true"})),
			conf.BindParam{Key: "key"},
			true,
		)
		assert.That(t, err).Nil()
		assert.That(t, v.Value()).Equal(true)
	})

	t.Run("float64", func(t *testing.T) {
		var v Value[float64]
		err := v.onRefresh(
			flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{"key": "3.14"})),
			conf.BindParam{Key: "key"},
			true,
		)
		assert.That(t, err).Nil()
		assert.That(t, v.Value()).Equal(3.14)
	})

	t.Run("slice", func(t *testing.T) {
		var v Value[[]int]
		err := v.onRefresh(
			flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{"key": []any{1, 2, 3}})),
			conf.BindParam{Key: "key"},
			true,
		)
		assert.That(t, err).Nil()
		assert.That(t, v.Value()).Equal([]int{1, 2, 3})
	})
}

func TestValue_ConcurrentAccess(t *testing.T) {
	var v Value[int]

	err := v.onRefresh(
		flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{"key": "100"})),
		conf.BindParam{Key: "key"},
		true,
	)
	assert.That(t, err).Nil()
	assert.That(t, v.Value()).Equal(100)

	var wg sync.WaitGroup
	const goroutines = 100

	for range goroutines {
		wg.Go(func() {
			val := v.Value()
			assert.Number(t, val).Between(0, 100)
		})
	}

	for i := range goroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := v.onRefresh(
				flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{"key": fmt.Sprintf("%d", idx)})),
				conf.BindParam{Key: "key"},
				true,
			)
			assert.That(t, err).Nil()
		}(i)
	}

	wg.Wait()
}

func TestDync(t *testing.T) {

	t.Run("invalid property format", func(t *testing.T) {
		p := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))

		var v Value[int]
		err := p.RefreshField(
			reflect.ValueOf(&v),
			conf.BindParam{Key: "${invalid..key}"},
		)
		assert.That(t, err).NotNil()
	})

	t.Run("missing required property", func(t *testing.T) {
		p := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))

		var cfg struct {
			Value Value[int] `value:"${required.property}"`
		}

		err := p.RefreshField(
			reflect.ValueOf(&cfg),
			conf.BindParam{Key: "config"},
		)
		assert.That(t, err).NotNil()
	})

	t.Run("type mismatch error", func(t *testing.T) {
		p := New(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"config.value": "not_a_number",
		})))

		var v Value[int]
		err := p.RefreshField(
			reflect.ValueOf(&v),
			conf.BindParam{Key: "config.value"},
		)
		assert.Error(t, err).Matches("strconv.ParseInt: parsing.*invalid syntax")
	})

	t.Run("refresh field", func(t *testing.T) {
		p := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		assert.That(t, p.ObjectsCount()).Equal(0)

		prop := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"config.s1.value": "99",
		}))
		err := p.Refresh(prop)
		assert.That(t, err).Nil()
		assert.That(t, p.Data()).Equal(prop)

		var v int
		err = p.RefreshField(reflect.ValueOf(&v), conf.BindParam{Key: "config.s1.value"})
		assert.That(t, err).Nil()
		assert.That(t, v).Equal(99)
		assert.That(t, p.ObjectsCount()).Equal(0)

		var cfg struct {
			S1 struct {
				Value Value[int] `value:"${value}"`
			} `value:"${s1}"`
			S2 struct {
				Value Value[int] `value:"${value:=123}"`
			} `value:"${s2}"`
		}

		err = p.RefreshField(reflect.ValueOf(&cfg), conf.BindParam{Key: "config"})
		assert.That(t, err).Nil()
		assert.That(t, p.ObjectsCount()).Equal(2)
		assert.That(t, cfg.S1.Value.Value()).Equal(99)
		assert.That(t, cfg.S2.Value.Value()).Equal(123)

		prop = flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"config.s1.value": "99",
			"config.s2.value": "456",
			"config.s4.value": "123",
		}))
		err = p.Refresh(prop)
		assert.That(t, err).Nil()
		assert.That(t, p.ObjectsCount()).Equal(2)
		assert.That(t, cfg.S1.Value.Value()).Equal(99)
		assert.That(t, cfg.S2.Value.Value()).Equal(456)

		prop = flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"config.s1.value": "99",
			"config.s2.value": "456",
			"config.s3.value": "xyz",
		}))
		err = p.Refresh(prop)
		assert.That(t, err).Nil()
		assert.That(t, p.ObjectsCount()).Equal(2)
		assert.That(t, cfg.S1.Value.Value()).Equal(99)
		assert.That(t, cfg.S2.Value.Value()).Equal(456)

		prop = flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"config.s1.value": "xyz",
			"config.s2.value": "abc",
			"config.s3.value": "xyz",
		}))
		err = p.Refresh(prop)
		assert.Error(t, err).Matches("strconv.ParseInt: parsing \"xyz\": invalid syntax")
		assert.Error(t, err).Matches("strconv.ParseInt: parsing \"abc\": invalid syntax")

		s1 := &Value[string]{}
		err = p.RefreshField(reflect.ValueOf(s1), conf.BindParam{Key: "config.s3.value"})
		assert.That(t, err).Nil()
		assert.That(t, s1.Value()).Equal("xyz")
		assert.That(t, p.ObjectsCount()).Equal(3)

		s2 := &Value[int]{}
		err = p.RefreshField(reflect.ValueOf(s2), conf.BindParam{Key: "config.s3.value"})
		assert.Error(t, err).Matches("strconv.ParseInt: parsing \\\"xyz\\\": invalid syntax")
		assert.That(t, p.ObjectsCount()).Equal(4)
	})

	t.Run("refresh nil storage", func(t *testing.T) {
		prop := flatten.NewPropertiesStorage(flatten.NewProperties(nil))
		p := New(prop)

		err := p.Refresh(nil)
		assert.Error(t, err).Matches("properties storage cannot be nil")
		assert.That(t, p.Data()).Equal(prop)
	})

	t.Run("refresh struct", func(t *testing.T) {
		p := New(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"config.s1.value": "99",
		})))

		v := &Value[struct {
			S1 struct {
				Value int `value:"${value}"`
			} `value:"${s1}"`
		}]{}

		var param conf.BindParam
		err := param.BindTag("${config}", "")
		assert.That(t, err).Nil()

		err = p.RefreshField(reflect.ValueOf(v), param)
		assert.That(t, err).Nil()
		assert.That(t, v.Value().S1.Value).Equal(99)

		err = p.Refresh(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"config.s1.value": "xyz",
		})))
		assert.Error(t, err).Matches("strconv.ParseInt: parsing \"xyz\": invalid syntax")
		assert.That(t, v.Value().S1.Value).Equal(99)

		err = p.Refresh(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"config.s1.value": "10",
		})))
		assert.That(t, err).Nil()
		assert.That(t, v.Value().S1.Value).Equal(10)
	})

	t.Run("with default value", func(t *testing.T) {
		p := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))

		var cfg struct {
			Value Value[int] `value:"${property:=42}"`
		}

		err := p.RefreshField(reflect.ValueOf(&cfg), conf.BindParam{Key: "config"})
		assert.That(t, err).Nil()
		assert.That(t, cfg.Value.Value()).Equal(42)
	})

	t.Run("override default value", func(t *testing.T) {
		p := New(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"config.property": "100",
		})))

		var cfg struct {
			Value Value[int] `value:"${property:=42}"`
		}

		err := p.RefreshField(reflect.ValueOf(&cfg), conf.BindParam{Key: "config"})
		assert.That(t, err).Nil()
		assert.That(t, cfg.Value.Value()).Equal(100)
	})

}

// TestValue_ComplexTypes verifies that gs_dync.Value[T] supports complex
// types: structs, maps, nested structs, and their combinations.
func TestValue_ComplexTypes(t *testing.T) {

	t.Run("value of struct", func(t *testing.T) {
		type ServerConfig struct {
			Host string `value:"${host}"`
			Port int    `value:"${port}"`
		}

		var v Value[ServerConfig]
		err := v.onRefresh(
			flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
				"key.host": "127.0.0.1",
				"key.port": "8080",
			})),
			conf.BindParam{Key: "key"},
			true,
		)
		assert.That(t, err).Nil()
		cfg := v.Value()
		assert.That(t, cfg.Host).Equal("127.0.0.1")
		assert.That(t, cfg.Port).Equal(8080)
	})

	t.Run("value of map[string]string", func(t *testing.T) {
		var v Value[map[string]string]
		err := v.onRefresh(
			flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
				"key.a": "1",
				"key.b": "2",
				"key.c": "3",
			})),
			conf.BindParam{Key: "key"},
			true,
		)
		assert.That(t, err).Nil()
		m := v.Value()
		assert.That(t, len(m)).Equal(3)
		assert.That(t, m["a"]).Equal("1")
		assert.That(t, m["b"]).Equal("2")
		assert.That(t, m["c"]).Equal("3")
	})

	t.Run("value of map[string]int", func(t *testing.T) {
		var v Value[map[string]int]
		err := v.onRefresh(
			flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
				"key.a": "10",
				"key.b": "20",
			})),
			conf.BindParam{Key: "key"},
			true,
		)
		assert.That(t, err).Nil()
		m := v.Value()
		assert.That(t, len(m)).Equal(2)
		assert.That(t, m["a"]).Equal(10)
		assert.That(t, m["b"]).Equal(20)
	})

	t.Run("value of map[string]struct", func(t *testing.T) {
		type Item struct {
			Name  string `value:"${name}"`
			Value int    `value:"${value}"`
		}

		var v Value[map[string]Item]
		err := v.onRefresh(
			flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
				"key.alpha.name":  "alpha",
				"key.alpha.value": "100",
				"key.beta.name":   "beta",
				"key.beta.value":  "200",
			})),
			conf.BindParam{Key: "key"},
			true,
		)
		assert.That(t, err).Nil()
		m := v.Value()
		assert.That(t, len(m)).Equal(2)
		assert.That(t, m["alpha"].Name).Equal("alpha")
		assert.That(t, m["alpha"].Value).Equal(100)
		assert.That(t, m["beta"].Name).Equal("beta")
		assert.That(t, m["beta"].Value).Equal(200)
	})

	t.Run("value of struct with nested struct", func(t *testing.T) {
		type DBConfig struct {
			Host string `value:"${host}"`
			Port int    `value:"${port}"`
		}
		type AppConfig struct {
			Name string   `value:"${name}"`
			DB   DBConfig `value:"${db}"`
		}

		var v Value[AppConfig]
		err := v.onRefresh(
			flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
				"key.name":    "myapp",
				"key.db.host": "localhost",
				"key.db.port": "3306",
			})),
			conf.BindParam{Key: "key"},
			true,
		)
		assert.That(t, err).Nil()
		cfg := v.Value()
		assert.That(t, cfg.Name).Equal("myapp")
		assert.That(t, cfg.DB.Host).Equal("localhost")
		assert.That(t, cfg.DB.Port).Equal(3306)
	})

	t.Run("value of struct with slice field", func(t *testing.T) {
		type Config struct {
			Ports []int `value:"${ports}"`
		}

		var v Value[Config]
		err := v.onRefresh(
			flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
				"key.ports": "8080,9090,3030",
			})),
			conf.BindParam{Key: "key"},
			true,
		)
		assert.That(t, err).Nil()
		cfg := v.Value()
		assert.That(t, cfg.Ports).Equal([]int{8080, 9090, 3030})
	})

	t.Run("value of struct with map field", func(t *testing.T) {
		type Config struct {
			Labels map[string]string `value:"${labels}"`
		}

		var v Value[Config]
		err := v.onRefresh(
			flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
				"key.labels.env":  "prod",
				"key.labels.team": "platform",
			})),
			conf.BindParam{Key: "key"},
			true,
		)
		assert.That(t, err).Nil()
		cfg := v.Value()
		assert.That(t, len(cfg.Labels)).Equal(2)
		assert.That(t, cfg.Labels["env"]).Equal("prod")
		assert.That(t, cfg.Labels["team"]).Equal("platform")
	})

	t.Run("value of slice of struct", func(t *testing.T) {
		type Server struct {
			Host string `value:"${host}"`
			Port int    `value:"${port}"`
		}

		var v Value[[]Server]
		err := v.onRefresh(
			flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
				"key[0].host": "srv1",
				"key[0].port": "8080",
				"key[1].host": "srv2",
				"key[1].port": "9090",
			})),
			conf.BindParam{Key: "key"},
			true,
		)
		assert.That(t, err).Nil()
		servers := v.Value()
		assert.That(t, len(servers)).Equal(2)
		assert.That(t, servers[0].Host).Equal("srv1")
		assert.That(t, servers[0].Port).Equal(8080)
		assert.That(t, servers[1].Host).Equal("srv2")
		assert.That(t, servers[1].Port).Equal(9090)
	})

	t.Run("value of time.Duration with converter", func(t *testing.T) {
		var v Value[time.Duration]
		err := v.onRefresh(
			flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
				"key": "5s",
			})),
			conf.BindParam{Key: "key"},
			true,
		)
		assert.That(t, err).Nil()
		assert.That(t, v.Value()).Equal(5 * time.Second)
	})

	t.Run("value of struct with time.Duration field", func(t *testing.T) {
		type TimeoutConfig struct {
			Read  time.Duration `value:"${read}"`
			Write time.Duration `value:"${write}"`
		}

		var v Value[TimeoutConfig]
		err := v.onRefresh(
			flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
				"key.read":  "30s",
				"key.write": "60s",
			})),
			conf.BindParam{Key: "key"},
			true,
		)
		assert.That(t, err).Nil()
		cfg := v.Value()
		assert.That(t, cfg.Read).Equal(30 * time.Second)
		assert.That(t, cfg.Write).Equal(60 * time.Second)
	})

	t.Run("value of map[string]string with one level", func(t *testing.T) {
		// map[string]string binds direct children under the key prefix.
		// Each child key becomes a map key; the value must be a leaf (no further nesting).
		var v Value[map[string]string]
		err := v.onRefresh(
			flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
				"key.alpha": "hello",
				"key.beta":  "world",
			})),
			conf.BindParam{Key: "key"},
			true,
		)
		assert.That(t, err).Nil()
		m := v.Value()
		assert.That(t, len(m)).Equal(2)
		assert.That(t, m["alpha"]).Equal("hello")
		assert.That(t, m["beta"]).Equal("world")
	})

	t.Run("value of map[string]string with deep nesting fails", func(t *testing.T) {
		// Nested keys like key.a.b are not supported for map[string]string
		// because MapKeys returns ["a"] and "a" is not a leaf value.
		var v Value[map[string]string]
		err := v.onRefresh(
			flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
				"key.level1.level2": "deep",
			})),
			conf.BindParam{Key: "key"},
			true,
		)
		assert.That(t, err).NotNil()
	})

	t.Run("value of struct with default values", func(t *testing.T) {
		type Config struct {
			Host    string `value:"${host:=localhost}"`
			Port    int    `value:"${port:=8080}"`
			Timeout string `value:"${timeout:=30s}"`
		}

		var v Value[Config]
		err := v.onRefresh(
			flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
				"key.host": "prod.example.com",
				// port and timeout omitted — should use defaults
			})),
			conf.BindParam{Key: "key"},
			true,
		)
		assert.That(t, err).Nil()
		cfg := v.Value()
		assert.That(t, cfg.Host).Equal("prod.example.com")
		assert.That(t, cfg.Port).Equal(8080)
		assert.That(t, cfg.Timeout).Equal("30s")
	})

	t.Run("value of struct with zero config (all defaults)", func(t *testing.T) {
		type Config struct {
			Host string `value:"${host:=0.0.0.0}"`
			Port int    `value:"${port:=3000}"`
		}

		var v Value[Config]
		err := v.onRefresh(
			flatten.NewPropertiesStorage(flatten.NewProperties(nil)),
			conf.BindParam{Key: "key"},
			true,
		)
		assert.That(t, err).Nil()
		cfg := v.Value()
		assert.That(t, cfg.Host).Equal("0.0.0.0")
		assert.That(t, cfg.Port).Equal(3000)
	})

	t.Run("two-phase refresh: struct validation failure rolls back", func(t *testing.T) {
		type Config struct {
			Port int `value:"${port}"`
		}

		p := New(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"cfg.port": "8080",
		})))

		var v Value[Config]
		err := p.RefreshField(reflect.ValueOf(&v), conf.BindParam{Key: "cfg"})
		assert.That(t, err).Nil()
		assert.That(t, v.Value().Port).Equal(8080)

		// Try to refresh with invalid value — should roll back
		err = p.Refresh(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"cfg.port": "not_a_number",
		})))
		assert.That(t, err).NotNil()
		// Value should be unchanged
		assert.That(t, v.Value().Port).Equal(8080)
	})

	t.Run("two-phase refresh: struct update succeeds", func(t *testing.T) {
		type Config struct {
			Host string `value:"${host}"`
			Port int    `value:"${port}"`
		}

		p := New(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"cfg.host": "old-host",
			"cfg.port": "1111",
		})))

		var v Value[Config]
		err := p.RefreshField(reflect.ValueOf(&v), conf.BindParam{Key: "cfg"})
		assert.That(t, err).Nil()
		assert.That(t, v.Value().Host).Equal("old-host")
		assert.That(t, v.Value().Port).Equal(1111)

		err = p.Refresh(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"cfg.host": "new-host",
			"cfg.port": "9999",
		})))
		assert.That(t, err).Nil()
		assert.That(t, v.Value().Host).Equal("new-host")
		assert.That(t, v.Value().Port).Equal(9999)
	})

	t.Run("two-phase refresh: map update succeeds", func(t *testing.T) {
		p := New(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"cfg.a": "1",
			"cfg.b": "2",
		})))

		var v Value[map[string]int]
		err := p.RefreshField(reflect.ValueOf(&v), conf.BindParam{Key: "cfg"})
		assert.That(t, err).Nil()
		assert.That(t, v.Value()["a"]).Equal(1)
		assert.That(t, v.Value()["b"]).Equal(2)

		err = p.Refresh(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"cfg.x": "10",
			"cfg.y": "20",
		})))
		assert.That(t, err).Nil()
		assert.That(t, len(v.Value())).Equal(2)
		assert.That(t, v.Value()["x"]).Equal(10)
		assert.That(t, v.Value()["y"]).Equal(20)
	})

	t.Run("two-phase refresh: map validation failure rolls back", func(t *testing.T) {
		p := New(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"cfg.a": "1",
		})))

		var v Value[map[string]int]
		err := p.RefreshField(reflect.ValueOf(&v), conf.BindParam{Key: "cfg"})
		assert.That(t, err).Nil()
		assert.That(t, v.Value()["a"]).Equal(1)

		err = p.Refresh(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"cfg.b": "not_an_int",
		})))
		assert.That(t, err).NotNil()
		// Should roll back to original
		assert.That(t, v.Value()["a"]).Equal(1)
		assert.That(t, len(v.Value())).Equal(1)
	})

	t.Run("zero value for struct", func(t *testing.T) {
		type Config struct {
			Host string `value:"${host}"`
			Port int    `value:"${port}"`
		}

		var v Value[Config]
		// Before any refresh, should return zero value
		cfg := v.Value()
		assert.That(t, cfg.Host).Equal("")
		assert.That(t, cfg.Port).Equal(0)
	})

	t.Run("zero value for map", func(t *testing.T) {
		var v Value[map[string]string]
		m := v.Value()
		assert.That(t, m).Nil()
	})

	t.Run("zero value for slice", func(t *testing.T) {
		var v Value[[]int]
		s := v.Value()
		assert.That(t, s).Nil()
	})
}
