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
	"os"
	"path/filepath"
	"runtime"

	"github.com/go-spring/spring-core/gs"
	"gorm.io/gorm"

	_ "github.com/go-spring/starter-gorm-mysql"
)

type Service struct {
	DB *gorm.DB `autowire:"__default__"`
}

func main() {

	// Here `s` is not referenced by any other object,
	// so we need to register it as a root object.
	s := &Service{}

	http.HandleFunc("/mysql_version", func(w http.ResponseWriter, r *http.Request) {
		var version string
		err := s.DB.Raw("SELECT VERSION()").Scan(&version).Error
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write([]byte(version))
	})

	// Configure and run the Go-Spring application.
	//
	// Here we register `s` (the Service instance) as a root object.
	// Root objects are top-level beans that are not dependencies of any other object.
	// By providing it as a root, Go-Spring will manage its lifecycle and dependency injection.
	//
	// gs.Configure sets up the application configuration, and Run starts the application.
	gs.Configure(func(app gs.App) {
		app.Root(app.Provide(s))
	}).Run()

	// Example usage:
	//
	// ~ curl http://127.0.0.1:9090/mysql_version
	// 9.3.0%
}

// ----------------------------------------------------------------------------
// Change working directory
// ----------------------------------------------------------------------------

// init sets the working directory of the application to the directory
// where this source file resides.
// This ensures that any relative file operations are based on the source file location,
// not the process launch path.
func init() {
	var execDir string
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		execDir = filepath.Dir(filename)
	}
	err := os.Chdir(execDir)
	if err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
