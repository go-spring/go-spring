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
	"fmt"
	"net/http"
	"time"

	"github.com/go-spring/spring-core/gs"
)

func init() {
	// Register the Service struct as a bean.
	gs.Object(&Service{})

	// Provide a [*http.ServeMux] as a bean.
	gs.Provide(func(s *Service) *http.ServeMux {
		http.HandleFunc("/echo", s.Echo)
		http.HandleFunc("/refresh", s.Refresh)
		return http.DefaultServeMux
	})

	gs.Property("start-time", time.Now().Format(timeLayout))
	gs.Property("refresh-time", time.Now().Format(timeLayout))
}

const timeLayout = "2006-01-02 15:04:05.999 -0700 MST"

type Service struct {
	StartTime   time.Time          `value:"${start-time}"`
	RefreshTime gs.Dync[time.Time] `value:"${refresh-time}"`
}

func (s *Service) Echo(w http.ResponseWriter, r *http.Request) {
	str := fmt.Sprintf("start-time: %s refresh-time: %s",
		s.StartTime.Format(timeLayout),
		s.RefreshTime.Value().Format(timeLayout))
	_, _ = w.Write([]byte(str))
}

func (s *Service) Refresh(w http.ResponseWriter, r *http.Request) {
	gs.Property("refresh-time", time.Now().Format(timeLayout))
	_ = gs.RefreshProperties()
	_, _ = w.Write([]byte("OK!"))
}

func main() {
	gs.Run()
}

// ➜ curl http://127.0.0.1:9090/echo
// start-time: 2025-03-14 13:32:51.608 +0800 CST refresh-time: 2025-03-14 13:32:51.608 +0800 CST%
// ➜ curl http://127.0.0.1:9090/refresh
// OK!%
// ➜ curl http://127.0.0.1:9090/echo
// start-time: 2025-03-14 13:32:51.608 +0800 CST refresh-time: 2025-03-14 13:33:02.936 +0800 CST%
// ➜ curl http://127.0.0.1:9090/refresh
// OK!%
// ➜ curl http://127.0.0.1:9090/echo
// start-time: 2025-03-14 13:32:51.608 +0800 CST refresh-time: 2025-03-14 13:33:08.88 +0800 CST%
