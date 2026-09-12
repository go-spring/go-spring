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
	"fmt"
	"hash/crc32"
	"net"
	"sort"
	"strings"
	"sync"

	"github.com/bradfitz/gomemcache/memcache"
	"go-spring.org/cloud/discovery"
)

// keyBufPool returns []byte buffers for use by pickServer's call to
// crc32.ChecksumIEEE. It mirrors the pool in gomemcache's own ServerList so the
// live selector hashes keys the same way — and with the same lack of per-call
// allocation.
var keyBufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 256)
		return &b
	},
}

// staticAddr caches the Network() and String() values from a resolved net.Addr,
// mirroring gomemcache's own unexported type: the client calls String() on every
// operation, and *net.TCPAddr would recompute it each time.
type staticAddr struct {
	ntw, str string
}

func newStaticAddr(a net.Addr) net.Addr {
	return &staticAddr{ntw: a.Network(), str: a.String()}
}

func (s *staticAddr) Network() string { return s.ntw }
func (s *staticAddr) String() string  { return s.str }

// liveServers adapts a discovery [discovery.Resolver] into gomemcache's
// [memcache.ServerSelector], so cluster membership follows the naming service
// instead of being frozen at client creation: every key lookup re-reads the live
// endpoint snapshot, and an instance that joins or leaves is visible on the next
// operation rather than at the next restart.
//
// It reproduces the built-in [memcache.ServerList] semantics on purpose — same
// CRC32-of-key hashing, same "list a server twice for double weight", same
// unix-socket handling — because for memcached the selector IS the cache
// semantics: a key must keep landing on the instance that holds it. What differs
// is only where the server set comes from, plus one stabilizing rule: the
// snapshot is sorted by address before hashing, so an unchanged cluster keeps an
// unchanged key→server mapping no matter what order the naming service reports.
// (Weight is not used — gomemcache has no weighted selector; express weight the
// way the library does, by repeating an address.)
//
// Resolved addresses are cached per snapshot, so DNS is paid once per membership
// change, not per key lookup.
type liveServers struct {
	resolve discovery.Resolver

	mu   sync.Mutex
	key  string     // signature of the snapshot the addresses were resolved from
	addr []net.Addr // resolved, sorted
}

// newLiveServers wraps resolve as a [memcache.ServerSelector].
func newLiveServers(resolve discovery.Resolver) *liveServers {
	return &liveServers{resolve: resolve}
}

// PickServer returns the server the key hashes onto, reading the current
// endpoint snapshot. An empty snapshot is reported as [memcache.ErrNoServers]
// rather than silently serving a stale cluster.
func (s *liveServers) PickServer(key string) (net.Addr, error) {
	addrs, err := s.snapshot()
	if err != nil {
		return nil, err
	}
	if len(addrs) == 0 {
		return nil, memcache.ErrNoServers
	}
	if len(addrs) == 1 {
		return addrs[0], nil
	}
	bufp := keyBufPool.Get().(*[]byte)
	n := copy(*bufp, key)
	cs := crc32.ChecksumIEEE((*bufp)[:n])
	keyBufPool.Put(bufp)
	return addrs[cs%uint32(len(addrs))], nil
}

// Each iterates the current snapshot, for the client's cluster-wide operations
// (FlushAll, Stats). Like [memcache.ServerList.Each] it stops at the first error.
func (s *liveServers) Each(f func(net.Addr) error) error {
	addrs, err := s.snapshot()
	if err != nil {
		return err
	}
	for _, a := range addrs {
		if err := f(a); err != nil {
			return err
		}
	}
	return nil
}

// snapshot returns the currently known servers, re-resolving only when the
// endpoint set changed. A read error or an empty snapshot is surfaced to the
// caller as-is: for a cache, "the cluster is unknown" must not be papered over
// with a stale set the key may no longer live on.
func (s *liveServers) snapshot() ([]net.Addr, error) {
	eps, err := s.resolve()
	if err != nil {
		return nil, err
	}
	addrs := make([]string, 0, len(eps))
	for _, ep := range eps {
		addrs = append(addrs, ep.Addr)
	}
	sort.Strings(addrs)
	key := strings.Join(addrs, ",")

	s.mu.Lock()
	defer s.mu.Unlock()
	if key == s.key {
		return s.addr, nil
	}
	resolved := make([]net.Addr, 0, len(addrs))
	for _, a := range addrs {
		var addr net.Addr
		var err error
		if strings.Contains(a, "/") {
			addr, err = net.ResolveUnixAddr("unix", a)
		} else {
			addr, err = net.ResolveTCPAddr("tcp", a)
		}
		if err != nil {
			return nil, fmt.Errorf("memcached: resolve server %q: %w", a, err)
		}
		resolved = append(resolved, newStaticAddr(addr))
	}
	s.key, s.addr = key, resolved
	return s.addr, nil
}
