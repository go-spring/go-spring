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

package fastdev_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/go-spring/spring-base/fastdev"
	"github.com/go-spring/spring-base/fastdev/json"
)

func TestNewSessionID(t *testing.T) {
	fmt.Println(fastdev.NewSessionID())
}

func TestSessionUnmarshal(t *testing.T) {

	src := &fastdev.Session{
		Session: "54c8fab33dcb4f46899a3a3b70987164",
		Inbound: &fastdev.Action{
			Protocol: fastdev.HTTP,
			Request:  "GET ...",
			Response: "... 200 ...",
		},
		Actions: []*fastdev.Action{
			{
				Protocol: fastdev.REDIS,
				Request:  "SET a 1",
				Response: "OK",
			}, {
				Protocol: fastdev.REDIS,
				Request:  "GET a",
				Response: "1",
			},
		},
	}

	b, err := json.MarshalIndent(src, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(b))

	var dest *fastdev.Session
	err = json.Unmarshal(b, &dest)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(src, dest) {
		t.Fatalf("expect %+v but got %+v", src, dest)
	}
}
