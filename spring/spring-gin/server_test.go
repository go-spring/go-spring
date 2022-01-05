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
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-core/web"
	"github.com/go-spring/spring-gin"
)

func TestContext_PanicSysError(t *testing.T) {
	c := SpringGin.New(web.ServerConfig{Port: 8080})
	c.GetMapping("/", func(webCtx web.Context) {
		panic(&net.OpError{
			Op:     "dial",
			Net:    "tcp",
			Source: net.Addr(nil),
			Err: &os.SyscallError{
				Err:     errors.New("broken pipe"),
				Syscall: "write",
			},
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
	fmt.Println(response.Status, string(b))
}

func TestContext_PanicString(t *testing.T) {
	c := SpringGin.New(web.ServerConfig{Port: 8080})
	c.AddFilter(web.FuncPrefilter(func(ctx web.Context, chain web.FilterChain) {
		log.Info("<<log>>")
		chain.Continue(ctx)
	}))
	c.GetMapping("/", func(webCtx web.Context) {
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
	fmt.Println(response.Status, string(b))
	assert.Equal(t, response.StatusCode, http.StatusOK)
	assert.Equal(t, string(b), "this is an error")
}

func TestContext_PanicError(t *testing.T) {
	c := SpringGin.New(web.ServerConfig{Port: 8080})
	c.GetMapping("/", func(webCtx web.Context) {
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
	fmt.Println(response.Status, string(b))
	assert.Equal(t, response.StatusCode, http.StatusInternalServerError)
	assert.Equal(t, string(b), "this is an error")
}

func TestContext_PanicWebHttpError(t *testing.T) {
	c := SpringGin.New(web.ServerConfig{Port: 8080})
	c.GetMapping("/", func(webCtx web.Context) {
		panic(&web.HttpError{
			Code:    http.StatusNotFound,
			Message: http.StatusText(http.StatusNotFound),
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
	fmt.Println(response.Status, string(b))
	assert.Equal(t, response.StatusCode, http.StatusNotFound)
}

func TestFilter_PanicWebHttpError(t *testing.T) {
	c := SpringGin.New(web.ServerConfig{Port: 8080})
	go c.Start()
	defer c.Stop(context.Background())
	time.Sleep(10 * time.Millisecond)
	response, err := http.Get("http://127.0.0.1:8080/")
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()
	b, _ := ioutil.ReadAll(response.Body)
	fmt.Println(response.Status, string(b))
	assert.Equal(t, response.StatusCode, http.StatusNotFound)
}

func TestFilter_Abort(t *testing.T) {
	c := SpringGin.New(web.ServerConfig{Port: 8080})
	c.AddFilter(web.FuncFilter(func(ctx web.Context, chain web.FilterChain) {
		if ctx.FormValue("filter") == "1" {
			ctx.String("1")
			return
		}
		chain.Next(ctx)
	}))
	c.AddFilter(SpringGin.Filter(func(ctx *gin.Context) {
		if ctx.Request.FormValue("filter") == "2" {
			ctx.String(200, "2")
			return
		}
		ctx.Next()
	}))
	c.AddFilter(web.FuncFilter(func(ctx web.Context, chain web.FilterChain) {
		if ctx.FormValue("filter") == "p1" {
			panic("p1")
		}
		chain.Next(ctx)
	}))
	c.AddFilter(web.FuncFilter(func(ctx web.Context, chain web.FilterChain) {
		if ctx.FormValue("filter") == "3" {
			ctx.String("3")
			return
		}
		chain.Next(ctx)
	}))
	c.AddFilter(SpringGin.Filter(func(ctx *gin.Context) {
		if ctx.Request.FormValue("filter") == "p2" {
			panic("p2")
		}
		ctx.Next()
	}))
	c.AddFilter(SpringGin.Filter(func(ctx *gin.Context) {
		if ctx.Request.FormValue("filter") == "4" {
			ctx.String(200, "4")
			return
		}
		ctx.Next()
	}))
	c.GetMapping("/index", func(ctx web.Context) {
		ctx.String("ok")
	})

	go c.Start()
	defer c.Stop(context.Background())
	time.Sleep(10 * time.Millisecond)

	testFunc := func(path, expectBody string, expectCode int) {
		response, err := http.Get(path)
		assert.Nil(t, err)
		defer response.Body.Close()
		b, _ := ioutil.ReadAll(response.Body)
		assert.Equal(t, string(b), expectBody)
		assert.Equal(t, response.StatusCode, expectCode)
	}

	testFunc("http://127.0.0.1:8080/index", "ok", 200)
	testFunc("http://127.0.0.1:8080/index?filter=1", "1", 200)
	testFunc("http://127.0.0.1:8080/index?filter=2", "2", 200)
	testFunc("http://127.0.0.1:8080/index?filter=3", "3", 200)
	testFunc("http://127.0.0.1:8080/index?filter=4", "4", 200)
	testFunc("http://127.0.0.1:8080/index?filter=p1", "p1", 200)
	testFunc("http://127.0.0.1:8080/index?filter=p2", "p2", 200)
}

func TestContainer_Static(t *testing.T) {

	c := SpringGin.New(web.ServerConfig{Port: 8080})
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
