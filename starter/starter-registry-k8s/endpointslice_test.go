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

package StarterRegistryK8s

import (
	"context"
	"testing"
	"time"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/testing/assert"
)

func strptr(s string) *string { return &s }
func i32ptr(i int32) *int32   { return &i }
func boolptr(b bool) *bool    { return &b }

// slice builds an EndpointSlice owned by service "svc" in namespace "ns".
func slice(name string, eps []discoveryv1.Endpoint, ports []discoveryv1.EndpointPort) *discoveryv1.EndpointSlice {
	return &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "ns",
			Labels:    map[string]string{serviceNameLabel: "svc"},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints:   eps,
		Ports:       ports,
	}
}

func newESD(t *testing.T, cfg Config, objs ...*discoveryv1.EndpointSlice) *endpointSliceDiscovery {
	t.Helper()
	client := fake.NewSimpleClientset()
	for _, o := range objs {
		_, err := client.DiscoveryV1().EndpointSlices("ns").Create(context.Background(), o, metav1.CreateOptions{})
		assert.Error(t, err).Nil()
	}
	return &endpointSliceDiscovery{
		cfg:      cfg,
		client:   client,
		entries:  map[string]*esEntry{},
		watchers: map[*watcherHandle]struct{}{},
	}
}

func TestEndpointSlice_ResolveNamedPortReadyZone(t *testing.T) {
	esd := newESD(t, Config{Mode: ModeEndpointSlice, Namespace: "ns", PortName: "grpc"},
		slice("svc-1",
			[]discoveryv1.Endpoint{
				{Addresses: []string{"10.0.0.1"}, Conditions: discoveryv1.EndpointConditions{Ready: boolptr(true)}, Zone: strptr("z-a")},
				{Addresses: []string{"10.0.0.2"}, Conditions: discoveryv1.EndpointConditions{Ready: boolptr(false)}, Zone: strptr("z-b")},
			},
			[]discoveryv1.EndpointPort{
				{Name: strptr("grpc"), Port: i32ptr(8080)},
				{Name: strptr("http"), Port: i32ptr(80)},
			},
		),
	)

	eps, err := esd.Resolve(context.Background(), "svc")
	assert.Error(t, err).Nil()
	assert.Slice(t, addrsOf(eps)).Equal([]string{"10.0.0.1:8080", "10.0.0.2:8080"})

	byAddr := map[string]discovery.Endpoint{}
	for _, e := range eps {
		byAddr[e.Addr] = e
	}
	assert.That(t, byAddr["10.0.0.1:8080"].Healthy).True()
	assert.That(t, byAddr["10.0.0.2:8080"].Healthy).False()
	assert.String(t, byAddr["10.0.0.1:8080"].Metadata["zone"]).Equal("z-a")
}

func TestEndpointSlice_ResolveSinglePortFallback(t *testing.T) {
	// No PortName and no Port: a slice with exactly one port uses it.
	esd := newESD(t, Config{Mode: ModeEndpointSlice, Namespace: "ns"},
		slice("svc-1",
			[]discoveryv1.Endpoint{{Addresses: []string{"10.0.0.5"}}},
			[]discoveryv1.EndpointPort{{Name: strptr("http"), Port: i32ptr(80)}},
		),
	)
	eps, err := esd.Resolve(context.Background(), "svc")
	assert.Error(t, err).Nil()
	assert.Slice(t, addrsOf(eps)).Equal([]string{"10.0.0.5:80"})
	// Nil Ready condition is treated as ready.
	assert.That(t, eps[0].Healthy).True()
}

func TestEndpointSlice_InformerRefreshesCache(t *testing.T) {
	esd := newESD(t, Config{Mode: ModeEndpointSlice, Namespace: "ns", PortName: "grpc"},
		slice("svc-1",
			[]discoveryv1.Endpoint{{Addresses: []string{"10.0.0.1"}, Conditions: discoveryv1.EndpointConditions{Ready: boolptr(true)}}},
			[]discoveryv1.EndpointPort{{Name: strptr("grpc"), Port: i32ptr(8080)}},
		),
	)

	// Seed: the first Resolve lists the current slices.
	eps, err := esd.Resolve(context.Background(), "svc")
	assert.Error(t, err).Nil()
	assert.Slice(t, addrsOf(eps)).Equal([]string{"10.0.0.1:8080"})

	// Scale up: update the slice; the informer refreshes the cache, so a
	// later Resolve observes it (poll for the async refresh).
	updated := slice("svc-1",
		[]discoveryv1.Endpoint{
			{Addresses: []string{"10.0.0.1"}, Conditions: discoveryv1.EndpointConditions{Ready: boolptr(true)}},
			{Addresses: []string{"10.0.0.2"}, Conditions: discoveryv1.EndpointConditions{Ready: boolptr(true)}},
		},
		[]discoveryv1.EndpointPort{{Name: strptr("grpc"), Port: i32ptr(8080)}},
	)
	_, err = esd.client.DiscoveryV1().EndpointSlices("ns").Update(context.Background(), updated, metav1.UpdateOptions{})
	assert.Error(t, err).Nil()

	deadline := time.Now().Add(3 * time.Second)
	for {
		eps, err := esd.Resolve(context.Background(), "svc")
		assert.Error(t, err).Nil()
		if len(eps) == 2 {
			assert.Slice(t, addrsOf(eps)).Equal([]string{"10.0.0.1:8080", "10.0.0.2:8080"})
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("informer refresh not observed, endpoints = %v", addrsOf(eps))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestEndpointSlice_CloseStopsWatchers(t *testing.T) {
	esd := newESD(t, Config{Mode: ModeEndpointSlice, Namespace: "ns", PortName: "grpc"},
		slice("svc-1",
			[]discoveryv1.Endpoint{{Addresses: []string{"10.0.0.1"}, Conditions: discoveryv1.EndpointConditions{Ready: boolptr(true)}}},
			[]discoveryv1.EndpointPort{{Name: strptr("grpc"), Port: i32ptr(8080)}},
		),
	)

	// Seed a watcher, wait for it to register with the backend, then Close:
	// the informer goroutine must exit rather than leak.
	_, err := esd.Resolve(context.Background(), "svc")
	assert.Error(t, err).Nil()
	deadline := time.Now().Add(3 * time.Second)
	for {
		esd.mu.Lock()
		n := len(esd.watchers)
		esd.mu.Unlock()
		if n == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("informer watcher did not register in time")
		}
		time.Sleep(10 * time.Millisecond)
	}
	assert.Error(t, esd.Close()).Nil()
}
