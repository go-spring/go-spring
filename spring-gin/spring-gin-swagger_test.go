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

package SpringGin_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/go-spring/go-spring/spring-gin"
	"github.com/go-spring/go-spring/spring-swagger"
	"github.com/go-spring/go-spring/spring-web"
)

func TestSwagger(t *testing.T) {
	c := SpringGin.NewContainer()

	f2 := NewNumberFilter(2)
	f5 := NewNumberFilter(5)
	f7 := NewNumberFilter(7)

	get := func(ctx SpringWeb.WebContext) {
		fmt.Println("invoke get()")
		ctx.String(http.StatusOK, "1")
	}

	c.GET(SpringSwagger.GET("/get", get, f2, f5, f7).Doc("get doc").Build())

	go c.Start(":8080")

	time.Sleep(time.Millisecond * 100)
	fmt.Println()

	resp, _ := http.Get("http://127.0.0.1:8080/get?key=a")
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))
	fmt.Println()
}
