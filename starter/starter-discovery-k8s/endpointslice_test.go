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

package StarterDiscoveryK8s

import (
	"context"
	"testing"
	"time"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"go-spring.org/stdlib/discovery"
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
		watchers: map[*endpointSliceWatcher]struct{}{},
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

func TestEndpointSlice_WatchDetectsScaleUp(t *testing.T) {
	esd := newESD(t, Config{Mode: ModeEndpointSlice, Namespace: "ns", PortName: "grpc"},
		slice("svc-1",
			[]discoveryv1.Endpoint{{Addresses: []string{"10.0.0.1"}, Conditions: discoveryv1.EndpointConditions{Ready: boolptr(true)}}},
			[]discoveryv1.EndpointPort{{Name: strptr("grpc"), Port: i32ptr(8080)}},
		),
	)

	w, err := esd.Watch(context.Background(), "svc")
	assert.Error(t, err).Nil()
	defer w.Stop()

	// Initial sync pushes the starting snapshot.
	eps := nextWithTimeout(t, w, 2*time.Second)
	assert.Slice(t, addrsOf(eps)).Equal([]string{"10.0.0.1:8080"})

	// Scale up: update the slice to add a second endpoint.
	updated := slice("svc-1",
		[]discoveryv1.Endpoint{
			{Addresses: []string{"10.0.0.1"}, Conditions: discoveryv1.EndpointConditions{Ready: boolptr(true)}},
			{Addresses: []string{"10.0.0.2"}, Conditions: discoveryv1.EndpointConditions{Ready: boolptr(true)}},
		},
		[]discoveryv1.EndpointPort{{Name: strptr("grpc"), Port: i32ptr(8080)}},
	)
	_, err = esd.client.DiscoveryV1().EndpointSlices("ns").Update(context.Background(), updated, metav1.UpdateOptions{})
	assert.Error(t, err).Nil()

	eps = nextWithTimeout(t, w, 2*time.Second)
	assert.Slice(t, addrsOf(eps)).Equal([]string{"10.0.0.1:8080", "10.0.0.2:8080"})
}

func TestEndpointSlice_CloseStopsWatchers(t *testing.T) {
	esd := newESD(t, Config{Mode: ModeEndpointSlice, Namespace: "ns", PortName: "grpc"},
		slice("svc-1",
			[]discoveryv1.Endpoint{{Addresses: []string{"10.0.0.1"}, Conditions: discoveryv1.EndpointConditions{Ready: boolptr(true)}}},
			[]discoveryv1.EndpointPort{{Name: strptr("grpc"), Port: i32ptr(8080)}},
		),
	)
	w, err := esd.Watch(context.Background(), "svc")
	assert.Error(t, err).Nil()

	// Drain the initial snapshot queued on cache sync, then Close: the next
	// read must observe the stopped watcher rather than block forever.
	nextWithTimeout(t, w, 2*time.Second)
	assert.Error(t, esd.Close()).Nil()
	_, err = w.Next()
	assert.Error(t, err).Is(context.Canceled)
}
