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
	"bytes"
	"testing"

	"examples/proto"

	"github.com/go-spring/stdlib/jsonflow"
)

func ptr[T any](v T) *T {
	return &v
}

func TestGeneratedEncodeJSON(t *testing.T) {
	m := &proto.Manager{
		Name: ptr("Jim"),
		Age:  ptr[int64](20),
		Vip:  ptr(false),
		Tags: map[string]string{
			"b": "2",
			"a": "1",
		},
		Labels: map[string][]string{
			"x": {"a", "b"},
		},
	}

	var buf bytes.Buffer
	if err := jsonflow.MarshalWrite(&buf, m); err != nil {
		t.Fatal(err)
	}

	want := `{"name":"Jim","age":20,"vip":false,"tags":{"a":"1","b":"2"},"labels":{"x":["a","b"]}}`
	if got := buf.String(); got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestGeneratedOneOfEncodeJSON(t *testing.T) {
	p := &proto.Payload{
		FieldType: proto.PayloadTypeAsString(proto.PayloadType_MessageDelta),
		MessageDelta: &proto.MessageDelta{
			Content: ptr("hello"),
			IsFinal: ptr(false),
		},
	}

	var buf bytes.Buffer
	if err := jsonflow.MarshalWrite(&buf, p); err != nil {
		t.Fatal(err)
	}

	want := `{"FieldType":"MessageDelta","MessageDelta":{"content":"hello","isFinal":false}}`
	if got := buf.String(); got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestGeneratedOneOfEncodeJSONMismatch(t *testing.T) {
	p := &proto.Payload{
		FieldType: proto.PayloadTypeAsString(proto.PayloadType_ToolCall),
		MessageDelta: &proto.MessageDelta{
			Content: ptr("hello"),
		},
	}

	var buf bytes.Buffer
	if err := jsonflow.MarshalWrite(&buf, p); err == nil {
		t.Fatal("expected oneof mismatch error")
	}
}

func TestGeneratedDecodeBytesNull(t *testing.T) {
	var image proto.ImageData
	if err := jsonflow.Unmarshal([]byte(`{"data":null}`), &image); err != nil {
		t.Fatal(err)
	}
	if image.Data != nil {
		t.Fatalf("got %v, want nil", image.Data)
	}
}

func TestGeneratedDecodeRejectsTrailingTokens(t *testing.T) {
	var m proto.Manager
	if err := jsonflow.Unmarshal([]byte(`{"name":"Jim"} true`), &m); err == nil {
		t.Fatal("expected trailing token error")
	}
}
