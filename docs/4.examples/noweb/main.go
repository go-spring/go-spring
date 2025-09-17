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
	"os"
	"time"

	"github.com/go-spring/log"
	"github.com/go-spring/spring-core/gs"
)

func main() {
	// Disable the built-in HTTP service.
	stopApp, err := gs.Web(false).RunAsync()
	if err != nil {
		log.Errorf(context.Background(), log.TagApp, "app run failed: %s", err.Error())
		os.Exit(1)
	}

	log.Infof(context.Background(), log.TagApp, "app started")
	time.Sleep(time.Minute)

	stopApp()
}

// ~ telnet 127.0.0.1 9090
// Trying 127.0.0.1...
// telnet: connect to address 127.0.0.1: Connection refused
// telnet: Unable to connect to remote host
