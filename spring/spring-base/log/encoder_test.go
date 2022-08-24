/*
 * Copyright 2012-2019 the original author or authors.
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

package log_test

import (
	"bytes"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/log"
)

func TestEncoder(t *testing.T) {
	var (
		fields = []log.Field{

			log.Bool("bool", true),
			log.Int("int", 1),
			log.String("string", "abc"),
			log.Reflect("reflect", map[string]string{"string": "abc"}),

			log.Any("bool_any", true),
			log.Any("int_any", 1),
			log.Any("string_any", "abc"),
			log.Any("reflect_any", map[string]string{"string": "abc"}),

			log.Array("array", log.BoolValue(true), log.StringValue("abc")),
			log.Object("object", log.Bool("bool", true), log.String("string", "abc")),
		}
		buffer = bytes.NewBuffer(nil)
	)
	testcases := []struct {
		encoder log.Encoder
		expect  string
	}{
		{
			encoder: log.NewJSONEncoder(buffer),
			expect:  `{"bool":true,"int":1,"string":"abc","reflect":{"string":"abc"},"bool_any":true,"int_any":1,"string_any":"abc","reflect_any":{"string":"abc"},"array":[true,"abc"],"object":{"bool":true,"string":"abc"}}`,
		},
		{
			encoder: log.NewFlatEncoder(buffer, "||"),
			expect:  `bool=true||int=1||string=abc||reflect={"string":"abc"}||bool_any=true||int_any=1||string_any=abc||reflect_any={"string":"abc"}||array=[true,"abc"]||object={"bool":true,"string":"abc"}`,
		},
	}
	for _, c := range testcases {
		buffer.Reset()
		err := c.encoder.AppendEncoderBegin()
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range fields {
			err = c.encoder.AppendKey(f.Key)
			if err != nil {
				t.Fatal(err)
			}
			err = f.Val.Encode(c.encoder)
			if err != nil {
				t.Fatal(err)
			}
		}
		err = c.encoder.AppendEncoderEnd()
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, buffer.String(), c.expect)
	}
}
