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

package recorder_test

import (
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-replay/recorder"
)

func TestCSV(t *testing.T) {
	inputs := []interface{}{
		"CMD",
		1,
		true,
		"string",
		"\x00\xc0\n\t\x00\xbem\x06\x89Z(\x00\n",
	}
	data := recorder.EncodeCSV(inputs...)
	assert.Equal(t, data, `"CMD","1","true","string","\x00\xc0\n\t\x00\xbem\x06\x89Z(\x00\n"`)
	outputs, err := recorder.DecodeCSV(data)
	if err != nil {
		return
	}
	assert.Equal(t, outputs, []string{
		"CMD",
		"1",
		"true",
		"string",
		"\x00\xc0\n\t\x00\xbem\x06\x89Z(\x00\n",
	})
}

func TestTTY(t *testing.T) {
	inputs := []interface{}{
		"CMD",
		1,
		true,
		"string",
		"hello world",
		"this is a quote \"",
		"\x00\xc0\n\t\x00\xbem\x06\x89Z(\x00\n",
	}
	data := recorder.EncodeTTY(inputs...)
	assert.Equal(t, data, `CMD 1 true string "hello world" "this is a quote \"" "\x00\xc0\n\t\x00\xbem\x06\x89Z(\x00\n"`)
	outputs, err := recorder.DecodeTTY(data)
	if err != nil {
		return
	}
	assert.Equal(t, outputs, []string{
		"CMD",
		"1",
		"true",
		"string",
		"hello world",
		"this is a quote \"",
		"\x00\xc0\n\t\x00\xbem\x06\x89Z(\x00\n",
	})
}
