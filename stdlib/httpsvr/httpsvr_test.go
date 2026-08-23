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
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/hashutil"
	"go-spring.org/stdlib/httpsvr"
	"go-spring.org/stdlib/jsonflow"
	"go-spring.org/stdlib/testing/assert"
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

// DecodeForm decodes a form-encoded request body into the request object.
func (x *HelloRequest) DecodeForm(b []byte) (err error) {
	values, parseErr := url.ParseQuery(string(b))
	if parseErr != nil {
		return errutil.Explain(nil, "parse form error: %s", parseErr)
	}
	if v, ok := values["message"]; ok && len(v) == 1 {
		x.Message = v[0]
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
	return &HelloResponse{Message: new(req.Message)}
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
			Method:  "POST",
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

// startServer starts a SimpleServer on an ephemeral port and returns its base
// URL. No startup sleeps: the listener is bound before Serve runs, so client
// connections are queued by the kernel until the server goroutine picks them up.
func startServer(t *testing.T) string {
	t.Helper()

	svr := httpsvr.NewSimpleServer(":0")
	for _, r := range Routers(&HelloServerImpl{}, httpsvr.NewSimpleContext) {
		svr.Route(r)
	}

	l, err := net.Listen("tcp", "127.0.0.1:0")
	assert.Error(t, err).Nil()
	t.Cleanup(func() { _ = svr.Shutdown(context.Background()) })
	go func() { _ = svr.Serve(l) }()
	return "http://" + l.Addr().String()
}

func getBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	assert.Error(t, err).Nil()
	_ = resp.Body.Close()
	return string(b)
}

func TestHello(t *testing.T) {
	baseURL := startServer(t)

	resp, err := http.Get(baseURL + "/v1/hello?message=world")
	assert.Error(t, err).Nil()
	assert.That(t, resp.StatusCode).Equal(http.StatusOK)
	assert.That(t, resp.Header.Get("Content-Type")).Equal("application/json")
	assert.That(t, getBody(t, resp)).Equal(`{"message":"world"}`)
}

func TestHelloInvalidJSONBody(t *testing.T) {
	baseURL := startServer(t)

	resp, err := http.Post(baseURL+"/v1/hello", "application/json",
		strings.NewReader(`{"message":`)) // truncated JSON
	assert.Error(t, err).Nil()
	assert.That(t, resp.StatusCode).Equal(http.StatusInternalServerError)
	assert.String(t, getBody(t, resp)).Contains("json decode error")
}

func TestHelloFormBody(t *testing.T) {
	baseURL := startServer(t)

	// A form-encoded body goes through DecodeForm; Bind runs afterwards and
	// rebinds from the query string, so the message must agree in both places.
	resp, err := http.Post(baseURL+"/v1/hello?message=form-world",
		"application/x-www-form-urlencoded", strings.NewReader("message=form-world"))
	assert.Error(t, err).Nil()
	assert.That(t, resp.StatusCode).Equal(http.StatusOK)
	assert.That(t, getBody(t, resp)).Equal(`{"message":"form-world"}`)
}

func TestStream(t *testing.T) {
	baseURL := startServer(t)

	resp, err := http.Get(baseURL + "/v1/stream?message=world")
	assert.Error(t, err).Nil()
	assert.That(t, resp.StatusCode).Equal(http.StatusOK)
	assert.That(t, resp.Header.Get("Content-Type")).Equal("text/event-stream")

	body := getBody(t, resp)

	// Each event is one SSE frame: a "data: " line terminated by a blank line,
	// otherwise spec-compliant clients never dispatch the event.
	assert.That(t, strings.Count(body, "data: \"world\"\n\n")).Equal(5)
}

func TestRequestContext(t *testing.T) {
	// GetRequestContext returns nil when no RequestContext was stored.
	assert.That(t, httpsvr.GetRequestContext(context.Background())).Nil()

	// Drive the request through a ServeMux pattern so PathValue is populated.
	var rc httpsvr.RequestContext
	var gotW http.ResponseWriter
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		gotW = w
		rc = httpsvr.NewSimpleContext(r, w)
	})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/users/42", nil))

	assert.String(t, rc.PathValue("id")).Equal("42")
	assert.That(t, rc.Request().URL.Path).Equal("/v1/users/42")
	assert.That(t, rc.ResponseWriter()).Equal(gotW)

	ctx := httpsvr.WithRequestContext(context.Background(), rc)
	assert.That(t, httpsvr.GetRequestContext(ctx)).Equal(rc)
}
