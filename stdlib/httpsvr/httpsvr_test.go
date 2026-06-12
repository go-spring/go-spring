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

package httpsvr_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/hashutil"
	"go-spring.org/stdlib/httpsvr"
	"go-spring.org/stdlib/jsonflow"
)

type HelloRequest struct {
	HelloRequestBody
	Message string `json:"message" query:"message" validate:"required"`
}

func NewHelloRequest() *HelloRequest {
	return &HelloRequest{}
}

// Bind binds the request parameters to the request object.
func (x *HelloRequest) Bind(r *http.Request) (err error) {
	values, parseErr := url.ParseQuery(r.URL.RawQuery)
	if parseErr != nil {
		err = errutil.Explain(err, "parse query error: %s", parseErr)
		return
	}

	var (
		hasMessage bool
	)

	if v, ok := values["message"]; ok {
		hasMessage = true
		if len(v) == 1 {
			x.Message = v[0]
		} else {
			err = errutil.Explain(err, "invalid value for \"message\"")
		}
	}
	if !hasMessage {
		err = errutil.Explain(err, "missing required field \"message\"")
	}
	return
}

func (x *HelloRequest) Validate() (err error) {
	if validateErr := x.HelloRequestBody.Validate(); validateErr != nil {
		err = errutil.Stack(err, "validate failed on \"HelloRequest\": %s", validateErr)
	}
	return
}

type HelloRequestBody struct{}

func (x *HelloRequestBody) DecodeJSON(d jsonflow.Decoder) (err error) {

	if err = jsonflow.DecodeObjectBegin(d); err != nil {
		return err
	}

	for d.PeekKind() != '}' {

		var key string
		key, err = jsonflow.DecodeString(d)
		if err != nil {
			return err
		}

		switch hashutil.FNV1a64(key) {
		default:
			if err = d.SkipValue(); err != nil {
				return err
			}
		}
	}

	if err = jsonflow.DecodeObjectEnd(d); err != nil {
		return err
	}
	return
}

func (x *HelloRequestBody) Validate() (err error) {
	return
}

type HelloResponse struct {
	Message *string `json:"message,omitempty" form:"message"`
}

type HelloServer interface {
	Hello(context.Context, *HelloRequest) *HelloResponse
	Stream(context.Context, *HelloRequest, chan<- *httpsvr.Event[string])
}

type HelloServerImpl struct{}

func (s *HelloServerImpl) Hello(ctx context.Context, req *HelloRequest) *HelloResponse {
	return &HelloResponse{Message: new("")}
}

func (s *HelloServerImpl) Stream(ctx context.Context, req *HelloRequest, resp chan<- *httpsvr.Event[string]) {
	for range 5 {
		resp <- httpsvr.NewEvent[string]().Data(req.Message)
	}
}

// Routers returns a list of HTTP routers for the service.
func Routers(server HelloServer, fn httpsvr.NewRequestContext) []httpsvr.Router {
	return []httpsvr.Router{
		{
			Method:  "GET",
			Pattern: "/v1/hello",
			Handler: func(w http.ResponseWriter, r *http.Request) {
				ctx := httpsvr.WithRequestContext(r.Context(), fn(r, w))
				httpsvr.HandleJSON(w, r.WithContext(ctx), NewHelloRequest(), server.Hello)
			},
		},
		{
			Method:  "GET",
			Pattern: "/v1/stream",
			Handler: func(w http.ResponseWriter, r *http.Request) {
				ctx := httpsvr.WithRequestContext(r.Context(), fn(r, w))
				httpsvr.HandleStream(w, r.WithContext(ctx), NewHelloRequest(), server.Stream)
			},
		},
	}
}

func TestHello(t *testing.T) {
	svr := httpsvr.NewSimpleServer(":9191")
	for _, r := range Routers(&HelloServerImpl{}, httpsvr.NewSimpleContext) {
		svr.Route(r)
	}
	go func() {
		fmt.Println(svr.ListenAndServe())
	}()
	time.Sleep(time.Millisecond * 300)

	resp, err := http.Get("http://localhost:9191/v1/hello?message=world")
	if err != nil {
		panic(err)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	_ = resp.Body.Close()
	fmt.Println(string(b))
	_ = svr.Shutdown(t.Context())
}

func TestStream(t *testing.T) {
	svr := httpsvr.NewSimpleServer(":9191")
	for _, r := range Routers(&HelloServerImpl{}, httpsvr.NewSimpleContext) {
		svr.Route(r)
	}
	go func() {
		fmt.Println(svr.ListenAndServe())
	}()
	time.Sleep(time.Millisecond * 300)

	resp, err := http.Get("http://localhost:9191/v1/stream?message=world")
	if err != nil {
		panic(err)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1025))
	if err != nil {
		panic(err)
	}
	_ = resp.Body.Close()
	fmt.Print(string(b))
	_ = svr.Shutdown(t.Context())
}
