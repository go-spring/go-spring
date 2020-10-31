package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/go-spring/examples/spring-boot-web/rpc"
	"github.com/go-spring/spring-boot"
	"github.com/go-spring/spring-logger"
	"github.com/go-spring/spring-web"
	//_ "github.com/go-spring/starter-echo"
	_ "github.com/go-spring/starter-gin"
)

type request struct{}

func init() {

	SpringWeb.ErrorHandler = func(webCtx SpringWeb.WebContext, err *SpringWeb.HttpError) {
		defer func() {
			if r := recover(); r != nil {
				webCtx.LogError(r)
			}
		}()
		webCtx.JSON(http.StatusOK, rpc.ERROR.Error(fmt.Errorf("%v", err.Message)))
	}

	SpringBoot.GetMapping("/mapping/error",
		func(webCtx SpringWeb.WebContext) {
			webCtx.JSON(http.StatusOK, rpc.ERROR.Error(errors.New("this is an error")))
		})

	SpringBoot.GetMapping("/mapping/success",
		func(webCtx SpringWeb.WebContext) {
			webCtx.JSON(http.StatusOK, rpc.SUCCESS.Data("ok"))
		})

	SpringBoot.GetMapping("/mapping/panic", func(webCtx SpringWeb.WebContext) {
		panic(errors.New("this is a panic"))
	})

	SpringBoot.GetBinding("/binding/error",
		func(ctx context.Context, req *request) *rpc.RpcResult {
			return rpc.ERROR.Error(errors.New("this is an error"))
		})

	SpringBoot.GetBinding("/binding/success",
		func(ctx context.Context, req *request) *rpc.RpcResult {
			return rpc.SUCCESS.Data("ok")
		})

	SpringBoot.GetBinding("/binding/panic",
		func(ctx context.Context, req *request) *rpc.RpcResult {
			panic(errors.New("this is a panic"))
		})
}

func get(url string, expected string) {
	response, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()
	b, _ := ioutil.ReadAll(response.Body)
	fmt.Println(response.Status, string(b))
	if string(b) != expected {
		SpringLogger.Errorf("get %s but want %s", string(b), expected)
	}
}

func main() {
	go func() {
		time.Sleep(20 * time.Millisecond)
		get("http://127.0.0.1:8080/mapping/error", `{"code":-1,"msg":"ERROR","err":"this is an error"}`)
		get("http://127.0.0.1:8080/mapping/success", `{"code":200,"msg":"SUCCESS","data":"ok"}`)
		get("http://127.0.0.1:8080/mapping/panic", `{"code":-1,"msg":"ERROR","err":"this is a panic"}`)
		get("http://127.0.0.1:8080/binding/error", `{"code":-1,"msg":"ERROR","err":"this is an error"}`)
		get("http://127.0.0.1:8080/binding/success", `{"code":200,"msg":"SUCCESS","data":"ok"}`)
		get("http://127.0.0.1:8080/binding/panic", `{"code":-1,"msg":"ERROR","err":"this is a panic"}`)
		SpringBoot.Exit()
	}()
	SpringBoot.RunApplication()
}
