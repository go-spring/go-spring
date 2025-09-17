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
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-spring/spring-core/gs"
	"github.com/lvan100/go-loop"

	_ "bookman/src/app"
	_ "bookman/src/biz"
)

func init() {
	gs.SetActiveProfiles("online")
	gs.EnableSimplePProfServer(true)
	gs.FuncJob(runTest).Name("#job")
}

func main() {
	gs.Run()
}

// runTest performs a simple test.
func runTest(ctx context.Context) error {
	time.Sleep(time.Millisecond * 500)

	loop.Times(5, func(_ int) {
		url := "http://127.0.0.1:9090/books"
		resp, err := http.Get(url)
		if err != nil {
			panic(err)
		}
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		defer func() {
			err = resp.Body.Close()
			_ = err
		}()
		fmt.Print(string(b))
		time.Sleep(time.Millisecond * 400)
	})

	// Shut down the application gracefully
	gs.ShutDown()
	return nil
}
