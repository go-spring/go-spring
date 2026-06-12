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

package formutil_test

import (
	"net/url"
	"testing"

	"go-spring.org/stdlib/formutil"
	"go-spring.org/stdlib/testing/assert"
)

type Item struct {
	Name string
}

type Request struct {
	Bool            bool
	BoolPtr         *bool
	Int             int
	IntPtr          *int
	Int64           int64
	Int64Ptr        *int64
	Uint            uint
	UintPtr         *uint
	Uint64          uint64
	Uint64Ptr       *uint64
	Float32         float32
	Float32Ptr      *float32
	String          string
	StringPtr       *string
	Bytes           []byte
	Item            *Item
	StringList      []string
	StringPtrList   []*string
	ItemList        []*Item
	StringIntMap    map[string]int
	StringIntPtrMap map[string]*int
	StringItemMap   map[string]*Item
}

func (x *Request) EncodeForm() (string, error) {
	m := url.Values{}
	_ = formutil.EncodeBool(m, "Bool", x.Bool)
	_ = formutil.EncodeBoolPtr(m, "BoolPtr", x.BoolPtr)
	_ = formutil.EncodeInt(m, "Int", x.Int)
	_ = formutil.EncodeIntPtr(m, "IntPtr", x.IntPtr)
	_ = formutil.EncodeInt(m, "Int64", x.Int64)
	_ = formutil.EncodeIntPtr(m, "Int64Ptr", x.Int64Ptr)
	_ = formutil.EncodeUint(m, "Uint", x.Uint)
	_ = formutil.EncodeUintPtr(m, "UintPtr", x.UintPtr)
	_ = formutil.EncodeUint(m, "Uint64", x.Uint64)
	_ = formutil.EncodeUintPtr(m, "Uint64Ptr", x.Uint64Ptr)
	_ = formutil.EncodeFloat(m, "Float32", x.Float32)
	_ = formutil.EncodeFloatPtr(m, "Float32Ptr", x.Float32Ptr)
	_ = formutil.EncodeString(m, "String", x.String)
	_ = formutil.EncodeStringPtr(m, "StringPtr", x.StringPtr)
	_ = formutil.EncodeBytes(m, "Bytes", x.Bytes)
	if err := formutil.EncodeJSON(m, "Item", x.Item); err != nil {
		return "", err
	}
	if err := formutil.EncodeList(m, "StringList", x.StringList, formutil.EncodeString); err != nil {
		return "", err
	}
	if err := formutil.EncodeList(m, "StringPtrList", x.StringPtrList, formutil.EncodeStringPtr); err != nil {
		return "", err
	}
	if err := formutil.EncodeList(m, "ItemList", x.ItemList, formutil.EncodeJSON); err != nil {
		return "", err
	}
	if err := formutil.EncodeJSON(m, "StringIntMap", x.StringIntMap); err != nil {
		return "", err
	}
	if err := formutil.EncodeJSON(m, "StringIntPtrMap", x.StringIntPtrMap); err != nil {
		return "", err
	}
	if err := formutil.EncodeJSON(m, "StringItemMap", x.StringItemMap); err != nil {
		return "", err
	}
	return m.Encode(), nil
}

var ExpectedReq = Request{
	Bool:            true,
	BoolPtr:         new(true),
	Int:             1,
	IntPtr:          new(int(1)),
	Int64:           1,
	Int64Ptr:        new(int64(1)),
	Uint:            1,
	UintPtr:         new(uint(1)),
	Uint64:          1,
	Uint64Ptr:       new(uint64(1)),
	Float32:         1,
	Float32Ptr:      new(float32(1)),
	String:          "1",
	StringPtr:       new("1"),
	Bytes:           []byte("1"),
	Item:            &Item{Name: "1"},
	StringList:      []string{"1"},
	StringPtrList:   []*string{new("1")},
	ItemList:        []*Item{{Name: "1"}},
	StringIntMap:    map[string]int{"1": 1},
	StringIntPtrMap: map[string]*int{"1": new(1)},
	StringItemMap:   map[string]*Item{"1": &Item{Name: "1"}},
}

func TestEncode(t *testing.T) {
	form, err := ExpectedReq.EncodeForm()
	if err != nil {
		t.Fatal(err)
	}
	assert.String(t, form).Equal(ExpectedStr)
}
