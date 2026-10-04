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

package StarterMemcached

import (
	"context"
	"testing"

	"github.com/bradfitz/gomemcache/memcache"
	"go-spring.org/cloud"
)

// fakeInner is a scripted tail: it records the keys it was called with and
// answers from a settable map. It stands in for the adapter layer so chain
// tests need no memcached server.
type fakeInner struct {
	keys     []string
	values   map[string]*memcache.Item
	released bool // whether Release(true) reached the tail
}

func (f *fakeInner) Get(_ context.Context, key string) (*memcache.Item, error) {
	f.keys = append(f.keys, key)
	if it, ok := f.values[key]; ok {
		return it, nil
	}
	return nil, memcache.ErrCacheMiss
}

func (f *fakeInner) Set(_ context.Context, item *memcache.Item) error {
	f.keys = append(f.keys, item.Key)
	if f.values == nil {
		f.values = map[string]*memcache.Item{}
	}
	f.values[item.Key] = item
	return nil
}

func (f *fakeInner) Delete(_ context.Context, key string) error {
	f.keys = append(f.keys, key)
	delete(f.values, key)
	return nil
}

func (f *fakeInner) Release(releaseRaw bool) error {
	if releaseRaw {
		f.released = true
	}
	return nil
}

// The remaining commands are not exercised by these tests; embed nothing —
// satisfy the interface with panics so an accidental call is loud.
func (f *fakeInner) GetAndTouch(context.Context, string, int32) (*memcache.Item, error) {
	panic("unexpected")
}
func (f *fakeInner) GetMulti(context.Context, []string) (map[string]*memcache.Item, error) {
	panic("unexpected")
}
func (f *fakeInner) Touch(context.Context, string, int32) error { panic("unexpected") }
func (f *fakeInner) Add(context.Context, *memcache.Item) error  { panic("unexpected") }
func (f *fakeInner) Replace(context.Context, *memcache.Item) error {
	panic("unexpected")
}
func (f *fakeInner) Append(context.Context, *memcache.Item) error { panic("unexpected") }
func (f *fakeInner) Prepend(context.Context, *memcache.Item) error {
	panic("unexpected")
}
func (f *fakeInner) CompareAndSwap(context.Context, *memcache.Item) error {
	panic("unexpected")
}
func (f *fakeInner) DeleteAll(context.Context) error { panic("unexpected") }
func (f *fakeInner) Increment(context.Context, string, uint64) (uint64, error) {
	panic("unexpected")
}
func (f *fakeInner) Decrement(context.Context, string, uint64) (uint64, error) {
	panic("unexpected")
}
func (f *fakeInner) Ping(context.Context) error     { panic("unexpected") }
func (f *fakeInner) FlushAll(context.Context) error { panic("unexpected") }

// prefixLayer namespaces every key it passes down — the kind of behavior
// change no gomemcache hook could express. It embeds the tail it found, so it
// inherits everything it does not override.
type prefixLayer struct {
	InnerClient
	prefix string
}

func (p prefixLayer) Get(ctx context.Context, key string) (*memcache.Item, error) {
	return p.InnerClient.Get(ctx, p.prefix+key)
}

func (p prefixLayer) Set(ctx context.Context, item *memcache.Item) error {
	return p.InnerClient.Set(ctx, &memcache.Item{Key: p.prefix + item.Key, Value: item.Value})
}

func (p prefixLayer) Delete(ctx context.Context, key string) error {
	return p.InnerClient.Delete(ctx, p.prefix+key)
}

// newChainClient builds a Client whose chain bottoms out in tail — the
// identity and governance layers are real (executor unmanaged, governance
// off), the adapter is not.
func newChainClient(t *testing.T, tail InnerClient) *Client {
	t.Helper()
	c := NewClient(memcache.New("127.0.0.1:0"), "hot", "", cloud.ClientParams{})
	c.InnerClient = NewObsClient(NewGuardClient(tail, "hot", "", cloud.ClientParams{}))
	return c
}

// TestInnerChainRewritesKeys pins the seam the embedded InnerClient opens: a
// custom layer under the executor head rewrites keys, and the promoted command
// surface runs through it — misses surface as ErrCacheMiss, exactly as the
// adapter layer would return them.
func TestInnerChainRewritesKeys(t *testing.T) {
	fake := &fakeInner{values: map[string]*memcache.Item{
		"app:k": {Key: "app:k", Value: []byte("v")},
	}}
	c := newChainClient(t, prefixLayer{InnerClient: fake, prefix: "app:"})

	if _, err := c.Get(context.Background(), "k"); err != nil {
		t.Fatalf("Get(k) through the prefix layer: %v", err)
	}
	if _, err := c.Get(context.Background(), "absent"); err != memcache.ErrCacheMiss {
		t.Fatalf("Get(absent) = %v, want memcache.ErrCacheMiss", err)
	}
	if err := c.Set(context.Background(), &memcache.Item{Key: "k2", Value: []byte("v2")}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := fake.keys; len(got) != 3 || got[0] != "app:k" || got[2] != "app:k2" {
		t.Fatalf("tail saw keys %v, want [app:k app:absent app:k2]", got)
	}
}

// TestInnerChainExternalRebuild pins the wrap-head rebuild protocol: the
// custom layer takes over the embedded field with the whole chain under it,
// and Close (the full teardown) still reaches the tail through the wrapped
// chain.
func TestInnerChainExternalRebuild(t *testing.T) {
	fake := &fakeInner{}
	c := newChainClient(t, fake)
	c.Set(context.Background(), &memcache.Item{Key: "k", Value: []byte("v")}) // warm: keys=[k]

	c.InnerClient = prefixLayer{InnerClient: c.InnerClient, prefix: "app:"}
	if _, err := c.Get(context.Background(), "k"); err != memcache.ErrCacheMiss {
		t.Fatalf("Get(k) in the rewritten namespace = %v, want miss", err)
	}
	if err := c.Set(context.Background(), &memcache.Item{Key: "k", Value: []byte("v")}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := fake.keys; len(got) != 3 || got[1] != "app:k" || got[2] != "app:k" {
		t.Fatalf("tail saw keys %v, want [k app:k app:k]", got)
	}

	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !fake.released {
		t.Fatal("Close must release the tail")
	}
}
