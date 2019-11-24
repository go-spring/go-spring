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

	// 方法 1，通常情况下都能够满足要求。
	SpringBoot.RegisterBean(&MyModule{"123"})

	// 方法 2，如果需要属性值来创建 Bean 的时候非常有用。
	SpringBoot.RegisterModule(func(ctx SpringCore.SpringContext) {
		fmt.Println("get all properties:")
		for k, v := range ctx.GetAllProperties() {
			fmt.Println(k + "=" + fmt.Sprint(v))
		}
	})
}

func TestRunApplication(t *testing.T) {
	os.Setenv(SpringBoot.SPRING_PROFILE, "test")
	SpringBoot.RunApplication("testdata/config/", "k8s:testdata/config/config-map.yaml")
}

////////////////// MyModule ///////////////////

type MyModule struct {
	id string
}

func (m *MyModule) OnStartApplication(ctx SpringBoot.ApplicationContext) {
	fmt.Println("MyModule start")

	var e *MyModule
	ctx.GetBean(&e)
	fmt.Println(e)

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
	fmt.Println(m)

	time.Sleep(200 * time.Millisecond)

	SpringBoot.Exit()
}
