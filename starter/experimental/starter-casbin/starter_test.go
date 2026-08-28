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

package StarterCasbin

import (
	"context"
	"strings"
	"testing"

	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	"go-spring.org/spring/gs"
)

// fakeAdapter is a minimal in-memory persist.Adapter so tests don't need a
// storage driver dependency.
type fakeAdapter struct{}

func (fakeAdapter) LoadPolicy(m model.Model) error                      { return nil }
func (fakeAdapter) SavePolicy(m model.Model) error                      { return nil }
func (fakeAdapter) AddPolicy(sec, ptype string, rule []string) error    { return nil }
func (fakeAdapter) RemovePolicy(sec, ptype string, rule []string) error { return nil }
func (fakeAdapter) RemoveFilteredPolicy(sec, ptype string, fieldIndex int, fieldValues ...string) error {
	return nil
}

var _ persist.Adapter = fakeAdapter{}

func newTestCtx() *gs.ContextProvider {
	return &gs.ContextProvider{Context: context.Background()}
}

const (
	testModel  = "testdata/model.conf"
	testPolicy = "testdata/policy.csv"
)

func TestPolicyAdapterMutualExclusion(t *testing.T) {
	RegisterAdapter("fake", fakeAdapter{})
	defer func() { delete(adapters, "fake") }()

	tests := []struct {
		name    string
		cfg     Config
		wantErr string // empty means success
	}{
		{
			name:    "both set is a startup error",
			cfg:     Config{Model: testModel, Policy: testPolicy, Adapter: "fake"},
			wantErr: "mutually exclusive",
		},
		{
			name: "only policy is ok",
			cfg:  Config{Model: testModel, Policy: testPolicy},
		},
		{
			name: "only adapter is ok",
			cfg:  Config{Model: testModel, Adapter: "fake"},
		},
		{
			name: "neither boots with an empty policy",
			cfg:  Config{Model: testModel},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, err := newEnforcer(newTestCtx(), "test", tt.cfg)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("want error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("want error containing %q, got %q", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if e.Enforcer == nil {
				t.Fatal("enforcer is nil")
			}
		})
	}
}

func TestOnlyPolicyLoadsRules(t *testing.T) {
	e, err := newEnforcer(newTestCtx(), "test", Config{Model: testModel, Policy: testPolicy})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ok, err := e.Enforce("alice", "/data", "read")
	if err != nil || !ok {
		t.Fatalf("alice should be allowed to read /data, ok=%v err=%v", ok, err)
	}
}

func TestNeitherSourceDeniesAll(t *testing.T) {
	e, err := newEnforcer(newTestCtx(), "test", Config{Model: testModel})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ok, err := e.Enforce("alice", "/data", "read")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("empty policy should deny everything")
	}
}
