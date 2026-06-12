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

package gsmock_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/go-spring/gs-mock/gsmock"
	"github.com/go-spring/gs-mock/internal/assert"
)

type Request struct {
	Value int
}

type Response struct {
	Message string
}

// Get is a sample function that can be mocked by context.Context.
func Get(ctx context.Context, req *Request) (*Response, error) {
	return &Response{Message: "9:xxx"}, nil
}

func TestFuncMock(t *testing.T) {
	r := gsmock.NewManager()
	ctx := gsmock.WithManager(t.Context(), r)

	// Test case: Unmocked - should return default value
	{
		resp, err := Get(t.Context(), &Request{})
		assert.Nil(t, err)
		assert.Equal(t, resp.Message, "9:xxx")
	}

	// Test case: When && Return - should return mocked value when condition is met
	{
		r.Reset()
		gsmock.Func22(Get, r).
			When(func(ctx context.Context, req *Request) bool {
				return req.Value == 5
			}).
			Return(func() (resp *Response, err error) {
				return &Response{Message: "1:abc"}, nil
			})

		resp, err := Get(ctx, &Request{Value: 5})
		assert.Nil(t, err)
		assert.Equal(t, resp.Message, "1:abc")

		gsmock.Func22(Get, r).
			When(func(ctx context.Context, req *Request) bool {
				return req.Value == 10
			}).
			Return(func() (resp *Response, err error) {
				return &Response{Message: "3:xyz"}, nil
			})

		resp, err = Get(ctx, &Request{Value: 10})
		assert.Nil(t, err)
		assert.Equal(t, resp.Message, "3:xyz")

		resp, err = Get(ctx, &Request{Value: 15})
		assert.Nil(t, err)
		assert.Equal(t, resp.Message, "9:xxx")
	}

	// Test case: Handle - should handle all calls with the provided function
	{
		r.Reset()
		gsmock.Func22(Get, r).
			Handle(func(ctx context.Context, req *Request) (resp *Response, err error) {
				return &Response{Message: "6:xyz"}, nil
			})

		resp, err := Get(ctx, &Request{Value: 5})
		assert.Nil(t, err)
		assert.Equal(t, resp.Message, "6:xyz")
	}

	// Test case: Invalid Handle - should fall back to default implementation when handle is nil
	{
		r.Reset()
		gsmock.Func22(Get, r).Handle(nil)

		resp, err := Get(ctx, &Request{})
		assert.Nil(t, err)
		assert.Equal(t, resp.Message, "9:xxx")
	}

	// Test case: Mock that returns an error
	{
		r.Reset()
		gsmock.Func22(Get, r).
			When(func(ctx context.Context, req *Request) bool {
				return req.Value == 7
			}).
			Return(func() (resp *Response, err error) {
				return nil, context.DeadlineExceeded
			})

		resp, err := Get(ctx, &Request{Value: 7})
		assert.Equal(t, err, context.DeadlineExceeded)
		assert.Nil(t, resp)
	}
}

// Client is a sample client type for testing context-based mocking.
type Client struct {
	Value int
}

// Get is a method of Client that can be mocked by context.Context.
func (c *Client) Get(ctx context.Context, req *Request) (*Response, error) {
	return &Response{Message: "9:xxx"}, nil
}

func TestMethodMock(t *testing.T) {
	c1 := &Client{Value: 5}
	c2 := &Client{Value: 10}

	r := gsmock.NewManager()
	ctx := gsmock.WithManager(t.Context(), r)

	// Test case: Unmocked - should return default value
	{
		resp, err := c1.Get(t.Context(), &Request{})
		assert.Nil(t, err)
		assert.Equal(t, resp.Message, "9:xxx")
	}

	// Test case: When && Return - should return mocked value when condition is met
	{
		r.Reset()
		gsmock.Func32((*Client).Get, r).
			When(func(c *Client, ctx context.Context, req *Request) bool {
				return c.Value == 5
			}).
			Return(func() (resp *Response, err error) {
				return &Response{Message: "1:abc"}, nil
			})

		gsmock.Func32((*Client).Get, r).
			When(func(c *Client, ctx context.Context, req *Request) bool {
				return c.Value == 10
			}).
			Return(func() (resp *Response, err error) {
				return &Response{Message: "3:xyz"}, nil
			})

		resp, err := c1.Get(ctx, &Request{Value: 5})
		assert.Nil(t, err)
		assert.Equal(t, resp.Message, "1:abc")

		resp, err = c2.Get(ctx, &Request{Value: 5})
		assert.Nil(t, err)
		assert.Equal(t, resp.Message, "3:xyz")
	}

	// Test case: Handle - should handle all calls with the provided function
	{
		r.Reset()
		gsmock.Func32((*Client).Get, r).
			Handle(func(c *Client, ctx context.Context, req *Request) (resp *Response, err error) {
				return &Response{Message: "6:xyz"}, nil
			})

		resp, err := c1.Get(ctx, &Request{Value: 5})
		assert.Nil(t, err)
		assert.Equal(t, resp.Message, "6:xyz")
	}

	// Test case: Invalid Handle - should fall back to default implementation when handle is nil
	{
		r.Reset()
		gsmock.Func32((*Client).Get, r).Handle(nil)

		resp, err := c1.Get(ctx, &Request{})
		assert.Nil(t, err)
		assert.Equal(t, resp.Message, "9:xxx")
	}

	// Test case: Method mock that returns an error
	{
		r.Reset()
		gsmock.Func32((*Client).Get, r).
			When(func(c *Client, ctx context.Context, req *Request) bool {
				return c.Value == 5
			}).
			Return(func() (resp *Response, err error) {
				return nil, context.Canceled
			})

		resp, err := c1.Get(ctx, &Request{Value: 5})
		assert.Equal(t, err, context.Canceled)
		assert.Nil(t, resp)
	}
}

// ClientInterface is an interface for testing non-context based mocking.
type ClientInterface interface {
	Query(req *Request) (*Response, error)
}

// MockClient is a mock implementation of ClientInterface.
type MockClient struct {
	r *gsmock.Manager
}

// NewMockClient creates a new instance of MockClient.
func NewMockClient(r *gsmock.Manager) *MockClient {
	return &MockClient{r}
}

// Query mocks the Query method by invoking a registered mock implementation.
func (c *MockClient) Query(req *Request) (*Response, error) {
	if ret, ok := gsmock.Invoke(c.r, c, c.Query, req); ok {
		return gsmock.Unbox2[*Response, error](ret)
	}
	panic("no mock code matched for MockClient.Query")
}

// MockQuery registers a mock implementation for the Query method.
func (c *MockClient) MockQuery() *gsmock.Mocker12[*Request, *Response, error] {
	return gsmock.Method12(c, c.Query, c.r)
}

func TestInterfaceMock(t *testing.T) {
	r := gsmock.NewManager()

	var c ClientInterface
	mockClient := NewMockClient(r)
	c = mockClient

	// Test case: When && Return - should return mocked value when condition is met
	{
		r.Reset()
		mockClient.MockQuery().
			When(func(req *Request) bool {
				return req.Value == 5
			}).
			Return(func() (resp *Response, err error) {
				return &Response{Message: "1:abc"}, nil
			})

		mockClient.MockQuery().
			When(func(req *Request) bool {
				return req.Value == 10
			}).
			Return(func() (resp *Response, err error) {
				return &Response{Message: "3:xyz"}, nil
			})

		resp, err := c.Query(&Request{Value: 5})
		assert.Nil(t, err)
		assert.Equal(t, resp.Message, "1:abc")

		resp, err = c.Query(&Request{Value: 10})
		assert.Nil(t, err)
		assert.Equal(t, resp.Message, "3:xyz")

		assert.Panic(t, func() {
			_, _ = c.Query(&Request{Value: 15})
		}, "no mock code matched for MockClient.Query")
	}

	// Test case: Handle - should handle all calls with the provided function
	{
		r.Reset()
		mockClient.MockQuery().
			Handle(func(req *Request) (resp *Response, err error) {
				return &Response{Message: "6:xyz"}, nil
			})

		resp, err := c.Query(&Request{Value: 5})
		assert.Nil(t, err)
		assert.Equal(t, resp.Message, "6:xyz")
	}

	// Test case: Invalid Handle - should panic when handle is nil and no other mock matches
	{
		r.Reset()
		mockClient.MockQuery().Handle(nil)

		assert.Panic(t, func() {
			_, _ = c.Query(&Request{})
		}, "no mock code matched for MockClient.Query")
	}
}

func TestConcurrentMock(t *testing.T) {
	r := gsmock.NewManager()

	var c ClientInterface
	mockClient := NewMockClient(r)
	c = mockClient

	mockClient.MockQuery().
		When(func(req *Request) bool {
			return req.Value%2 == 0 // even numbers
		}).
		Return(func() (resp *Response, err error) {
			return &Response{Message: "even"}, nil
		})

	mockClient.MockQuery().
		When(func(req *Request) bool {
			return req.Value%2 == 1 // odd numbers
		}).
		Return(func() (resp *Response, err error) {
			return &Response{Message: "odd"}, nil
		})

	var wg sync.WaitGroup
	errs := make(chan error, 10)

	for i := range 10 {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			resp, err := c.Query(&Request{Value: val})
			if err != nil {
				errs <- err
				return
			}
			expected := "even"
			if val%2 == 1 {
				expected = "odd"
			}
			if resp.Message != expected {
				errs <- fmt.Errorf("expected %s, got %s", expected, resp.Message)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent test failed: %v", err)
		}
	}
}

func TestConcurrentDifferentManagers(t *testing.T) {
	var wg sync.WaitGroup

	for i := range 3 {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()

			r := gsmock.NewManager()

			var c ClientInterface
			mockClient := NewMockClient(r)
			c = mockClient

			mockClient.MockQuery().
				When(func(req *Request) bool {
					return req.Value == k
				}).
				Return(func() (resp *Response, err error) {
					return &Response{Message: "manager-" + string(rune('0'+k))}, nil
				})

			resp, err := c.Query(&Request{Value: k})
			assert.Nil(t, err)
			if resp == nil {
				t.Errorf("Expected non-nil response for manager %d", k)
			}
		}(i)
	}

	wg.Wait()
}
