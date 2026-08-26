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

package StarterConfigNacos

import (
	"sync"
	"testing"
	"time"

	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"go-spring.org/cloud/governance"
)

const govRulesV1 = `govern:
  enabled: true
  default:
    enabled: true
    attempt-timeout: 100ms
`

const govRulesV2 = `govern:
  enabled: true
  default:
    enabled: true
    attempt-timeout: 300ms
    max-retries: 1
`

// fakeConfigClient stands in for a nacos server: it serves one document and
// records the installed listener so tests can play publishes. Unimplemented
// methods come from the embedded interface (nil panics if wrongly touched).
// getErr/listenErr inject failures for error-path tests; listens counts
// ListenConfig calls so listener-dedup can be asserted.
type fakeConfigClient struct {
	config_client.IConfigClient

	mu        sync.Mutex
	data      string
	getErr    error
	listenErr error
	listens   int
	onChange  func(namespace, group, dataId, data string)
}

func (f *fakeConfigClient) GetConfig(vo.ConfigParam) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.data, f.getErr
}

func (f *fakeConfigClient) ListenConfig(p vo.ConfigParam) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listenErr != nil {
		return f.listenErr
	}
	f.listens++
	f.onChange = p.OnChange
	return nil
}

func (f *fakeConfigClient) CancelListenConfig(vo.ConfigParam) error { return nil }

// publish simulates a nacos config push.
func (f *fakeConfigClient) publish(data string) {
	f.mu.Lock()
	f.data = data
	cb := f.onChange
	f.mu.Unlock()
	if cb != nil {
		cb("", "DEFAULT_GROUP", "govern.yaml", data)
	}
}

func newGovTestSource(t *testing.T, data string) (*NacosSource, *fakeConfigClient) {
	t.Helper()
	fake := &fakeConfigClient{data: data}
	src, err := NewNacosSource(fake, configSource{server: "127.0.0.1:8848", dataID: "govern.yaml", group: "DEFAULT_GROUP", format: "yaml"})
	if err != nil {
		t.Fatal(err)
	}
	return src, fake
}

// TestNacosSource_PushChain covers the direct-listener chain: seed, publish →
// parse → push, bad publish keeps last good, byte-equal re-push is a no-op.
func TestNacosSource_PushChain(t *testing.T) {
	src, fake := newGovTestSource(t, govRulesV1)
	if err := src.Init(); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var pushes int
	var pushed governance.Config
	src.Subscribe(func(cfg governance.Config) { mu.Lock(); pushed = cfg; pushes++; mu.Unlock() })

	if cfg := src.Snapshot(); cfg.Default.AttemptTimeout != 100*time.Millisecond {
		t.Fatalf("seed snapshot: %+v", cfg.Default)
	}

	// A publish with new rules pushes.
	fake.publish(govRulesV2)
	mu.Lock()
	got, n := pushed.Default.AttemptTimeout, pushes
	mu.Unlock()
	if n != 1 || got != 300*time.Millisecond {
		t.Fatalf("publish should push new rules once: n=%d timeout=%v", n, got)
	}

	// A bad publish keeps the last good snapshot and pushes nothing.
	fake.publish("govern: { broken")
	if cfg := src.Snapshot(); cfg.Default.AttemptTimeout != 300*time.Millisecond {
		t.Fatalf("bad publish must keep last good snapshot: %+v", cfg.Default)
	}
	mu.Lock()
	n = pushes
	mu.Unlock()
	if n != 1 {
		t.Fatalf("bad publish must not push: n=%d", n)
	}

	// A byte-equal re-push (nacos may re-deliver on reconnect) is a no-op.
	fake.publish(govRulesV2)
	mu.Lock()
	n = pushes
	mu.Unlock()
	if n != 1 {
		t.Fatalf("byte-equal re-push must not push: n=%d", n)
	}
}

// TestNacosSource_BadSeedFailsFast pins startup behavior: an unparseable
// initial document fails construction instead of arming a disabled center.
func TestNacosSource_BadSeedFailsFast(t *testing.T) {
	fake := &fakeConfigClient{data: "govern: {"}
	if _, err := NewNacosSource(fake, configSource{server: "s", dataID: "govern.yaml", format: "yaml"}); err == nil {
		t.Fatal("bad initial document should fail construction")
	}
}
