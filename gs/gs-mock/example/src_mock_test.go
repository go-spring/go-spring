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

package example

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	exp "github.com/go-spring/gs-mock/example/inner"
	"github.com/go-spring/gs-mock/gsmock"
	"github.com/go-spring/gs-mock/internal/assert"
)

type ItemType int

func TestRepositoryMockImpl_FindByID(t *testing.T) {
	r := gsmock.NewManager()
	s := NewRepositoryMockImpl[ItemType](r)

	assert.Panic(t, func() {
		_, _ = s.FindByID("1")
	}, "no mock code matched for RepositoryMockImpl.FindByID")

	// Test parameter matching in handle and then return result
	s.MockFindByID().Handle(func(id string) (ItemType, error) {
		if id == "2" {
			return ItemType(777), nil
		}
		return ItemType(666), nil
	})

	v, err := s.FindByID("1")
	assert.Nil(t, err)
	assert.Equal(t, v, ItemType(666))

	v, err = s.FindByID("2")
	assert.Nil(t, err)
	assert.Equal(t, v, ItemType(777))

	v, err = s.FindByID("2")
	assert.Nil(t, err)
	assert.Equal(t, v, ItemType(777))

	// This mock is not effective because there is already a mock
	s.MockFindByID().Handle(func(s string) (ItemType, error) {
		return ItemType(555), nil
	})

	v, err = s.FindByID("2")
	assert.Nil(t, err)
	assert.Equal(t, v, ItemType(777))
}

func TestRepositoryMockImpl_Save(t *testing.T) {
	r := gsmock.NewManager()
	s1 := NewRepositoryMockImpl[ItemType](r)
	s2 := NewRepositoryMockImpl[ItemType](r)

	assert.Panic(t, func() {
		_ = s2.Save(ItemType(666))
	}, "no mock code matched for RepositoryMockImpl.Save")

	s1.MockSave().Handle(func(v ItemType) error {
		return errors.New("error")
	})

	err := s1.Save(ItemType(666))
	assert.Equal(t, err.Error(), "error")

	// Test that different interface instances return their own
	// results when mocking the same method
	s2.MockSave().Handle(func(v ItemType) error {
		return nil
	})

	err = s2.Save(ItemType(666))
	assert.Nil(t, err)
}

func TestGenericServiceMockImpl_Init(t *testing.T) {
	r := gsmock.NewManager()
	s := NewGenericServiceMockImpl[string, int](r)

	assert.Panic(t, func() {
		s.Init()
	}, "no mock code matched for GenericServiceMockImpl.Init")

	// Test mocking methods after generic interface instantiation
	s.MockInit().ReturnDefault()
	s.Init()
}

func TestGenericServiceMockImpl_Default(t *testing.T) {
	r := gsmock.NewManager()
	s := NewGenericServiceMockImpl[string, int](r)

	assert.Panic(t, func() {
		s.Default()
	}, "no mock code matched for GenericServiceMockImpl.Default")

	// Test mocking methods after generic interface instantiation
	s.MockDefault().ReturnValue(5)

	resp := s.Default()
	assert.Equal(t, resp, 5)
}

func TestGenericServiceMockImpl_TryDefault(t *testing.T) {
	r := gsmock.NewManager()
	s := NewGenericServiceMockImpl[string, int](r)

	assert.Panic(t, func() {
		s.TryDefault()
	}, "no mock code matched for GenericServiceMockImpl.TryDefault")

	s.MockTryDefault().Handle(func() (int, bool) {
		return 5, true
	})

	resp, ok := s.TryDefault()
	assert.Equal(t, ok, true)
	assert.Equal(t, resp, 5)
}

func TestGenericServiceMockImpl_Accept(t *testing.T) {
	r := gsmock.NewManager()
	s := NewGenericServiceMockImpl[string, int](r)

	assert.Panic(t, func() {
		s.Accept("")
	}, "no mock code matched for GenericServiceMockImpl.Accept")

	// Test mocking methods after generic interface instantiation
	s.MockAccept().Return(func() {})
	s.Accept("abc")
}

func TestGenericServiceMockImpl_Convert(t *testing.T) {
	r := gsmock.NewManager()
	s := NewGenericServiceMockImpl[string, int](r)

	assert.Panic(t, func() {
		s.Convert("")
	}, "no mock code matched for GenericServiceMockImpl.Convert")

	s.MockConvert().When(func(s string) bool {
		return s == "abc"
	}).Return(func() int {
		return 5
	})

	s.MockConvert().When(func(s string) bool {
		return s == "123"
	}).Return(func() int {
		return 10
	})

	resp := s.Convert("abc")
	assert.Equal(t, resp, 5)

	resp = s.Convert("123")
	assert.Equal(t, resp, 10)

	// Test when/then combination, if the first match succeeds,
	// return the result, otherwise return default result
	assert.Panic(t, func() {
		s.Convert("")
	}, "no mock code matched for GenericServiceMockImpl.Convert")
}

func TestGenericServiceMockImpl_TryConvert(t *testing.T) {
	r := gsmock.NewManager()
	s := NewGenericServiceMockImpl[string, int](r)

	assert.Panic(t, func() {
		s.TryConvert("")
	}, "no mock code matched for GenericServiceMockImpl.TryConvert")

	s.MockTryConvert().Handle(func(s string) (int, bool) {
		return 5, false
	})

	resp, ok := s.TryConvert("abc")
	assert.Equal(t, ok, false)
	assert.Equal(t, resp, 5)
}

func TestGenericServiceMockImpl_Process(t *testing.T) {
	r := gsmock.NewManager()
	s := NewGenericServiceMockImpl[string, int](r)

	assert.Panic(t, func() {
		_, _ = s.Process(context.Background(), map[string]string{})
	}, "no mock code matched for GenericServiceMockImpl.Process")

	s.MockProcess().Handle(func(ctx context.Context, m map[string]string) (int, error) {
		return 5, nil
	})

	// Test interface implementation method mock that does not depend on ctx
	resp, err := s.Process(context.Background(), map[string]string{})
	assert.Nil(t, err)
	assert.Equal(t, resp, 5)
}

func TestGenericServiceMockImpl_Printf(t *testing.T) {
	r := gsmock.NewManager()
	s := NewGenericServiceMockImpl[string, int](r)

	assert.Panic(t, func() {
		s.Printf("%s\n", "123")
	}, "no mock code matched for GenericServiceMockImpl.Printf")

	var buf bytes.Buffer
	s.MockPrintf().Handle(func(format string, args []any) {
		_, _ = fmt.Fprintf(&buf, format, args...)
	})

	// Test variadic parameter method mock
	s.Printf("%s:%s\n", "123", "456")
	assert.Equal(t, buf.String(), "123:456\n")
}

func TestServiceMockImpl_Init(t *testing.T) {
	r := gsmock.NewManager()
	s := NewServiceMockImpl(r)

	assert.Panic(t, func() {
		s.Init()
	}, "no mock code matched for ServiceMockImpl.Init")

	s.MockInit().ReturnDefault()
	s.Init()
}

func TestServiceMockImpl_Default(t *testing.T) {
	r := gsmock.NewManager()
	s := NewServiceMockImpl(r)

	assert.Panic(t, func() {
		s.Default()
	}, "no mock code matched for ServiceMockImpl.Default")

	s.MockDefault().Handle(func() *Response {
		return &Response{Value: 5}
	})

	resp := s.Default()
	assert.Equal(t, resp.Value, 5)
}

func TestServiceMockImpl_TryDefault(t *testing.T) {
	r := gsmock.NewManager()
	s := NewServiceMockImpl(r)

	assert.Panic(t, func() {
		s.TryDefault()
	}, "no mock code matched for ServiceMockImpl.TryDefault")

	s.MockTryDefault().Handle(func() (*Response, bool) {
		return &Response{Value: 5}, false
	})

	resp, ok := s.TryDefault()
	assert.Equal(t, ok, false)
	assert.Equal(t, resp.Value, 5)
}

func TestServiceMockImpl_Accept(t *testing.T) {
	r := gsmock.NewManager()
	s := NewServiceMockImpl(r)

	assert.Panic(t, func() {
		s.Accept(&exp.Request{})
	}, "no mock code matched for ServiceMockImpl.Accept")

	s.MockAccept().ReturnDefault()
	s.Accept(&exp.Request{})
}

func TestServiceMockImpl_Convert(t *testing.T) {
	r := gsmock.NewManager()
	s := NewServiceMockImpl(r)

	assert.Panic(t, func() {
		s.Convert(&exp.Request{})
	}, "no mock code matched for ServiceMockImpl.Convert")

	s.MockConvert().Handle(func(req *exp.Request) *Response {
		return &Response{Value: 5}
	})

	resp := s.Convert(&exp.Request{})
	assert.Equal(t, resp.Value, 5)
}

func TestServiceMockImpl_TryConvert(t *testing.T) {
	r := gsmock.NewManager()
	s := NewServiceMockImpl(r)

	assert.Panic(t, func() {
		s.TryConvert(&exp.Request{})
	}, "no mock code matched for ServiceMockImpl.TryConvert")

	s.MockTryConvert().Handle(func(req *exp.Request) (*Response, bool) {
		return &Response{Value: 5}, false
	})

	resp, ok := s.TryConvert(&exp.Request{})
	assert.Equal(t, ok, false)
	assert.Equal(t, resp.Value, 5)
}

func TestServiceMockImpl_Process(t *testing.T) {
	r := gsmock.NewManager()
	s := NewServiceMockImpl(r)

	assert.Panic(t, func() {
		_, _ = s.Process(context.Background(), map[string]*exp.Request{})
	}, "no mock code matched for ServiceMockImpl.Process")

	s.MockProcess().Handle(func(ctx context.Context, m map[string]*exp.Request) (*Response, error) {
		return &Response{Value: 5}, nil
	})

	resp, err := s.Process(context.Background(), map[string]*exp.Request{})
	assert.Nil(t, err)
	assert.Equal(t, resp.Value, 5)
}

func TestServiceMockImpl_Printf(t *testing.T) {
	r := gsmock.NewManager()
	s1 := NewServiceMockImpl(r)
	s2 := NewServiceMockImpl(r)

	assert.Panic(t, func() {
		s1.Printf("%s\n", "123")
	}, "no mock code matched for ServiceMockImpl.Printf")

	var buf bytes.Buffer
	s1.MockPrintf().Handle(func(format string, args []any) {
		_, _ = fmt.Fprintf(&buf, format, args...)
	})
	s2.MockPrintf().Handle(func(format string, args []any) {
		buf.WriteString("abc")
	})

	s1.Printf("%s:%s\n", "123", "456")
	s2.Printf("%s\n", "123")
	assert.Equal(t, buf.String(), "123:456\nabc")
}

func TestServiceMockImpl_Writer(t *testing.T) {
	r := gsmock.NewManager()
	s := NewServiceMockImpl(r)

	assert.Panic(t, func() {
		_, _ = s.Write([]byte("123"))
	}, "runtime error: invalid memory address or nil pointer dereference")

	buf := bytes.NewBuffer(nil)
	s.Writer = buf

	buf.Reset()
	_, _ = s.Write([]byte("abc"))
	assert.Equal(t, buf.String(), "abc")

	buf.Reset()
	_, _ = s.Write([]byte("123"))
	assert.Equal(t, buf.String(), "123")
}
