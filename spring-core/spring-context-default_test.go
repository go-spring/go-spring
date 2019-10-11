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

	"github.com/go-spring/go-spring/spring-core"
	"github.com/go-spring/go-spring/spring-core/testdata/bar"
	"github.com/go-spring/go-spring/spring-core/testdata/foo"
	"github.com/go-spring/go-spring/spring-utils"
	"github.com/stretchr/testify/assert"
)

func TestValueWire(t *testing.T) {

	type People struct {
		FirstName string `value:"${people.first_name}"`
		LastName  string `value:"${people.last_name:=Green}"`
	}

	ctx := SpringCore.NewDefaultSpringContext()

	p := new(People)
	ctx.RegisterSingletonBean(p)

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
	ctx.RegisterSingletonBean(app)

	cfg := &Config{Name: "application.cfg"}
	ctx.RegisterSingletonBean(cfg)

	ds := &DataSource{
		Url: "mysql:127.0.0.1...",
	}

	ctx.RegisterSingletonBean(ds)
	ctx.RegisterSingletonNameBean("ds", ds)

	barBean := new(foo.Demo)
	fooBean := new(bar.Demo)
	ctx.RegisterSingletonBean(barBean)
	ctx.RegisterSingletonBean(fooBean)

	if e := ctx.AutoWireBeans(); e != nil {
		t.Error(e)
	}

	for _, v := range ctx.GetAllBeanNames() {
		t.Logf("bean name : %v", v)
	}

	var (
		f foo.Demo
		b bar.Demo
	)

	foundFooBean := ctx.FindBeanByType(&f)
	foundFarBean := ctx.FindBeanByType(&b)

	assert.NotEqual(t, foundFarBean, foundFooBean)

	fmt.Println(SpringUtils.ToJson(app))
}
