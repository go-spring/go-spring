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

package app

import (
	"errors"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/go-spring/spring-boot"
	"github.com/go-spring/spring-logger"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

func init() {
	SpringBoot.RegisterBeanFn(NewMyModule, "${message}")
}

type MyModule struct {
	_ SpringBoot.ApplicationEvent `export:""`

	msg string
}

func NewMyModule(msg string) *MyModule {
	return &MyModule{
		msg: msg,
	}
}

func (m *MyModule) OnStartApplication(appCtx SpringBoot.ApplicationContext) {
	SpringLogger.Info("MyModule start")

	var e *MyModule
	appCtx.GetBean(&e)
	SpringLogger.Infof("event: %+v", e)

	appCtx.SafeGoroutine(Process)
}

func (m *MyModule) OnStopApplication(appCtx SpringBoot.ApplicationContext) {
	SpringLogger.Info("MyModule stop")
}

func Process() {
	defer SpringBoot.Exit()

	defer func() { SpringLogger.Info("go stop") }()
	SpringLogger.Info("go start")

	var m *MyModule
	SpringBoot.GetBean(&m)
	SpringLogger.Infof("process: %+v", m)

	time.Sleep(200 * time.Millisecond)

	if resp, err := http.Get("http://localhost:8080/api/ok"); err != nil {
		panic(err)
	} else {
		if body, e := ioutil.ReadAll(resp.Body); e != nil {
			panic(e)
		} else {
			SpringLogger.Infof("resp code=%d body=%s", resp.StatusCode, string(body))
			if string(body) != "ok" {
				panic(errors.New("error"))
			}
		}
	}

	if resp, err := http.Get("http://127.0.0.1:8080/swagger/doc.json"); err != nil {
		panic(err)
	} else {
		if body, e := ioutil.ReadAll(resp.Body); e != nil {
			panic(e)
		} else {
			SpringLogger.Infof("resp code=%d body=%s", resp.StatusCode, string(body))
		}
	}

	if resp, err := http.Get("http://localhost:8080/echo?str=echo"); err != nil {
		panic(err)
	} else {
		if body, e := ioutil.ReadAll(resp.Body); e != nil {
			panic(e)
		} else {
			SpringLogger.Infof("resp code=%d body=%s(echo add a \\n on text end)", resp.StatusCode, string(body))
			if string(body) != "{\"code\":200,\"msg\":\"SUCCESS\",\"data\":{\"echo\":\"echo echo\"}}\n" {
				panic(errors.New("error"))
			}
		}
	}

	if req, err := http.NewRequest("GET", "http://localhost:8080/api/func", nil); err != nil {
		panic(err)
	} else {
		auth := middleware.DefaultKeyAuthConfig.AuthScheme + " " + "key_auth"
		req.Header.Set(echo.HeaderAuthorization, auth)
		if resp, e := http.DefaultClient.Do(req); e != nil {
			panic(e)
		} else {
			if body, e0 := ioutil.ReadAll(resp.Body); e0 != nil {
				panic(e0)
			} else {
				SpringLogger.Infof("resp code=%d body=%s", resp.StatusCode, string(body))
				if string(body) != "func() return ok" {
					panic(errors.New("error"))
				}
			}
		}
	}

	if resp, err := http.Get("http://127.0.0.1:8080/static/config/config-map.yaml"); err != nil {
		panic(err)
	} else {
		if body, e := ioutil.ReadAll(resp.Body); e != nil {
			panic(e)
		} else {
			SpringLogger.Infof("resp code=%d body=%s", resp.StatusCode, string(body))
		}
	}
}
