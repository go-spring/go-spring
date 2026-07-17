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
	"bufio"

	"github.com/gogf/gf/v2/net/gtcp"
	goframetcp "go-spring.org/starter-goframe/tcp"
	"go-spring.org/spring/gs"
)

func init() {
	// Provide the starter's ServiceRegister bean that attaches echoHandler onto
	// the raw *gtcp.Server via SetHandler. Importing the starter package
	// (goframetcp) triggers its module init, which registers the *gtcp.Server as
	// a gs.Server; this bean is the only wiring the application supplies — the
	// listen/register lifecycle and log bridge live in the starter now (they used
	// to be the deleted provider/server.go).
	gs.Provide(func() goframetcp.ServiceRegister {
		return func(s *gtcp.Server) {
			s.SetHandler(echoHandler)
		}
	})
}

// echoHandler is the gtcp connection handler attached via the ServiceRegister
// bean above. It is a bufio.Reader-backed line echo: for every newline-
// terminated frame received, it writes the same bytes back. That gives the
// consumer a deterministic value to assert on.
func echoHandler(conn *gtcp.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		// ReadBytes preserves the delimiter so the echoed frame is
		// self-describing on the wire; the consumer trims it.
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if _, werr := conn.Write(line); werr != nil {
				return
			}
		}
		if err != nil {
			// io.EOF or a broken pipe both end the loop; the deferred
			// Close will run.
			return
		}
	}
}
