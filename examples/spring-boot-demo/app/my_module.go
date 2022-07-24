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
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-core/gs"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

var (
	logger = log.GetLogger()
)

func init() {
	gs.Provide(new(MyModule)).Export((*gs.AppEvent)(nil))
}

type MyModule struct {
	BasePath string `value:"${web.server.base-path:=}"`
}

func (m *MyModule) OnAppStart(ctx gs.Context) {
	logger.Info("MyModule start")
	ctx.Go(m.Process)
}

func (m *MyModule) OnAppStop(ctx context.Context) {
	logger.Info("MyModule stop")
}

func (m *MyModule) Process(ctx context.Context) {
	defer gs.ShutDown("run end")

	defer func() { logger.Info("go stop") }()
	logger.Info("go start")

	time.Sleep(200 * time.Millisecond)

	path := fmt.Sprintf("http://localhost:8080%s/api/ok", m.BasePath)
	if resp, err := http.Get(path); err != nil {
		panic(err)
	} else {
		if body, e := ioutil.ReadAll(resp.Body); e != nil {
			panic(e)
		} else {
			logger.Infof("resp code=%d body=%s", resp.StatusCode, string(body))
			if string(body) != "ok" {
				panic(errors.New("error"))
			}
		}
	}

	path = fmt.Sprintf("http://localhost:8080%s/swagger/doc.json", m.BasePath)
	if resp, err := http.Get(path); err != nil {
		panic(err)
	} else {
		if body, e := ioutil.ReadAll(resp.Body); e != nil {
			panic(e)
		} else {
			logger.Infof("resp code=%d body=%s", resp.StatusCode, string(body))
		}
	}

	path = fmt.Sprintf("http://localhost:8080%s/api/echo?str=echo", m.BasePath)
	if resp, err := http.Get(path); err != nil {
		panic(err)
	} else {
		if body, e := ioutil.ReadAll(resp.Body); e != nil {
			panic(e)
		} else {
			logger.Infof("resp code=%d body=%s", resp.StatusCode, string(body))
			if string(body) != "{\"code\":200,\"msg\":\"SUCCESS\",\"data\":{\"echo\":\"echo echo\"}}" {
				panic(errors.New("error"))
			}
		}
	}

	path = fmt.Sprintf("http://localhost:8080%s/api/func", m.BasePath)
	if req, err := http.NewRequest("GET", path, nil); err != nil {
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
				logger.Infof("resp code=%d body=%s", resp.StatusCode, string(body))
				if string(body) != "func() return ok" {
					panic(errors.New("error"))
				}
			}
		}
	}

	path = fmt.Sprintf("http://localhost:8080%s/static/config/banner.txt", m.BasePath)
	if resp, err := http.Get(path); err != nil {
		panic(err)
	} else {
		if body, e := ioutil.ReadAll(resp.Body); e != nil {
			panic(e)
		} else {
			logger.Infof("resp code=%d body=(banner.txt)\n%s", resp.StatusCode, string(body))
		}
	}

	path = fmt.Sprintf("http://localhost:8080%s/static/html/hello.html", m.BasePath)
	if resp, err := http.Get(path); err != nil {
		panic(err)
	} else {
		if body, e := ioutil.ReadAll(resp.Body); e != nil {
			panic(e)
		} else {
			logger.Infof("resp code=%d body=(hello.html)\n%s", resp.StatusCode, string(body))
		}
	}
}
