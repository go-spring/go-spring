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

package StarterRegistryNacos

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"go-spring.org/cloud/discovery"
)

// fakeNamingClient stands in for a Nacos naming server: it serves one instance
// set and records the installed subscriber so tests can play pushes.
// Unimplemented methods come from the embedded interface.
type fakeNamingClient struct {
	naming_client.INamingClient

	mu       sync.Mutex
	set      []model.Instance
	callback func(services []model.Instance, err error)
	clusters []string
}

func (f *fakeNamingClient) SelectInstances(p vo.SelectInstancesParam) ([]model.Instance, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clusters = p.Clusters
	if !p.HealthyOnly {
		return nil, nil
	}
	return f.set, nil
}

func (f *fakeNamingClient) Subscribe(p *vo.SubscribeParam) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.callback = p.SubscribeCallback
	return nil
}

func (f *fakeNamingClient) Unsubscribe(*vo.SubscribeParam) error { return nil }

// push simulates a Nacos service-info update.
func (f *fakeNamingClient) push(set []model.Instance) {
	f.mu.Lock()
	f.set = set
	cb := f.callback
	f.mu.Unlock()
	if cb != nil {
		cb(set, nil)
	}
}

func inst(ip string, port uint64, weight float64, enabled, healthy bool) model.Instance {
	return model.Instance{Ip: ip, Port: port, Weight: weight, Enable: enabled, Healthy: healthy}
}

// TestNacosDiscovery_Resolve covers the query path: field mapping (addr,
// weight, enable→Disabled inverted, healthy) and scheme filtering.
func TestNacosDiscovery_Resolve(t *testing.T) {
	fake := &fakeNamingClient{set: []model.Instance{
		inst("10.0.0.1", 8080, 10, true, true),
		{Ip: "10.0.0.2", Port: 8443, Weight: 5, Enable: false, Healthy: true, Metadata: map[string]string{"scheme": "tls"}},
	}}
	d := &nacosDiscovery{client: fake, group: "DEFAULT_GROUP"}

	eps, err := d.Resolve(context.Background(), "order-svc")
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 2 {
		t.Fatalf("want 2 endpoints, got %d", len(eps))
	}
	if eps[0].Addr != "10.0.0.1:8080" || eps[0].Weight != 10 || eps[0].Disabled || !eps[0].Healthy {
		t.Fatalf("field mapping wrong: %+v", eps[0])
	}
	if eps[1].Scheme != "tls" || !eps[1].Disabled {
		t.Fatalf("metadata scheme / enable mapping wrong: %+v", eps[1])
	}

	// Scheme narrowing drops the plain instance.
	eps, err = d.Resolve(context.Background(), "order-svc", discovery.WithScheme("tls"))
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 || eps[0].Addr != "10.0.0.2:8443" {
		t.Fatalf("scheme filter wrong: %+v", eps)
	}
}

// TestNacosDiscovery_WatchPushesSnapshots covers the watch chain: the channel
// seeds with the current set, a Nacos push delivers the new snapshot, a no-op
// re-push delivers nothing, and cancelling ctx closes the channel.
func TestNacosDiscovery_WatchPushesSnapshots(t *testing.T) {
	fake := &fakeNamingClient{set: []model.Instance{inst("10.0.0.1", 8080, 10, true, true)}}
	d := &nacosDiscovery{client: fake, group: "DEFAULT_GROUP"}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := d.Watch(ctx, "order-svc")
	if err != nil {
		t.Fatal(err)
	}

	// First result carries the current set.
	select {
	case r := <-ch:
		if len(r.Endpoints) != 1 || r.Endpoints[0].Addr != "10.0.0.1:8080" {
			t.Fatalf("seed snapshot wrong: %+v", r.Endpoints)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no seed snapshot")
	}

	// A Nacos push with one more instance delivers the new full snapshot.
	fake.push([]model.Instance{
		inst("10.0.0.1", 8080, 10, true, true),
		inst("10.0.0.2", 8080, 10, true, true),
	})
	select {
	case r := <-ch:
		if len(r.Endpoints) != 2 {
			t.Fatalf("pushed snapshot wrong: %+v", r.Endpoints)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no snapshot after push")
	}

	// A byte-equal re-push is suppressed (no churn).
	fake.push([]model.Instance{
		inst("10.0.0.1", 8080, 10, true, true),
		inst("10.0.0.2", 8080, 10, true, true),
	})
	select {
	case r := <-ch:
		t.Fatalf("no-op re-push must not deliver, got %+v", r.Endpoints)
	case <-time.After(300 * time.Millisecond):
	}

	// Cancelling ctx closes the channel (the caller's only stop signal).
	cancel()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("channel must be closed after cancel")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("channel not closed after cancel")
	}
}

// TestNacosDiscovery_ClusterScope covers the cluster contract: a set cluster
// narrows queries to that one cluster, an empty cluster spans all clusters
// (nil filter). This mirrors the registrar's "DEFAULT" default so a no-config
// consumer sees a no-config provider.
func TestNacosDiscovery_ClusterScope(t *testing.T) {
	fake := &fakeNamingClient{set: []model.Instance{inst("10.0.0.1", 8080, 1, true, true)}}

	d := &nacosDiscovery{client: fake, group: "DEFAULT_GROUP", cluster: "DEFAULT"}
	if _, err := d.Resolve(context.Background(), "order-svc"); err != nil {
		t.Fatal(err)
	}
	if len(fake.clusters) != 1 || fake.clusters[0] != "DEFAULT" {
		t.Fatalf("cluster DEFAULT must narrow the query, got %v", fake.clusters)
	}

	d = &nacosDiscovery{client: fake, group: "DEFAULT_GROUP", cluster: ""}
	if _, err := d.Resolve(context.Background(), "order-svc"); err != nil {
		t.Fatal(err)
	}
	if len(fake.clusters) != 0 {
		t.Fatalf("empty cluster must span all clusters (nil filter), got %v", fake.clusters)
	}
}
