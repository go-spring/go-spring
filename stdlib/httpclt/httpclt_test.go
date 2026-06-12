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

package httpclt_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"go-spring.org/stdlib/hashutil"
	"go-spring.org/stdlib/httpclt"
	"go-spring.org/stdlib/jsonflow"
	"go-spring.org/stdlib/testing/assert"
)

func init() {
	doRequest := httpclt.DoRequest
	httpclt.DoRequest = func(r *http.Request, meta httpclt.Metadata, fn func(io.Reader) error) (*http.Response, error) {
		fmt.Printf("%#v\n", meta)
		return doRequest(r, meta, fn)
	}
}

type HelloRequest struct {
	HelloRequestBody
	Message string `json:"message" query:"message" validate:"required"`
}

func (x *HelloRequest) QueryForm() (string, error) {
	m := make(url.Values)
	m.Add("message", x.Message)
	return m.Encode(), nil
}

type HelloRequestBody struct{}

type HelloResponse struct {
	Message *string `json:"message,omitempty" form:"message"`
}

func NewHelloResponse() *HelloResponse {
	return &HelloResponse{}
}

func (r *HelloResponse) DecodeJSON(d jsonflow.Decoder) (err error) {
	const (
		hashMessage = 0x546401b5d2a8d2a4 // HashKey("message")
	)

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
		case hashMessage:
			if r.Message, err = jsonflow.DecodeStringPtr(d); err != nil {
				return err
			}
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

type HelloClient struct {
	ServiceName string
}

// Hello sends a GET request to the /v1/hello endpoint with the given request body.
func (c *HelloClient) Hello(ctx context.Context, req *HelloRequest, opts ...httpclt.RequestOption) (*http.Response, *HelloResponse, error) {
	meta := httpclt.CombineMetadata(httpclt.Metadata{
		Target:  c.ServiceName,
		Schema:  "http",
		Method:  "GET",
		Pattern: "/v1/hello",
		// nolint: staticcheck
		RawPath: fmt.Sprintf("/v1/hello"),
		Query:   req,
		Header: http.Header{
			"Content-Type": []string{"application/x-www-form-urlencoded"},
			"Accept":       []string{"application/json"},
		},
	}, opts...)
	//return httpclt.JSONResponse[*HelloResponse](ctx, meta)
	return httpclt.ObjectResponse(ctx, NewHelloResponse(), meta)
}

func TestHello(t *testing.T) {
	server := &http.Server{Addr: ":9090", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println(r.URL.RawQuery)
		_ = r.Header.Write(os.Stdout)
		_, _ = w.Write(fmt.Appendf(nil, `{"message": "hello %s"}`, r.URL.Query().Get("message")))
	})}

	var wg sync.WaitGroup
	wg.Go(func() {
		_ = server.ListenAndServe()
	})
	time.Sleep(time.Millisecond * 100)

	h := http.Header{}
	h.Set("X-Request-ID", "12345678")

	client := &HelloClient{ServiceName: "127.0.0.1:9090"}
	_, resp, err := client.Hello(context.Background(), &HelloRequest{
		Message: "world",
	}, httpclt.WithHeader(h))

	assert.Error(t, err).Nil()
	assert.That(t, resp).Equal(&HelloResponse{Message: new("hello world")})
	fmt.Println(resp.Message)

	_ = server.Shutdown(context.Background())
	wg.Wait()
}
