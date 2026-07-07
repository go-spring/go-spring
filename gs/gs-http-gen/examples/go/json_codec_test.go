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

	"go-spring.org/stdlib/jsonflow"
	"go-spring.org/stdlib/testing/assert"
)

func TestGeneratedEncodeJSON(t *testing.T) {
	m := &proto.Manager{
		Name: new("Jim"),
		Age:  new(int64(20)),
		Vip:  new(false),
		Tags: map[string]string{
			"b": "2",
			"a": "1",
		},
		Labels: map[string][]string{
			"x": {"a", "b"},
		},
	}

	var buf bytes.Buffer
	assert.Error(t, jsonflow.MarshalWrite(&buf, m)).Nil()

	want := `{"name":"Jim","age":20,"vip":false,"tags":{"a":"1","b":"2"},"labels":{"x":["a","b"]}}`
	assert.String(t, buf.String()).Equal(want)
}

func TestGeneratedOneOfEncodeJSON(t *testing.T) {
	p := &proto.Payload{
		FieldType: proto.PayloadTypeAsString(proto.PayloadType_MessageDelta),
		MessageDelta: &proto.MessageDelta{
			Content: new("hello"),
			IsFinal: new(false),
		},
	}

	var buf bytes.Buffer
	assert.Error(t, jsonflow.MarshalWrite(&buf, p)).Nil()

	want := `{"FieldType":"MessageDelta","MessageDelta":{"content":"hello","isFinal":false}}`
	assert.String(t, buf.String()).Equal(want)
}

func TestGeneratedOneOfEncodeJSONMismatch(t *testing.T) {
	p := &proto.Payload{
		FieldType: proto.PayloadTypeAsString(proto.PayloadType_ToolCall),
		MessageDelta: &proto.MessageDelta{
			Content: new("hello"),
		},
	}

	var buf bytes.Buffer
	assert.Error(t, jsonflow.MarshalWrite(&buf, p)).NotNil()
}

func TestGeneratedDecodeBytesNull(t *testing.T) {
	var image proto.ImageData
	assert.Error(t, jsonflow.Unmarshal([]byte(`{"data":null}`), &image)).Nil()
	assert.That(t, image.Data).Nil()
}

func TestGeneratedDecodeRejectsTrailingTokens(t *testing.T) {
	var m proto.Manager
	assert.Error(t, jsonflow.Unmarshal([]byte(`{"name":"Jim"} true`), &m)).NotNil()
}
