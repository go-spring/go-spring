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

package jsonflow

import (
	"bytes"
	"strings"
	"testing"
)

func encodeToString(t *testing.T, fn func(Encoder) error) string {
	t.Helper()

	var buf bytes.Buffer
	if err := fn(NewEncoder(&buf)); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

func TestEncodeScalar(t *testing.T) {
	if got := encodeToString(t, func(e Encoder) error {
		return EncodeBool(e, true)
	}); got != "true" {
		t.Fatalf("got %s", got)
	}

	if got := encodeToString(t, func(e Encoder) error {
		return EncodeInt(e, int32(-12))
	}); got != "-12" {
		t.Fatalf("got %s", got)
	}

	if got := encodeToString(t, func(e Encoder) error {
		return EncodeUint(e, uint16(12))
	}); got != "12" {
		t.Fatalf("got %s", got)
	}

	if got := encodeToString(t, func(e Encoder) error {
		return EncodeString(e, "hello")
	}); got != `"hello"` {
		t.Fatalf("got %s", got)
	}

	if got := encodeToString(t, func(e Encoder) error {
		return EncodeFloat(e, float32(0.1))
	}); got != `0.1` {
		t.Fatalf("got %s", got)
	}

	if got := encodeToString(t, func(e Encoder) error {
		return EncodeFloat(e, float64(1.23456789012345))
	}); got != `1.23456789012345` {
		t.Fatalf("got %s", got)
	}
}

func TestEncodePointer(t *testing.T) {
	if got := encodeToString(t, func(e Encoder) error {
		return EncodeStringPtr[string](e, nil)
	}); got != "null" {
		t.Fatalf("got %s", got)
	}

	s := "hello"
	if got := encodeToString(t, func(e Encoder) error {
		return EncodeStringPtr(e, &s)
	}); got != `"hello"` {
		t.Fatalf("got %s", got)
	}
}

func TestEncodeBytes(t *testing.T) {
	if got := encodeToString(t, func(e Encoder) error {
		return EncodeBytes(e, []byte("hello"))
	}); got != `"aGVsbG8="` {
		t.Fatalf("got %s", got)
	}

	if got := encodeToString(t, func(e Encoder) error {
		return EncodeBytes(e, nil)
	}); got != "null" {
		t.Fatalf("got %s", got)
	}
}

func TestEncodeArray(t *testing.T) {
	if got := encodeToString(t, func(e Encoder) error {
		return EncodeArray(EncodeInt[int])(e, []int{1, 2, 3})
	}); got != `[1,2,3]` {
		t.Fatalf("got %s", got)
	}

	if got := encodeToString(t, func(e Encoder) error {
		return EncodeArray(EncodeInt[int])(e, nil)
	}); got != "null" {
		t.Fatalf("got %s", got)
	}
}

func TestEncodeMap(t *testing.T) {
	if got := encodeToString(t, func(e Encoder) error {
		return EncodeMap(EncodeStringKey[string], EncodeInt[int])(e, map[string]int{
			"b": 2,
			"a": 1,
		})
	}); got != `{"a":1,"b":2}` {
		t.Fatalf("got %s", got)
	}

	if got := encodeToString(t, func(e Encoder) error {
		return EncodeMap(EncodeIntKey[int], EncodeString[string])(e, map[int]string{
			2: "b",
			1: "a",
		})
	}); got != `{"1":"a","2":"b"}` {
		t.Fatalf("got %s", got)
	}

	if got := encodeToString(t, func(e Encoder) error {
		return EncodeMap(EncodeUintKey[uint], EncodeString[string])(e, map[uint]string{
			2: "b",
			1: "a",
		})
	}); got != `{"1":"a","2":"b"}` {
		t.Fatalf("got %s", got)
	}
}

type encodedObject struct {
	Name string
}

func (o *encodedObject) EncodeJSON(e Encoder) error {
	if o == nil {
		return EncodeNull(e)
	}
	if err := EncodeObjectBegin(e); err != nil {
		return err
	}
	if err := EncodeString(e, "name"); err != nil {
		return err
	}
	if err := EncodeString(e, o.Name); err != nil {
		return err
	}
	return EncodeObjectEnd(e)
}

func (o *encodedObject) DecodeJSON(d Decoder) error {
	return nil
}

func TestEncodeObject(t *testing.T) {
	if got := encodeToString(t, func(e Encoder) error {
		return EncodeObject(e, &encodedObject{Name: "alice"})
	}); got != `{"name":"alice"}` {
		t.Fatalf("got %s", got)
	}

	if got := encodeToString(t, func(e Encoder) error {
		return EncodeObject[*encodedObject](e, nil)
	}); got != "null" {
		t.Fatalf("got %s", got)
	}
}

func TestMarshalWriteUsesEncodeJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := MarshalWrite(&buf, &encodedObject{Name: "alice"}); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != `{"name":"alice"}` {
		t.Fatalf("got %s", got)
	}
}

func TestMarshalUsesEncodeJSON(t *testing.T) {
	b, err := Marshal(&encodedObject{Name: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(b); got != `{"name":"alice"}` {
		t.Fatalf("got %s", got)
	}
}

type streamingDecodedObject struct {
	called bool
}

func (o *streamingDecodedObject) EncodeJSON(e Encoder) error {
	return nil
}

func (o *streamingDecodedObject) DecodeJSON(d Decoder) error {
	o.called = true
	return d.SkipValue()
}

func TestUnmarshalUsesDecodeJSON(t *testing.T) {
	var o streamingDecodedObject
	if err := Unmarshal([]byte(`{"name":"alice"}`), &o); err != nil {
		t.Fatal(err)
	}
	if !o.called {
		t.Fatal("DecodeJSON was not called")
	}
}

func TestUnmarshalDecodeJSONRejectsTrailingTokens(t *testing.T) {
	var o streamingDecodedObject
	if err := Unmarshal([]byte(`{"name":"alice"} true`), &o); err == nil {
		t.Fatal("expected trailing token error")
	}
}
