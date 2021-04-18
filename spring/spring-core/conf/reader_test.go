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
	"fmt"
	"strings"
	"testing"

	"github.com/go-spring/spring-core/assert"
	"github.com/go-spring/spring-core/conf"
	"github.com/spf13/viper"
)

func TestPropertiesReader(t *testing.T) {

	// viper 的 properties 不支持数组
	t.Run("slice", func(t *testing.T) {
		v := viper.New()
		v.SetConfigType("properties")
		_ = v.ReadConfig(strings.NewReader(`
			s[0].a=b
		`))
		assert.Nil(t, v.Get("s"))
		assert.Equal(t, v.Get("s[0]"), map[string]interface{}{"a": "b"})
	})

	// viper 的 properties 支持引用
	t.Run("ref", func(t *testing.T) {
		v := viper.New()
		v.SetConfigType("properties")
		_ = v.ReadConfig(strings.NewReader(`
			a.b=c
			d.e=${a.b}
		`))
		assert.Equal(t, v.Get("a.b"), "c")
		assert.Equal(t, v.Get("d.e"), "c")
	})
}

func TestYamlReader(t *testing.T) {

	// viper 的 yaml 不支持数组 key
	t.Run("slice", func(t *testing.T) {
		v := viper.New()
		v.SetConfigType("yaml")
		err := v.ReadConfig(strings.NewReader("point:\n  -(2,3)\n  -(5,3)\n"))
		fmt.Println(err, v.AllKeys())
	})

	t.Run("slice slice", func(t *testing.T) {
		v := viper.New()
		v.SetConfigType("yaml")
		err := v.ReadConfig(strings.NewReader(`
k:
  -
    - 1
    - 2
  -
    - 3
    - 4
`))
		fmt.Println(err, v.AllKeys())
	})
}

func TestTomlReader(t *testing.T) {

	// viper 的 toml 不支持数组 key
	t.Run("slice", func(t *testing.T) {
		v := viper.New()
		v.SetConfigType("toml")
		err := v.ReadConfig(strings.NewReader(`point=["(2,3)","(5,3)"]`))
		fmt.Println(err, v.AllKeys())
	})

	t.Run("", func(t *testing.T) {
		p, _ := conf.Read([]byte("a.b=c"), ".properties")
		m := p.Get("a").(map[string]interface{})
		m["d"] = "e"
		fmt.Println(p)
	})
}
