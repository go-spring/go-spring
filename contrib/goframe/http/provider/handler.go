/*
 * Copyright 2025 The Go-Spring Authors.
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
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"go-spring.org/spring/gs"
	goframehttp "go-spring.org/starter-goframe/http"
)

func init() {
	// Provide the starter's ServiceRegister bean that binds the HelloController
	// onto the response-wrapping router group. Importing the starter package
	// (goframehttp) triggers its module init, which registers the *ghttp.Server
	// as a gs.Server; this bean is the only wiring the application supplies —
	// the server lifecycle, log bridge and optional metrics all live in the
	// starter now (they used to be the deleted provider/server.go).
	gs.Provide(func() goframehttp.ServiceRegister {
		return func(group *ghttp.RouterGroup) {
			group.Bind(
				&HelloController{},
			)
		}
	})
}

// HelloReq / HelloRes are the request and response types for the hello route.
//
// A stock goframe project keeps these under api/hello/v*/ and generates the
// matching controller with `gf gen ctrl`. Here the handler is hand-written and
// self-contained in this file, so the example no longer depends on goframe's
// generated api/ + internal/controller/ tree — the g.Meta tag still drives
// goframe's route registration (GET /hello) and OpenAPI metadata.
type HelloReq struct {
	g.Meta `path:"/hello" tags:"Hello" method:"get" summary:"You first hello api"`
}

// HelloRes carries no fields; the handler writes the body directly. The
// text/html mime tag keeps the response type goframe-standard.
type HelloRes struct {
	g.Meta `mime:"text/html" example:"string"`
}

// HelloController is bound via group.Bind in server.go. goframe reflects over
// its methods whose signature is (ctx, *Req) (*Res, error) and wires each to
// the route declared on the request type's g.Meta tag.
type HelloController struct{}

// Hello writes a fixed body the consumer asserts on. Returning nil res is fine
// because the body is written directly onto the response. Logging through the
// request ctx makes glog stamp each line with the active span's trace-id, so
// the log entry correlates with the trace exported to Jaeger.
func (c *HelloController) Hello(ctx context.Context, req *HelloReq) (res *HelloRes, err error) {
	g.Log().Info(ctx, "handling hello request")
	g.RequestFromCtx(ctx).Response.Writeln("Hello World!")
	return
}
