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
	mapconfig "go-spring.org/starter-dubbo/internal/mapconfig"
)

const testApp = "test-app"

// setDyncConsumer pushes a DubboConsumer into the poller's Consumer field.
func setDyncConsumer(p *dyncPoller, c DubboConsumer) {
	type dync struct{ v atomic.Value }
	(*dync)(unsafe.Pointer(&p.Consumer)).v.Store(c)
}

func newTestPoller() *dyncPoller {
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
	svcKey := "greet.GreetService"

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
	svcKey := "greet.GreetService"

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
	svcKey := "greet.GreetService"

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
	rawA := getRule(t, dc, "svc.A")
	urlsA, err := dc.Parser().ParseToUrls(rawA)
	if err != nil {
		t.Fatal(err)
	}
	if v := urlsA[0].GetParam("loadbalance", ""); v != "random" {
		t.Fatalf("expected loadbalance=random for svc.A, got %q", v)
	}

	rawB := getRule(t, dc, "svc.B")
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
	svcKey := "svc.Full"

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
		"cluster":                       "failover",
		"loadbalance":                   "leastactive",
		"group":                         "v2",
		"version":                       "1.0",
		"serialization":                 "protobuf",
		"sticky":                        "true",
		"force.tag":                     "true",
		"timeout":                       "5s",
		"retries":                       "3",
		"methods.GetUser.loadbalance":   "roundrobin",
		"methods.GetUser.weight":        "200",
		"methods.GetUser.sticky":        "true",
		"methods.GetUser.timeout":       "2s",
		"methods.GetUser.retries":       "1",
	}
	for k, expected := range checks {
		if v := url.GetParam(k, ""); v != expected {
			t.Fatalf("expected %s=%s, got %q", k, expected, v)
		}
	}
}

func TestDyncPoller_StartAndStop(t *testing.T) {
	p := newTestPoller()

	if err := p.Init(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)

	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestDyncPoller_SnapshotLast(t *testing.T) {
	p := newTestPoller()
	svcKey := "greet.GreetService"

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
	svcKey := "greet.GreetService"

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
