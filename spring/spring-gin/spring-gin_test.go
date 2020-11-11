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

	"github.com/go-spring/spring-gin"
	"github.com/go-spring/spring-web"
	"github.com/magiconair/properties/assert"
)

func TestContext_PanicSysError(t *testing.T) {
	c := SpringGin.NewContainer(SpringWeb.ContainerConfig{Port: 8080})
	c.GetMapping("/", func(webCtx SpringWeb.WebContext) {
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
	c.Start()
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
	c := SpringGin.NewContainer(SpringWeb.ContainerConfig{Port: 8080})
	c.GetMapping("/", func(webCtx SpringWeb.WebContext) {
		panic("this is an error")
	})
	c.Start()
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
	assert.Equal(t, string(b), `"this is an error"`)
}

func TestContext_PanicError(t *testing.T) {
	c := SpringGin.NewContainer(SpringWeb.ContainerConfig{Port: 8080})
	c.GetMapping("/", func(webCtx SpringWeb.WebContext) {
		panic(errors.New("this is an error"))
	})
	c.Start()
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
	assert.Equal(t, string(b), `this is an error`)
}

func TestContext_PanicWebHttpError(t *testing.T) {
	c := SpringGin.NewContainer(SpringWeb.ContainerConfig{Port: 8080})
	c.GetMapping("/", func(webCtx SpringWeb.WebContext) {
		panic(&SpringWeb.HttpError{
			Code:    http.StatusNotFound,
			Message: http.StatusText(http.StatusNotFound),
		})
	})
	c.Start()
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

type dummyFilter struct{}

func (f *dummyFilter) Invoke(webCtx SpringWeb.WebContext, chain SpringWeb.FilterChain) {
	panic(&SpringWeb.HttpError{
		Code:    http.StatusMethodNotAllowed,
		Message: http.StatusText(http.StatusMethodNotAllowed),
	})
}

func TestFilter_PanicWebHttpError(t *testing.T) {
	c := SpringGin.NewContainer(SpringWeb.ContainerConfig{Port: 8080})
	c.GetMapping("/", func(webCtx SpringWeb.WebContext) {
		webCtx.String("OK!")
	}, &dummyFilter{})
	c.Start()
	defer c.Stop(context.Background())
	time.Sleep(10 * time.Millisecond)
	response, err := http.Get("http://127.0.0.1:8080/")
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()
	b, _ := ioutil.ReadAll(response.Body)
	fmt.Println(response.Status, string(b))
	assert.Equal(t, response.StatusCode, http.StatusMethodNotAllowed)
}
