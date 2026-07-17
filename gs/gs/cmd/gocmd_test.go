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

package cmd

import (
	"testing"

	"go-spring.org/gs/internal/runcmd"
	"go-spring.org/stdlib/testing/assert"
)

func TestPeelVerbosity(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantLevel int
		wantRest  []string
	}{
		{
			name:      "no verbosity, passthrough intact",
			args:      []string{"build", "./..."},
			wantLevel: 0,
			wantRest:  []string{"build", "./..."},
		},
		{
			name:      "leading -v is a gs flag",
			args:      []string{"-v", "build"},
			wantLevel: 1,
			wantRest:  []string{"build"},
		},
		{
			name:      "stacked -vv",
			args:      []string{"-vv", "test"},
			wantLevel: 2,
			wantRest:  []string{"test"},
		},
		{
			name:      "repeated -v -v",
			args:      []string{"-v", "-v", "vet"},
			wantLevel: 2,
			wantRest:  []string{"vet"},
		},
		{
			name:      "--verbose long form",
			args:      []string{"--verbose", "build"},
			wantLevel: 1,
			wantRest:  []string{"build"},
		},
		{
			name:      "--verbose=N explicit",
			args:      []string{"--verbose=3", "build"},
			wantLevel: 3,
			wantRest:  []string{"build"},
		},
		{
			name:      "go's own -v after subcommand passes through",
			args:      []string{"build", "-v", "./..."},
			wantLevel: 0,
			wantRest:  []string{"build", "-v", "./..."},
		},
		{
			name:      "gs -v then go -v: only leading one is peeled",
			args:      []string{"-v", "test", "-v"},
			wantLevel: 1,
			wantRest:  []string{"test", "-v"},
		},
		{
			name:      "empty args",
			args:      []string{},
			wantLevel: 0,
			wantRest:  []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runcmd.Verbosity = 0
			rest := peelVerbosity(tt.args)
			assert.That(t, runcmd.Verbosity).Equal(tt.wantLevel)
			assert.That(t, rest).Equal(tt.wantRest)
		})
	}
}
