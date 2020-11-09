package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/go-spring/spring-boot"
	"github.com/go-spring/spring-logger"
	"github.com/go-spring/spring-web"
	_ "github.com/go-spring/starter-echo"
	//_ "github.com/go-spring/starter-gin"
)

type request struct{}

func init() {

	SpringBoot.GetMapping("/mapping/json/error",
		func(webCtx SpringWeb.WebContext) {
			webCtx.JSON(SpringWeb.ERROR.Error(errors.New("this is an error")))
		})

	SpringBoot.GetMapping("/mapping/json/success",
		func(webCtx SpringWeb.WebContext) {
			webCtx.JSON(SpringWeb.SUCCESS.Data("ok"))
		})

	SpringBoot.GetMapping("/mapping/panic/error", func(webCtx SpringWeb.WebContext) {
		panic(errors.New("this is an error"))
	})

	SpringBoot.GetMapping("/mapping/panic/rpc_result", func(webCtx SpringWeb.WebContext) {
		panic(SpringWeb.ERROR.Error(errors.New("this is a rpc_result")))
	})

	SpringBoot.GetBinding("/binding/json/error",
		func(ctx context.Context, req *request) *SpringWeb.RpcResult {
			return SpringWeb.ERROR.Error(errors.New("this is an error"))
		})

	SpringBoot.GetBinding("/binding/json/success",
		func(ctx context.Context, req *request) *SpringWeb.RpcResult {
			return SpringWeb.SUCCESS.Data("ok")
		})

	SpringBoot.GetBinding("/binding/panic/error",
		func(ctx context.Context, req *request) *SpringWeb.RpcResult {
			panic(errors.New("this is an error"))
		})

	SpringBoot.GetBinding("/binding/panic/rpc_result",
		func(ctx context.Context, req *request) *SpringWeb.RpcResult {
			err := errors.New("this is a rpc_result")
			// SpringWeb.ERROR.Panic(err).When(true)
			panic(SpringWeb.ERROR.Error(err))
		})
}

func read(response *http.Response, err error, expected string) {
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()
	b, _ := ioutil.ReadAll(response.Body)
	fmt.Println("status:", response.Status, "body:", string(b))
	if string(b) != expected {
		SpringLogger.Errorf("get %s but want %s", string(b), expected)
	}
}

func get(url string, expected string) {
	response, err := http.Get(url)
	read(response, err, expected)
}

func postForm(url string, expected string) {
	response, err := http.PostForm(url, nil)
	read(response, err, expected)
}

func main() {
	go func() {
		time.Sleep(20 * time.Millisecond)
		get("http://127.0.0.1:8080/404", `404 page not found`)
		postForm("http://127.0.0.1:8080/mapping/json/error", `405 method not allowed`)
		get("http://127.0.0.1:8080/mapping/json/error", `{"code":-1,"msg":"ERROR","err":"/Users/didi/GitHub/go-spring/go-spring/examples/spring-boot-web/main.go:24: this is an error"}`)
		get("http://127.0.0.1:8080/mapping/json/success", `{"code":200,"msg":"SUCCESS","data":"ok"}`)
		get("http://127.0.0.1:8080/mapping/panic/error", `this is an error`)
		get("http://127.0.0.1:8080/mapping/panic/rpc_result", `{"code":-1,"msg":"ERROR","err":"/Users/didi/GitHub/go-spring/go-spring/examples/spring-boot-web/main.go:37: this is a rpc_result"}`)
		get("http://127.0.0.1:8080/binding/json/error", `{"code":-1,"msg":"ERROR","err":"/Users/didi/GitHub/go-spring/go-spring/examples/spring-boot-web/main.go:42: this is an error"}`)
		get("http://127.0.0.1:8080/binding/json/success", `{"code":200,"msg":"SUCCESS","data":"ok"}`)
		get("http://127.0.0.1:8080/binding/panic/error", `this is an error`)
		get("http://127.0.0.1:8080/binding/panic/rpc_result", `{"code":-1,"msg":"ERROR","err":"/Users/didi/GitHub/go-spring/go-spring/examples/spring-boot-web/main.go:59: this is a rpc_result"}`)
		SpringBoot.Exit()
	}()
	SpringBoot.RunApplication()
}
