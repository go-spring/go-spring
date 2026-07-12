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
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"

	"greetapi/internal/types"
)

// The consumer talks to the provider directly over HTTP. Unlike the sibling
// greet-rpc, there is no etcd hop: go-zero's rest.Server does not participate
// in a registry, so the consumer takes the provider's host:port on the
// command line and self-asserts on the JSON body.
func main() {
	endpoint := flag.String("endpoint", "http://127.0.0.1:8888", "provider base URL")
	flag.Parse()

	body, err := json.Marshal(types.GreetReq{Name: "Hello, go-zero!"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error marshalling request: %v\n", err)
		os.Exit(1)
	}

	url := *endpoint + "/greet"
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error calling %s: %v\n", url, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "unexpected status %d from %s: %s\n", resp.StatusCode, url, string(raw))
		os.Exit(1)
	}

	var greet types.GreetResp
	if err := json.NewDecoder(resp.Body).Decode(&greet); err != nil {
		fmt.Fprintf(os.Stderr, "error decoding response: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Response from provider:", greet.Greeting)
	if greet.Greeting != "Hello, go-zero!" {
		fmt.Fprintf(os.Stderr, "unexpected greet body: %q\n", greet.Greeting)
		os.Exit(1)
	}
}
