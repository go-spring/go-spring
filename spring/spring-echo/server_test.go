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
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/web"
	"github.com/go-spring/spring-echo"
	"github.com/labstack/echo/v4"
)

func TestContext_PanicEchoHttpError(t *testing.T) {
	c := SpringEcho.New(web.ServerConfig{Port: 8080})
	c.AddPrefilter(web.FuncPrefilter(func(ctx web.Context, chain web.FilterChain) {
		fmt.Println("<<log>>")
		chain.Continue(ctx)
	}))
	c.GetMapping("/", func(ctx web.Context) {
		panic(echo.ErrTooManyRequests)
	})
	go c.Start()
	defer c.Stop(context.Background())
	time.Sleep(10 * time.Millisecond)
	response, err := http.Get("http://127.0.0.1:8080/")
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()
	b, _ := ioutil.ReadAll(response.Body)
	fmt.Println(string(b))
	fmt.Println(response.Status)
	assert.Equal(t, response.StatusCode, http.StatusTooManyRequests)
	assert.Equal(t, string(b), "Too Many Requests")
}

func TestContext_PanicString(t *testing.T) {
	c := SpringEcho.New(web.ServerConfig{Port: 8080})
	c.GetMapping("/", func(ctx web.Context) {
		panic("this is an error")
	})
	go c.Start()
	defer c.Stop(context.Background())
	time.Sleep(10 * time.Millisecond)
	response, err := http.Get("http://127.0.0.1:8080/")
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()
	b, _ := ioutil.ReadAll(response.Body)
	fmt.Println(string(b))
	fmt.Println(response.Status)
	assert.Equal(t, response.StatusCode, http.StatusOK)
	assert.Equal(t, string(b), "this is an error")
}

func TestContext_PanicError(t *testing.T) {
	c := SpringEcho.New(web.ServerConfig{Port: 8080})
	c.GetMapping("/", func(ctx web.Context) {
		panic(errors.New("this is an error"))
	})
	go c.Start()
	defer c.Stop(context.Background())
	time.Sleep(10 * time.Millisecond)
	response, err := http.Get("http://127.0.0.1:8080/")
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()
	b, _ := ioutil.ReadAll(response.Body)
	fmt.Println(string(b))
	fmt.Println(response.Status)
	assert.Equal(t, response.StatusCode, http.StatusInternalServerError)
	assert.Equal(t, string(b), "this is an error")
}

func TestContext_PanicWebHttpError(t *testing.T) {
	c := SpringEcho.New(web.ServerConfig{Port: 8080})
	c.GetMapping("/", func(ctx web.Context) {
		panic(&web.HttpError{
			Code:    http.StatusInternalServerError,
			Message: http.StatusText(http.StatusInternalServerError),
		})
	})
	go c.Start()
	defer c.Stop(context.Background())
	time.Sleep(10 * time.Millisecond)
	response, err := http.Get("http://127.0.0.1:8080/")
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()
	b, _ := ioutil.ReadAll(response.Body)
	fmt.Println(string(b))
	fmt.Println(response.Status)
	assert.Equal(t, response.StatusCode, http.StatusInternalServerError)
	assert.Equal(t, string(b), "Internal Server Error")
}

func TestContext_PathNotFound(t *testing.T) {
	c := SpringEcho.New(web.ServerConfig{Port: 8080})
	go c.Start()
	defer c.Stop(context.Background())
	time.Sleep(10 * time.Millisecond)
	response, err := http.Get("http://127.0.0.1:8080/not_found")
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()
	b, _ := ioutil.ReadAll(response.Body)
	fmt.Println(string(b))
	fmt.Println(response.Status)
	assert.Equal(t, response.StatusCode, http.StatusNotFound)
	assert.Equal(t, string(b), "404 page not found")
}

func TestContainer_Static(t *testing.T) {

	c := SpringEcho.New(web.ServerConfig{Port: 8080})
	go c.Start()
	defer c.Stop(context.Background())
	c.File("/", "testdata/public/a.txt")
	c.Static("/public", "testdata/public/")
	time.Sleep(10 * time.Millisecond)

	{
		response, err := http.Get("http://127.0.0.1:8080/public/a.txt")
		if err != nil {
			panic(err)
		}
		defer response.Body.Close()
		b, _ := ioutil.ReadAll(response.Body)
		fmt.Println(string(b))
		fmt.Println(response.Status)
		assert.Equal(t, response.StatusCode, http.StatusOK)
		assert.Equal(t, string(b), "hello world!")
	}

	{
		response, err := http.Get("http://127.0.0.1:8080/")
		if err != nil {
			panic(err)
		}
		defer response.Body.Close()
		b, _ := ioutil.ReadAll(response.Body)
		fmt.Println(string(b))
		fmt.Println(response.Status)
		assert.Equal(t, response.StatusCode, http.StatusOK)
		assert.Equal(t, string(b), "hello world!")
	}
}

//func TestI18N(t *testing.T) {
//
//	langMap := map[string]interface{}{
//		"hello": "Hello World!",
//	}
//
//	err := i18n.Register("zh", conf.Map(langMap))
//	assert.Nil(t, err)
//
//	c := SpringEcho.New(web.ServerConfig{Port: 8080})
//	c.AddFilter(web.FuncFilter(func(ctx web.Context, chain web.FilterChain) {
//		lang := ctx.Header("Accept-Language")
//		err := i18n.SetLanguage(ctx.Context(), lang)
//		web.ERROR.Panic(err).When(err != nil)
//		chain.Next(ctx)
//	}))
//	c.GetMapping("/:key", func(ctx web.Context) {
//		ctx.String(i18n.Get(ctx.Context(), ctx.PathParam("key")))
//	})
//
//	go c.Start()
//	defer c.Stop(context.Background())
//	time.Sleep(10 * time.Millisecond)
//
//	{
//		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080/hello", nil)
//		if err != nil {
//			t.Fatal(err)
//		}
//		req.Header.Set("Accept-Language", "zh")
//		response, err := http.DefaultClient.Do(req)
//		if err != nil {
//			t.Fatal(err)
//		}
//		defer response.Body.Close()
//		b, _ := ioutil.ReadAll(response.Body)
//		fmt.Println(string(b))
//		fmt.Println(response.Status)
//		assert.Equal(t, response.StatusCode, http.StatusOK)
//		assert.Equal(t, string(b), "Hello World!")
//	}
//}
//
//func TestMethodOverride(t *testing.T) {
//
//	c := SpringEcho.New(web.ServerConfig{Port: 8080})
//	c.AddFilter(web.NewMethodOverrideFilter(web.NewMethodOverrideConfig()))
//	c.GetMapping("/", func(ctx web.Context) { ctx.String("ok!") })
//
//	go c.Start()
//	defer c.Stop(context.Background())
//	time.Sleep(10 * time.Millisecond)
//
//	response, err := http.PostForm("http://127.0.0.1:8080/?_method=GET", nil)
//	if err != nil {
//		t.Fatal(err)
//	}
//	defer response.Body.Close()
//	b, _ := ioutil.ReadAll(response.Body)
//	fmt.Println(string(b))
//}
