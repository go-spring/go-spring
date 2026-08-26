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

package StarterDubbo

import (
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"dubbo.apache.org/dubbo-go/v3/config_center"
	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/governance/resilience"
	mapconfig "go-spring.org/starter-dubbo/internal/mapconfig"
)

const testApp = "test-app"

// setDyncConsumer pushes a DubboConsumer into the poller's Consumer field.
func setDyncConsumer(p *dyncPoller, c DubboConsumer) {
	type dync struct{ v atomic.Value }
	(*dync)(unsafe.Pointer(&p.Consumer)).v.Store(c)
}

func newTestPoller() *dyncPoller {
	// Clear any authority a prior governance test armed on the global facade so
	// these (non-governance) tests see Enabled()==false and don't inherit overrides.
	governance.Reset()
	return newDyncPoller(DubboApplication{Name: testApp})
}

// getRule fetches an override rule from the config center.
func getRule(t *testing.T, dc *mapconfig.MapDynamicConfiguration, key string) string {
	t.Helper()
	raw, err := dc.GetRule(key+".configurators", config_center.WithGroup("dubbo"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestDyncPoller_NoChange(t *testing.T) {
	dc := mapconfig.Singleton()
	p := newTestPoller()

	setDyncConsumer(p, DubboConsumer{
		References: map[string]DubboReference{
			"greet": {Interface: "greet.GreetService", Timeout: "3000", Retries: 3},
		},
	})

	p.poll()

	snap := dc.Snapshot()
	p.poll()
	snap2 := dc.Snapshot()
	if len(snap) != len(snap2) {
		t.Fatal("second poll with same values should not alter config center")
	}
}

func TestDyncPoller_ChangeDetected(t *testing.T) {
	dc := mapconfig.Singleton()
	p := newTestPoller()
	svcKey := "greet.GreetService::"

	setDyncConsumer(p, DubboConsumer{
		References: map[string]DubboReference{
			"greet": {Interface: "greet.GreetService", LoadBalance: "roundrobin"},
		},
	})
	p.poll()

	setDyncConsumer(p, DubboConsumer{
		References: map[string]DubboReference{
			"greet": {Interface: "greet.GreetService", LoadBalance: "leastactive"},
		},
	})
	p.poll()

	raw := getRule(t, dc, svcKey)
	urls, err := dc.Parser().ParseToUrls(raw)
	if err != nil {
		t.Fatal(err)
	}
	if v := urls[0].GetParam("loadbalance", ""); v != "leastactive" {
		t.Fatalf("expected loadbalance=leastactive, got %q", v)
	}
}

func TestDyncPoller_EmptyRefsSkipped(t *testing.T) {
	dc := mapconfig.Singleton()
	dc.RefreshOverrideRules(nil)
	p := newTestPoller()

	p.poll()

	keys, _ := dc.GetConfigKeysByGroup("dubbo")
	if keys.Size() != 0 {
		t.Fatal("expected no keys when Consumer is empty")
	}
}

func TestDyncPoller_ClusterAndLoadBalance(t *testing.T) {
	dc := mapconfig.Singleton()
	p := newTestPoller()
	svcKey := "greet.GreetService::"

	setDyncConsumer(p, DubboConsumer{
		References: map[string]DubboReference{
			"greet": {Interface: "greet.GreetService", Cluster: "failfast", LoadBalance: "roundrobin"},
		},
	})
	p.poll()

	raw := getRule(t, dc, svcKey)
	urls, err := dc.Parser().ParseToUrls(raw)
	if err != nil {
		t.Fatal(err)
	}
	url := urls[0]
	if v := url.GetParam("cluster", ""); v != "failfast" {
		t.Fatalf("expected cluster=failfast, got %q", v)
	}
	if v := url.GetParam("loadbalance", ""); v != "roundrobin" {
		t.Fatalf("expected loadbalance=roundrobin, got %q", v)
	}
}

func TestDyncPoller_ConsumerDefaults(t *testing.T) {
	dc := mapconfig.Singleton()
	p := newTestPoller()
	svcKey := "greet.GreetService::"

	setDyncConsumer(p, DubboConsumer{
		LoadBalance:    "roundrobin",
		Cluster:        "failfast",
		RequestTimeout: "5s",
		References: map[string]DubboReference{
			"greet": {Interface: "greet.GreetService", Timeout: "3000"},
		},
	})
	p.poll()

	// Consumer-level defaults published under appName.
	raw := getRule(t, dc, testApp)
	urls, err := dc.Parser().ParseToUrls(raw)
	if err != nil {
		t.Fatal(err)
	}
	url := urls[0]
	if v := url.GetParam("loadbalance", ""); v != "roundrobin" {
		t.Fatalf("expected loadbalance=roundrobin, got %q", v)
	}
	if v := url.GetParam("cluster", ""); v != "failfast" {
		t.Fatalf("expected cluster=failfast, got %q", v)
	}

	// Per-reference overrides published under the service key.
	rawRef := getRule(t, dc, svcKey)
	urlsRef, err := dc.Parser().ParseToUrls(rawRef)
	if err != nil {
		t.Fatal(err)
	}
	if v := urlsRef[0].GetParam("timeout", ""); v != "3000" {
		t.Fatalf("expected timeout=3000 (reference override), got %q", v)
	}
}

func TestDyncPoller_MultipleRefs(t *testing.T) {
	dc := mapconfig.Singleton()
	p := newTestPoller()

	setDyncConsumer(p, DubboConsumer{
		References: map[string]DubboReference{
			"app-a": {Interface: "svc.A", LoadBalance: "random"},
			"app-b": {Interface: "svc.B", LoadBalance: "p2c"},
		},
	})
	p.poll()

	// Each reference gets its own service-level rule — no last-wins merge.
	rawA := getRule(t, dc, "svc.A::")
	urlsA, err := dc.Parser().ParseToUrls(rawA)
	if err != nil {
		t.Fatal(err)
	}
	if v := urlsA[0].GetParam("loadbalance", ""); v != "random" {
		t.Fatalf("expected loadbalance=random for svc.A, got %q", v)
	}

	rawB := getRule(t, dc, "svc.B::")
	urlsB, err := dc.Parser().ParseToUrls(rawB)
	if err != nil {
		t.Fatal(err)
	}
	if v := urlsB[0].GetParam("loadbalance", ""); v != "p2c" {
		t.Fatalf("expected loadbalance=p2c for svc.B, got %q", v)
	}
}

func TestDyncPoller_AllDynamicFields(t *testing.T) {
	dc := mapconfig.Singleton()
	p := newTestPoller()
	svcKey := "svc.Full:1.0:v2" // version=1.0, group=v2 → colonSeparatedKey

	setDyncConsumer(p, DubboConsumer{
		References: map[string]DubboReference{
			"full-app": {
				Interface:     "svc.Full",
				Cluster:       "failover",
				LoadBalance:   "leastactive",
				Group:         "v2",
				Version:       "1.0",
				Serialization: "protobuf",
				Sticky:        true,
				ForceTag:      true,
				Timeout:       "5s",
				Retries:       3,
				Methods: map[string]DubboMethod{
					"GetUser": {
						LoadBalance: "roundrobin",
						Weight:      200,
						Sticky:      true,
						Timeout:     "2s",
						Retries:     1,
					},
				},
			},
		},
	})
	p.poll()

	raw := getRule(t, dc, svcKey)
	urls, err := dc.Parser().ParseToUrls(raw)
	if err != nil {
		t.Fatal(err)
	}
	url := urls[0]

	checks := map[string]string{
		"cluster":                     "failover",
		"loadbalance":                 "leastactive",
		"group":                       "v2",
		"version":                     "1.0",
		"serialization":               "protobuf",
		"sticky":                      "true",
		"force.tag":                   "true",
		"timeout":                     "5s",
		"retries":                     "3",
		"methods.GetUser.loadbalance": "roundrobin",
		"methods.GetUser.weight":      "200",
		"methods.GetUser.sticky":      "true",
		"methods.GetUser.timeout":     "2s",
		"methods.GetUser.retries":     "1",
	}
	for k, expected := range checks {
		if v := url.GetParam(k, ""); v != expected {
			t.Fatalf("expected %s=%s, got %q", k, expected, v)
		}
	}
}

func TestDyncPoller_Init(t *testing.T) {
	dc := mapconfig.Singleton()
	dc.RefreshOverrideRules(nil)
	p := newTestPoller()

	setDyncConsumer(p, DubboConsumer{
		References: map[string]DubboReference{
			"greet": {Interface: "greet.GreetService", Timeout: "3000"},
		},
	})

	if err := p.Init(); err != nil {
		t.Fatal(err)
	}

	// Init must push the initial rules even though OnChanged does not fire on
	// the init bind (RefreshField runs onCommit, not onFinish).
	if raw := getRule(t, dc, "greet.GreetService::"); raw == "" {
		t.Fatal("expected initial override push on Init")
	}
}

func TestDyncPoller_SnapshotLast(t *testing.T) {
	p := newTestPoller()
	svcKey := "greet.GreetService::"

	setDyncConsumer(p, DubboConsumer{
		References: map[string]DubboReference{
			"greet": {Interface: "greet.GreetService", LoadBalance: "random", Cluster: "failover"},
		},
	})
	p.poll()

	snap := p.snapshotLast()
	if snap[svcKey]["loadbalance"] != "random" {
		t.Fatalf("expected loadbalance=random in snapshot, got %q", snap[svcKey]["loadbalance"])
	}
	if snap[svcKey]["cluster"] != "failover" {
		t.Fatalf("expected cluster=failover in snapshot, got %q", snap[svcKey]["cluster"])
	}
}

func TestDyncPoller_ConsumerOnly(t *testing.T) {
	dc := mapconfig.Singleton()
	p := newTestPoller()

	setDyncConsumer(p, DubboConsumer{
		LoadBalance: "roundrobin",
		Cluster:     "failfast",
	})
	p.poll()

	raw := getRule(t, dc, testApp)
	urls, err := dc.Parser().ParseToUrls(raw)
	if err != nil {
		t.Fatal(err)
	}
	url := urls[0]
	if v := url.GetParam("loadbalance", ""); v != "roundrobin" {
		t.Fatalf("expected loadbalance=roundrobin, got %q", v)
	}
	if v := url.GetParam("cluster", ""); v != "failfast" {
		t.Fatalf("expected cluster=failfast, got %q", v)
	}
}

func TestDyncPoller_RefWithoutInterface(t *testing.T) {
	dc := mapconfig.Singleton()
	p := newTestPoller()

	setDyncConsumer(p, DubboConsumer{
		LoadBalance: "roundrobin",
		References: map[string]DubboReference{
			"greet": {LoadBalance: "random"},
		},
	})
	p.poll()

	// Reference without Interface is skipped, consumer-level defaults apply.
	raw := getRule(t, dc, testApp)
	urls, err := dc.Parser().ParseToUrls(raw)
	if err != nil {
		t.Fatal(err)
	}
	if v := urls[0].GetParam("loadbalance", ""); v != "roundrobin" {
		t.Fatalf("expected loadbalance=roundrobin (consumer default), got %q", v)
	}
}

func TestDyncPoller_SidePresent(t *testing.T) {
	dc := mapconfig.Singleton()
	p := newTestPoller()
	svcKey := "greet.GreetService::"

	setDyncConsumer(p, DubboConsumer{
		References: map[string]DubboReference{
			"greet": {Interface: "greet.GreetService", LoadBalance: "roundrobin"},
		},
	})
	p.poll()

	raw := getRule(t, dc, svcKey)
	urls, err := dc.Parser().ParseToUrls(raw)
	if err != nil {
		t.Fatal(err)
	}
	// Verify both consumer and provider side items are present.
	if len(urls) != 2 {
		t.Fatalf("expected 2 URLs (consumer+provider), got %d", len(urls))
	}
	consumerSide := urls[0].GetParam("side", "")
	providerSide := urls[1].GetParam("side", "")
	if consumerSide != "consumer" {
		t.Fatalf("expected first URL side=consumer, got %q", consumerSide)
	}
	if providerSide != "provider" {
		t.Fatalf("expected second URL side=provider, got %q", providerSide)
	}
}

// TestDyncPoller_GovernOverride is the Level A test: when a governance center is
// armed, its PolicyFor for each dubbo resource label overrides timeout/retries in
// the published override rules, regardless of the dubbo-native values. This is
// how ${govern} takes over dubbo's dynamic timeout/retry.
func TestDyncPoller_GovernOverride(t *testing.T) {
	dc := mapconfig.Singleton()
	reset := governance.Arm(governance.Config{
		Enabled: true,
		Driver:  "default",
		Default: resilience.PolicyConfig{AttemptTimeout: 2 * time.Second, MaxRetries: 4},
		// Per-reference override: a different timeout for one service. The key is
		// the full dubbo resource label ("dubbo:" + colonSeparatedKey).
		Rules: []governance.Rule{{
			Resources: []string{"dubbo:greet.GreetService::"},
			PolicyConfig: resilience.PolicyConfig{AttemptTimeout: 500 * time.Millisecond, MaxRetries: 1},
		}},
	})
	t.Cleanup(reset) // don't leak into other tests
	p := &dyncPoller{
		dynCfg:  mapconfig.Singleton(),
		appName: testApp,
		last:    make(map[string]map[string]string),
		regged:  make(map[string]bool),
	}

	setDyncConsumer(p, DubboConsumer{
		// Dubbo-native values that the center must override:
		RequestTimeout: "3000", // overridden to 2000 (2s) by center default
		Retries:        9,      // overridden to 4 by center default
		References: map[string]DubboReference{
			"greet": {Interface: "greet.GreetService", Timeout: "9999", Retries: 9},
		},
	})
	p.poll()

	// App-level override: timeout/retries from center Default (2s/4), not 3000/9.
	appRaw := getRule(t, dc, testApp)
	appURLs, err := dc.Parser().ParseToUrls(appRaw)
	if err != nil {
		t.Fatal(err)
	}
	if v := appURLs[0].GetParam("timeout", ""); v != "2000" {
		t.Fatalf("app timeout: center should override to 2000ms, got %q", v)
	}
	if v := appURLs[0].GetParam("retries", ""); v != "4" {
		t.Fatalf("app retries: center should override to 4, got %q", v)
	}

	// Reference-level: the per-reference override (500ms/1) beats the center
	// default, and beats the dubbo-native 9999/9.
	refRaw := getRule(t, dc, "greet.GreetService::")
	refURLs, err := dc.Parser().ParseToUrls(refRaw)
	if err != nil {
		t.Fatal(err)
	}
	if v := refURLs[0].GetParam("timeout", ""); v != "500" {
		t.Fatalf("ref timeout: center override should win at 500ms, got %q", v)
	}
	if v := refURLs[0].GetParam("retries", ""); v != "1" {
		t.Fatalf("ref retries: center override should win at 1, got %q", v)
	}
}

// TestDyncPoller_GovernDisabledIsNoop confirms a disabled center (or nil) leaves
// the dubbo-native timeout/retries untouched - the Level A path is inert.
func TestDyncPoller_GovernDisabledIsNoop(t *testing.T) {
	dc := mapconfig.Singleton()
	// Disabled authority: Enabled=false even though Default has a policy.
	reset := governance.Arm(governance.Config{Enabled: false, Default: resilience.PolicyConfig{AttemptTimeout: 2 * time.Second}})
	t.Cleanup(reset)
	p := &dyncPoller{dynCfg: mapconfig.Singleton(), appName: testApp,
		last: make(map[string]map[string]string), regged: make(map[string]bool)}

	setDyncConsumer(p, DubboConsumer{
		RequestTimeout: "3000", Retries: 2,
		References: map[string]DubboReference{
			"greet": {Interface: "greet.GreetService", Timeout: "3000", Retries: 3},
		},
	})
	p.poll()

	appRaw := getRule(t, dc, testApp)
	appURLs, _ := dc.Parser().ParseToUrls(appRaw)
	if v := appURLs[0].GetParam("timeout", ""); v != "3000" {
		t.Fatalf("disabled center must not override: timeout want 3000, got %q", v)
	}
	if v := appURLs[0].GetParam("retries", ""); v != "2" {
		t.Fatalf("disabled center must not override: retries want 2, got %q", v)
	}
}
