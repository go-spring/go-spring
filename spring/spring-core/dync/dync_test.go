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

package dync_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/dync"
)

type Config struct {
	Int   dync.Int64   `value:"${int:=3}"`
	Float dync.Float64 `value:"${float:=1.2}"`
	Map   dync.Ref     `value:"${map:=}"`
	Slice dync.Ref     `value:"${slice:=}"`
	Event dync.Event   `value:"${event}"`
}

func TestDynamic(t *testing.T) {

	mgr := dync.New()

	p := conf.New()
	p.Set("int", 1)
	p.Set("float", 5.4)
	p.Set("map.a", 3)
	p.Set("map.b", 7)
	p.Set("slice[0]", 2)
	p.Set("slice[1]", 9)
	mgr.Refresh(p)

	cfg := new(Config)

	b, _ := json.Marshal(cfg)
	fmt.Printf("%s\n", string(b))

	cfgValue := reflect.ValueOf(cfg).Elem()
	cfgType := cfgValue.Type()

	{
		{
			var param conf.BindParam
			if err := param.BindTag(cfgType.Field(0).Tag.Get("value")); err != nil {
				t.Fatal(err)
			}
			v := cfgValue.Field(0).Addr().Interface().(dync.Value)
			v.SetParam(param)
			mgr.Watch(v)
		}
		{
			var param conf.BindParam
			if err := param.BindTag(cfgType.Field(1).Tag.Get("value")); err != nil {
				t.Fatal(err)
			}
			v := cfgValue.Field(1).Addr().Interface().(dync.Value)
			v.SetParam(param)
			mgr.Watch(v)
		}
		{
			var param conf.BindParam
			if err := param.BindTag(cfgType.Field(2).Tag.Get("value")); err != nil {
				t.Fatal(err)
			}
			v := cfgValue.Field(2).Addr().Interface().(dync.Value)
			v.SetParam(param)
			mgr.Watch(v)
		}
		{
			var param conf.BindParam
			if err := param.BindTag(cfgType.Field(3).Tag.Get("value")); err != nil {
				t.Fatal(err)
			}
			v := cfgValue.Field(3).Addr().Interface().(dync.Value)
			v.SetParam(param)
			mgr.Watch(v)
		}
		{
			var param conf.BindParam
			if err := param.BindTag(cfgType.Field(4).Tag.Get("value")); err != nil {
				t.Fatal(err)
			}
			v := cfgValue.Field(4).Addr().Interface().(dync.Value)
			v.SetParam(param)
			mgr.Watch(v)
		}
	}

	cfg.Slice.Init(make([]string, 0))
	cfg.Map.Init(make(map[string]string))
	cfg.Event.Init(func(prop *conf.Properties) error {
		fmt.Println("event fired.")
		return nil
	})

	b, _ = json.Marshal(cfg)
	fmt.Printf("%s\n", string(b))

	p = conf.New()
	p.Set("int", 4)
	p.Set("float", 2.3)
	p.Set("map.a", 1)
	p.Set("map.b", 2)
	p.Set("slice[0]", 3)
	p.Set("slice[1]", 4)
	mgr.Refresh(p)

	b, _ = json.Marshal(cfg)
	fmt.Printf("%s\n", string(b))

	p = conf.New()
	p.Set("int", 6)
	p.Set("float", 5.1)
	p.Set("map.a", 9)
	p.Set("map.b", 8)
	p.Set("slice[0]", 7)
	p.Set("slice[1]", 6)
	mgr.Refresh(p)

	b, _ = json.Marshal(cfg)
	fmt.Printf("%s\n", string(b))
}
