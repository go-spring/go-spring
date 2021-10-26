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
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/go-spring/spring-base/recorder"
)

func TestUnmarshal(t *testing.T) {
	src := &recorder.Session{
		ID: "1635158767928168604136517013",
		Actions: []*recorder.Action{
			{
				Protocol: recorder.HTTP,
				Key:      "GET^/index",
				Data: &recorder.Http{
					Method:  "GET",
					URI:     "/index",
					Version: "HTTP/1.1",
					Request: &recorder.HttpRequest{
						Query: map[string][]string{
							"ticket": {"IvOSbAkTHZNntCWTqrwKk"},
						},
						Header: map[string][]string{
							"Content-Type": {"application/json"},
						},
						Body: "{\"id\":\"580502622244281\"}",
					},
					Response: &recorder.HttpResponse{
						Status: "HTTP/1.1 200 OK",
						Header: map[string][]string{
							"Content-Type": {"application/json"},
						},
						Body: "{\"errno\":0,\"errmsg\":\"SUCCESS\"}",
					},
				},
			},
			{
				Protocol: recorder.REDIS,
				Key:      "GET^a",
				Data: &recorder.Redis{
					Request: []interface{}{
						"GET", "a",
					},
					Response: "1",
				},
			},
		},
	}
	b, err := json.MarshalIndent(src, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(b))

	var dest *recorder.Session
	err = json.Unmarshal(b, &dest)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(src, dest) {
		t.Fatalf("expect %+v but got %+v", src, dest)
	}
}
