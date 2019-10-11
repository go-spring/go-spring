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

package SpringEcho_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/go-spring/go-spring/spring-echo"
	"github.com/go-spring/go-spring/spring-utils"
	"github.com/go-spring/go-spring/spring-web"
)

func TestContainer(t *testing.T) {
	c := SpringEcho.NewContainer()

	store := make(map[string]string)

	c.GET("/get", func(ctx SpringWeb.WebContext) {

		key := ctx.QueryParam("key")
		fmt.Println("/get", "key=", key)

		val := store[key]
		fmt.Println("/get", "val=", val)

		ctx.String(http.StatusOK, val)
	})

	c.POST("/set", func(ctx SpringWeb.WebContext) {

		var param struct {
			A string `form:"a" json:"a"`
		}

		ctx.Bind(&param)

		fmt.Println("/set", "param="+SpringUtils.ToJson(param))

		store["a"] = param.A
	})

	c.GET("/panic", func(ctx SpringWeb.WebContext) {
		panic("this is a panic")
	})

	go c.Start(":8080")

	time.Sleep(time.Millisecond * 100)

	resp, _ := http.Get("http://127.0.0.1:8080/get?key=a")
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))

	http.PostForm("http://127.0.0.1:8080/set", url.Values{
		"a": []string{"1"},
	})

	resp, _ = http.Get("http://127.0.0.1:8080/get?key=a")
	body, _ = ioutil.ReadAll(resp.Body)
	fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))

	resp, _ = http.Get("http://127.0.0.1:8080/panic")
	body, _ = ioutil.ReadAll(resp.Body)
	fmt.Println("code:", resp.StatusCode, "||", "resp:", string(body))
}
