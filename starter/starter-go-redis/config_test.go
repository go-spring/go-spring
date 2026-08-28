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

package StarterGoRedis

import (
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string // empty means no error
	}{
		{
			name: "single with addr",
			cfg:  Config{Mode: "single", Addr: "127.0.0.1:6379"},
		},
		{
			name: "single with service-name only",
			cfg:  Config{Mode: "single", ServiceName: "my-redis"},
		},
		{
			name:    "single with neither addr nor service-name",
			cfg:     Config{Mode: "single"},
			wantErr: "addr",
		},
		{
			name:    "single with non-zero db is allowed",
			cfg:     Config{Mode: "single", Addr: "127.0.0.1:6379", DB: 2},
			wantErr: "",
		},
		{
			name: "sentinel ok",
			cfg:  Config{Mode: "sentinel", MasterName: "m", SentinelAddrs: []string{"127.0.0.1:26379"}},
		},
		{
			name:    "sentinel with service-name rejected",
			cfg:     Config{Mode: "sentinel", MasterName: "m", SentinelAddrs: []string{"127.0.0.1:26379"}, ServiceName: "x"},
			wantErr: "service-name is not supported in sentinel mode",
		},
		{
			name: "cluster ok with db 0",
			cfg:  Config{Mode: "cluster", Addrs: []string{"127.0.0.1:7000"}, DB: 0},
		},
		{
			name:    "cluster with non-zero db rejected",
			cfg:     Config{Mode: "cluster", Addrs: []string{"127.0.0.1:7000"}, DB: 1},
			wantErr: "db is not supported in cluster mode",
		},
		{
			name:    "cluster without addrs rejected",
			cfg:     Config{Mode: "cluster"},
			wantErr: "addrs is required in cluster mode",
		},
		{
			name:    "invalid mode rejected",
			cfg:     Config{Mode: "sharded"},
			wantErr: "invalid mode",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.cfg)
			if tt.wantErr == "" {
				assert.Error(t, err).Nil()
			} else {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				assert.String(t, err.Error()).Contains(tt.wantErr)
			}
		})
	}
}
