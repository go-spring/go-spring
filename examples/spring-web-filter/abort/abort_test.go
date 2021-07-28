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

package abort_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-spring/examples/spring-boot-filter/abort"
	"github.com/go-spring/spring-core/web"
	"github.com/go-spring/spring-echo"
	"github.com/go-spring/spring-gin"
	"github.com/go-spring/spring-stl/util"
	"github.com/magiconair/properties/assert"
)

func testAbort(t *testing.T, fn func() web.Container,
	testAbort bool, expect []string) {

	c := fn()
	s := &abort.StringArray{}
	c.AddFilter(abort.NewPushFilter(1, false, s))
	c.AddFilter(abort.NewPushFilter(2, testAbort, s))
	c.GetMapping("/", func(webCtx web.Context) {
		webCtx.String("hello world")
	})

	go c.Start()
	time.Sleep(30 * time.Millisecond)

	resp, err := http.Get("http://localhost:8080/")
	util.Panic(err).When(err != nil)
	fmt.Println(resp.Status, s.Data)
	assert.Equal(t, s.Data, expect)

	c.Stop(context.Background())
}

func test(t *testing.T, fn func() web.Container) {
	testAbort(t, fn, false, []string{"1", "2", "2", "1"})
	testAbort(t, fn, true, []string{"1"})
}

func TestFilterAbort(t *testing.T) {

	// 测试 gin 服务器
	test(t, func() web.Container {
		return SpringGin.NewContainer(web.ContainerConfig{Port: 8080})
	})

	// 测试 echo 服务器
	test(t, func() web.Container {
		return SpringEcho.NewContainer(web.ContainerConfig{Port: 8080})
	})
}
