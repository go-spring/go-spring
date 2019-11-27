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

package SpringBoot_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-spring/go-spring/spring-boot"
	"github.com/go-spring/go-spring/spring-core"
)

func init() {
	SpringBoot.RegisterBean(new(MyRunner))
	SpringBoot.RegisterBeanFn(NewMyModule, "${message}")
}

func TestRunApplication(t *testing.T) {
	os.Setenv(SpringBoot.SPRING_PROFILE, "test")
	SpringBoot.RunApplication("testdata/config/", "k8s:testdata/config/config-map.yaml")
}

////////////////// MyRunner ///////////////////

type MyRunner struct {
	Ctx SpringCore.SpringContext `autowire:""`
}

func (r *MyRunner) Run() {
	fmt.Println("get all properties:")
	for k, v := range r.Ctx.GetAllProperties() {
		fmt.Println(k + "=" + fmt.Sprint(v))
	}
}

////////////////// MyModule ///////////////////

type MyModule struct {
	msg string
}

func NewMyModule(msg string) *MyModule {
	return &MyModule{
		msg: msg,
	}
}

func (m *MyModule) OnStartApplication(ctx SpringBoot.ApplicationContext) {
	fmt.Println("MyModule start")

	var e *MyModule
	ctx.GetBean(&e)
	fmt.Printf("event: %+v\n", e)

	ctx.SafeGoroutine(Process)
}

func (m *MyModule) OnStopApplication(ctx SpringBoot.ApplicationContext) {
	fmt.Println("MyModule stop")
}

func Process() {

	defer fmt.Println("go stop")
	fmt.Println("go start")

	var m *MyModule
	SpringBoot.GetBean(&m)
	fmt.Printf("process: %+v\n", m)

	time.Sleep(200 * time.Millisecond)

	SpringBoot.Exit()
}
