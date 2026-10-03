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

package governance

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestParseGovernanceDoc pins the shared parse core every backend adapter
// funnels through: the same spring.governance.* keys every source document uses, format inference
// by name, and the no-govern-keys guard.
func TestParseGovernanceDoc(t *testing.T) {
	// properties (default format), same keys an app.properties entry would use.
	cfg, err := Parse("governance.properties", []byte(rulesV1), "")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Enabled || cfg.Client.Default.AttemptTimeout != 100*time.Millisecond {
		t.Fatalf("properties parse: %+v", cfg)
	}

	// yaml inferred from the name.
	cfg, err = Parse("governance.yaml", []byte(rulesV1YAML), "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Client.Default.AttemptTimeout != 100*time.Millisecond {
		t.Fatalf("yaml parse: %+v", cfg.Client.Default)
	}

	// format given explicitly, name without extension.
	cfg, err = Parse("console-payload", []byte(rulesV1), "properties")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Enabled {
		t.Fatal("explicit-format parse should bind enabled")
	}

	// no spring.governance.* keys → error, never a silently disabled center.
	if _, err = Parse("governance.yaml", []byte("foo: bar\n"), ""); err == nil {
		t.Fatal("document without spring.governance.* keys must be rejected")
	}
	// malformed yaml → error.
	if _, err = Parse("governance.yaml", []byte("spring.governance: {"), ""); err == nil {
		t.Fatal("malformed document must be rejected")
	}
}

// TestParse_LegacyKeysArmNothing pins the no-compatibility promise of the
// client/server split: the pre-split keys (spring.governance.default / spring.governance.rules /
// spring.governance.fault) are simply unknown now, so a document still written against them
// binds NOTHING — neither direction gets a policy or a fire, and the failure is
// loud only where it should be (the operator's own document review).
func TestParse_LegacyKeysArmNothing(t *testing.T) {
	cfg, err := Parse("governance.properties", []byte(`spring.governance.enabled=true
spring.governance.driver=default
spring.governance.default.attempt-timeout=100ms
spring.governance.rules[0].service=redis:cache
spring.governance.rules[0].attempt-timeout=50ms
spring.governance.fault.enabled=true
spring.governance.fault.rate=1
`), "")
	if err != nil {
		t.Fatal(err)
	}
	// The document still binds the keys that survived the split.
	if !cfg.Enabled || cfg.Driver != "default" {
		t.Fatalf("enabled/driver must still bind: %+v", cfg)
	}
	if !cfg.Client.Default.ClientPolicy.IsZero() || len(cfg.Client.Rules) != 0 {
		t.Fatalf("legacy spring.governance.default/rules must arm no outbound policy: %+v", cfg.Client)
	}
	if !cfg.Server.Default.IsZero() || len(cfg.Server.Rules) != 0 {
		t.Fatalf("legacy keys must arm no inbound governance: %+v", cfg.Server)
	}
	if cfg.Client.Fault.Enabled || cfg.Server.Fault.Enabled {
		t.Fatalf("legacy spring.governance.fault must arm neither fire: %+v / %+v", cfg.Client.Fault, cfg.Server.Fault)
	}
}

// TestHTTPSource_PollAndPush covers the console-pull chain end to end with a
// live test server: initial snapshot, poll picks up a changed document, and a
// failing endpoint keeps the last good snapshot.
func TestHTTPSource_PollAndPush(t *testing.T) {
	var muBody sync.Mutex
	body := rulesV1YAML
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		muBody.Lock()
		defer muBody.Unlock()
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	s, err := NewHTTPSource(srv.URL+"/governance.yaml", 50*time.Millisecond, "", map[string]string{"Authorization": "Bearer t"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err = s.Start(); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var pushes int
	var pushed Config
	s.Subscribe(func(cfg Config) { mu.Lock(); pushed = cfg; pushes++; mu.Unlock() })

	if cfg := s.Snapshot(); cfg.Client.Default.AttemptTimeout != 100*time.Millisecond {
		t.Fatalf("initial snapshot: %+v", cfg.Client.Default)
	}

	// Console publishes new rules; the next poll must pick them up and push.
	muBody.Lock()
	body = rulesV2YAML
	muBody.Unlock()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		done := pushed.Client.Default.AttemptTimeout == 300*time.Millisecond
		mu.Unlock()
		if done {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	got := pushed.Client.Default.AttemptTimeout
	mu.Unlock()
	if got != 300*time.Millisecond {
		t.Fatalf("poll should converge on the new document: %v", got)
	}

	// Console breaks (empty document): keep the last good snapshot, push nothing.
	mu.Lock()
	before := pushes
	mu.Unlock()
	muBody.Lock()
	body = ""
	muBody.Unlock()
	time.Sleep(300 * time.Millisecond)
	if cfg := s.Snapshot(); cfg.Client.Default.AttemptTimeout != 300*time.Millisecond {
		t.Fatalf("broken console must keep last good snapshot: %+v", cfg.Client.Default)
	}
	mu.Lock()
	after := pushes
	mu.Unlock()
	if after != before {
		t.Fatalf("broken console must not push, got %d extra pushes", after-before)
	}
}

// TestHTTPSource_UnreachableFailsFast pins startup behavior: a console that
// cannot be fetched at construction time surfaces as a construction error.
func TestHTTPSource_UnreachableFailsFast(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	if _, err := NewHTTPSource(srv.URL, time.Second, "", nil); err == nil {
		t.Fatal("non-200 console should fail construction")
	}
}
