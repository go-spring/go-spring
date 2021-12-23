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

package main

import (
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
	"github.com/go-spring/spring-echo"
	"github.com/go-spring/starter-core"
	_ "github.com/go-spring/starter-web"
)

var config struct {
	Help bool
	Host string
	Port int
}

func init() {
	flag.BoolVar(&config.Help, "help", false, "print help message")
	flag.StringVar(&config.Host, "host", "", "set file server's host")
	flag.StringVar(&config.Host, "h", "", "set file server's host")
	flag.IntVar(&config.Port, "port", 0, "set file server's port")
	flag.IntVar(&config.Port, "p", 0, "set file server's port")
}

func init() {
	gs.Provide(func(config StarterCore.WebServerConfig) web.Container {
		c := SpringEcho.NewContainer(web.ContainerConfig(config))
		c.SetLoggerFilter(web.FuncFilter(func(ctx web.Context, chain web.FilterChain) {
			start := time.Now()
			chain.Next(ctx)
			cost := time.Since(start)
			req := ctx.Request()
			resp := ctx.ResponseWriter()
			now := time.Now().Format("2006-01-02 15:04:05.000 -0700")
			fmt.Printf("[%s] %s %s %s %d %d %s\n",
				now, req.Method, req.RequestURI, cost, resp.Size(), resp.Status(), req.UserAgent())
		}))
		return c
	})
}

func main() {
	flag.Parse()

	if config.Help {
		fmt.Println(`fileserver [-h 0.0.0.0] [-p 8080]
[usage]
    -host   set file server's host
    -h      set file server's host
    -port   set file server's port
    -p      set file server's port
    -help   print help message`)
		return
	}

	if config.Host != "" {
		gs.Property("web.server.ip", config.Host)
	}

	if config.Port > 0 {
		gs.Property("web.server.port", config.Port)
	}

	gs.HandleGet("/*", web.WrapH(http.FileServer(http.Dir("."))))
	fmt.Println(gs.Run())
}
