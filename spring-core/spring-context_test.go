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

package SpringCore_test

import (
	"fmt"
	"testing"

	"github.com/didi/go-spring/spring-core"
	"github.com/didi/go-spring/spring-utils"
)

func TestValueWire(t *testing.T) {

	type People struct {
		FirstName string `value:"${people.first_name}"`
		LastName  string `value:"${people.last_name:=Green}"`
	}

	ctx := SpringCore.NewDefaultSpringContext()

	p := new(People)
	ctx.RegisterBean(p)

	ctx.SetProperties("people.first_name", "Jim")

	if err := ctx.AutoWireBeans(); err != nil {
		panic(err)
	}

	fmt.Println(SpringUtils.ToJson(p))
}

func TestBeanWire(t *testing.T) {

	type Config struct {
		Name string
	}

	type DataSource struct {
		Url string
	}

	type Application struct {
		Config     *Config     `autowire:""`
		DataSource *DataSource `autowire:"ds"`
	}

	ctx := SpringCore.NewDefaultSpringContext()

	app := new(Application)
	ctx.RegisterBean(app)

	cfg := &Config{Name: "application.cfg"}
	ctx.RegisterBean(cfg)

	ds := &DataSource{
		Url: "mysql:127.0.0.1...",
	}

	ctx.RegisterBean(ds)
	ctx.RegisterNameBean("ds", ds)

	ctx.AutoWireBeans()

	fmt.Println(SpringUtils.ToJson(app))
}
