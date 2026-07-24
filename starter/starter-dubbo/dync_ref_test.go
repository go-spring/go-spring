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

// setDyncRefs pushes a rule map into the poller's Refs field.
// gs.Dync[T] is { v atomic.Value }; we use unsafe to set the internal
// value without going through conf.BindValue. This is a test helper only.
func setDyncRefs(p *dyncPoller, rules map[string]map[string]string) {
	type dync struct{ v atomic.Value }
	(*dync)(unsafe.Pointer(&p.Refs)).v.Store(rules)
}

func TestDyncPoller_NoChange(t *testing.T) {
	dc := mapconfig.Singleton()
	p := newDyncPoller()

	setDyncRefs(p, map[string]map[string]string{
		"greet": {"timeout": "3000", "retries": "3"},
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
	p := newDyncPoller()

	setDyncRefs(p, map[string]map[string]string{
		"greet": {"timeout": "3000"},
	})
	p.poll()

	setDyncRefs(p, map[string]map[string]string{
		"greet": {"timeout": "5000"},
	})
	p.poll()

	raw, err := dc.GetRule("greet.configurators", config_center.WithGroup("dubbo"))
	if err != nil {
		t.Fatal(err)
	}
	urls, err := dc.Parser().ParseToUrls(raw)
	if err != nil {
		t.Fatal(err)
	}
	if v := urls[0].GetParam("timeout", ""); v != "5000" {
		t.Fatalf("expected timeout=5000, got %q", v)
	}
}

func TestDyncPoller_EmptyRefsSkipped(t *testing.T) {
	dc := mapconfig.Singleton()
	dc.RefreshOverrideRules(nil) // clear leftover state from other tests
	p := newDyncPoller()

	p.poll()

	keys, _ := dc.GetConfigKeysByGroup("dubbo")
	if keys.Size() != 0 {
		t.Fatal("expected no keys when Refs is empty")
	}
}

func TestDyncPoller_ClusterAndLoadBalance(t *testing.T) {
	dc := mapconfig.Singleton()
	p := newDyncPoller()

	setDyncRefs(p, map[string]map[string]string{
		"greet": {"cluster": "failfast", "loadbalance": "roundrobin"},
	})
	p.poll()

	raw, err := dc.GetRule("greet.configurators", config_center.WithGroup("dubbo"))
	if err != nil {
		t.Fatal(err)
	}
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

func TestDyncPoller_PushesToConfigCenter(t *testing.T) {
	dc := mapconfig.Singleton()
	p := newDyncPoller()

	setDyncRefs(p, map[string]map[string]string{
		"my-app": {"timeout": "5000", "retries": "3"},
	})
	p.poll()

	raw, err := dc.GetRule("my-app.configurators", config_center.WithGroup("dubbo"))
	if err != nil {
		t.Fatal(err)
	}
	urls, err := dc.Parser().ParseToUrls(raw)
	if err != nil {
		t.Fatal(err)
	}
	url := urls[0]
	if v := url.GetParam("timeout", ""); v != "5000" {
		t.Fatalf("expected timeout=5000, got %q", v)
	}
	if v := url.GetParam("retries", ""); v != "3" {
		t.Fatalf("expected retries=3, got %q", v)
	}
}

func TestDyncPoller_MultipleRefs(t *testing.T) {
	dc := mapconfig.Singleton()
	p := newDyncPoller()

	setDyncRefs(p, map[string]map[string]string{
		"app-a": {"timeout": "3000"},
		"app-b": {"timeout": "8000"},
	})
	p.poll()

	for _, tc := range []struct{ name, expectedTimeout string }{
		{"app-a", "3000"},
		{"app-b", "8000"},
	} {
		raw, err := dc.GetRule(tc.name+".configurators", config_center.WithGroup("dubbo"))
		if err != nil {
			t.Fatal(err)
		}
		urls, err := dc.Parser().ParseToUrls(raw)
		if err != nil {
			t.Fatal(err)
		}
		if v := urls[0].GetParam("timeout", ""); v != tc.expectedTimeout {
			t.Fatalf("%s: expected timeout=%s, got %q", tc.name, tc.expectedTimeout, v)
		}
	}
}

func TestDyncPoller_AllFields(t *testing.T) {
	dc := mapconfig.Singleton()
	p := newDyncPoller()

	setDyncRefs(p, map[string]map[string]string{
		"full-app": {
			"timeout":     "5000",
			"retries":     "2",
			"cluster":     "failover",
			"loadbalance": "leastactive",
		},
	})
	p.poll()

	raw, err := dc.GetRule("full-app.configurators", config_center.WithGroup("dubbo"))
	if err != nil {
		t.Fatal(err)
	}
	urls, err := dc.Parser().ParseToUrls(raw)
	if err != nil {
		t.Fatal(err)
	}
	url := urls[0]
	if v := url.GetParam("timeout", ""); v != "5000" {
		t.Fatalf("expected timeout=5000, got %q", v)
	}
	if v := url.GetParam("retries", ""); v != "2" {
		t.Fatalf("expected retries=2, got %q", v)
	}
	if v := url.GetParam("cluster", ""); v != "failover" {
		t.Fatalf("expected cluster=failover, got %q", v)
	}
	if v := url.GetParam("loadbalance", ""); v != "leastactive" {
		t.Fatalf("expected loadbalance=leastactive, got %q", v)
	}
}

func TestDyncPoller_StartAndStop(t *testing.T) {
	p := newDyncPoller()

	if err := p.Init(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)

	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestDyncPoller_SnapshotLast(t *testing.T) {
	p := newDyncPoller()

	setDyncRefs(p, map[string]map[string]string{
		"greet": {"timeout": "3000", "retries": "3"},
	})
	p.poll()

	snap := p.snapshotLast()
	if snap["greet"]["timeout"] != "3000" {
		t.Fatalf("expected timeout=3000 in snapshot, got %q", snap["greet"]["timeout"])
	}
	if snap["greet"]["retries"] != "3" {
		t.Fatalf("expected retries=3 in snapshot, got %q", snap["greet"]["retries"])
	}
}
