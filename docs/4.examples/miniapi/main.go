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
	"context"
	"net/http"

	"github.com/go-spring/log"
	"github.com/go-spring/spring-core/gs"
)

func main() {
	// Register an HTTP handler for the "/echo" endpoint.
	http.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello world!"))
	})

	// Start the Go-Spring framework.
	// Compared to http.ListenAndServe, gs.Run() starts a full-featured application context with:
	// - Auto Configuration: Automatically loads properties and beans.
	// - Property Binding: Binds external configs (YAML, ENV) into structs.
	// - Dependency Injection: Wires beans automatically.
	// - Dynamic Refresh: Updates configs at runtime without restart.
	gs.RunWith(func(ctx context.Context) error {
		log.Infof(ctx, log.TagAppDef, "app started")
		return nil
	})
}

//~ curl http://127.0.0.1:9090/echo
//hello world!
