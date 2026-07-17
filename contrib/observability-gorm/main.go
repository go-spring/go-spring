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

// Command observability-gorm demonstrates go-spring's unified observability:
// importing starter-otel (which installs the shared OTel providers as process
// globals) plus starter-gorm-mysql (whose client is auto-bridged to those
// globals) is enough to get GORM query spans and connection-pool metrics — no
// per-component instrumentation code. Configuration lives entirely in
// conf/app.properties under ${spring.observability} and ${spring.gorm.mysql}.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"go-spring.org/spring/gs"

	// Unified observability layer: builds TracerProvider/MeterProvider from
	// ${spring.observability} and sets them as OTel globals during startup.
	_ "go-spring.org/starter-otel"

	// GORM MySQL client; its newClient calls db.Use(otel plugin), which reads
	// the globals installed above. Importing both is the entire wiring.
	_ "go-spring.org/starter-gorm-mysql"
)

func main() {
	gs.Run()
}

// init pins the working directory to this source directory so gs loads
// conf/app.properties regardless of where the binary is launched from.
func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot resolve caller")
	}
	if err := os.Chdir(filepath.Dir(filename)); err != nil {
		panic(err)
	}
	fmt.Println("workdir:", filepath.Dir(filename))
}
