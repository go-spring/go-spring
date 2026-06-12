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
	"os"
	"testing"

	"github.com/go-spring/gs-mock/internal/assert"
)

func TestMockgen(t *testing.T) {

	// Test default generation for all interfaces in a sample directory
	t.Run("all_default", func(t *testing.T) {
		old := stdOut
		stdOut = bytes.NewBuffer(nil)
		defer func() { stdOut = old }()

		run(runConfig{
			SourceDir: "./testdata/all_default",
		})

		b, err := os.ReadFile("./testdata/all_default/output.txt")
		assert.Nil(t, err)
		assert.Equal(t, stdOut.(*bytes.Buffer).String(), string(b))
	})

	// Test package name conflict scenario
	t.Run("conflict_pkg_name", func(t *testing.T) {
		assert.Panic(t, func() {
			run(runConfig{
				SourceDir: "./testdata/conflict_pkg_name",
			})
		}, "import package name conflict: stdio, io")
	})

	// Test exceeding maximum allowed input parameters
	t.Run("error_input_params", func(t *testing.T) {
		assert.Panic(t, func() {
			run(runConfig{
				SourceDir: "./testdata/error_input_params",
			})
		}, "have more than 6 parameters")
	})

	// Test exceeding maximum allowed return values
	t.Run("error_return_params", func(t *testing.T) {
		assert.Panic(t, func() {
			run(runConfig{
				SourceDir: "./testdata/error_return_params",
			})
		}, "have more than 4 results")
	})

	// Test successful generation with interface filtering
	t.Run("success", func(t *testing.T) {
		run(runConfig{
			SourceDir:      "example",
			OutputFile:     "src_mock.go",
			MockInterfaces: "'!RepositoryV2,,GenericService,Service,,Repository'",
		})
	})
}
