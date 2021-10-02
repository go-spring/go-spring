package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
	_ "github.com/go-spring/starter-echo"
	//_ "github.com/go-spring/starter-gin"
)

type request struct{}

func init() {

	gs.GetMapping("/mapping/json/error",
		func(webCtx web.Context) {
			webCtx.JSON(web.ERROR.Error(errors.New("this is an error")))
		})

	gs.GetMapping("/mapping/json/success",
		func(webCtx web.Context) {
			webCtx.JSON(web.SUCCESS.Data("ok"))
		})

	gs.GetMapping("/mapping/panic/error", func(webCtx web.Context) {
		panic(errors.New("this is an error"))
	})

	gs.GetMapping("/mapping/panic/rpc_result", func(webCtx web.Context) {
		panic(web.ERROR.Error(errors.New("this is a rpc_result")))
	})

	gs.GetBinding("/binding/json/error",
		func(ctx context.Context, req *request) *web.RpcResult {
			return web.ERROR.Error(errors.New("this is an error"))
		})

	gs.GetBinding("/binding/json/success",
		func(ctx context.Context, req *request) *web.RpcResult {
			return web.SUCCESS.Data("ok")
		})

	gs.GetBinding("/binding/panic/error",
		func(ctx context.Context, req *request) *web.RpcResult {
			panic(errors.New("this is an error"))
		})

	gs.GetBinding("/binding/panic/rpc_result",
		func(ctx context.Context, req *request) *web.RpcResult {
			err := errors.New("this is a rpc_result")
			// web.ERROR.Panic(err).When(true)
			panic(web.ERROR.Error(err))
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
		log.Errorf("get %s but want %s", string(b), expected)
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
		gs.ShutDown(errors.New("app run end"))
	}()
	fmt.Println(gs.Run())
}
