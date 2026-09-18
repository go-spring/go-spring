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

package StarterRegistryEtcd

import (
	"context"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// The two halves of the backend talk to etcd through the narrow interfaces
// below instead of *clientv3.Client, so the non-live tests can drive each half
// against an in-process double. What they pin is timing no real cluster can be
// asked for on cue: a keep-alive dying mid-flight, a Grant failing while a stop
// races the re-publish.
//
// They are split per consumer rather than merged into one "etcd client"
// interface, and stay unexported, for the same reason: each names exactly the
// calls its consumer makes, so a double only has to stand in for behaviour
// something actually uses. The control-plane calls the backend needs (Status,
// Close) are in neither — they carry nothing worth faking, and keeping them on
// the concrete client stops the seam from widening into the bean's surface.

// registrarKV is the etcd surface the registrar's lease protocol uses: grant a
// lease, write under it, renew it, release it.
type registrarKV interface {
	Grant(ctx context.Context, ttl int64) (*clientv3.LeaseGrantResponse, error)
	Put(ctx context.Context, key, val string, lease clientv3.LeaseID) (*clientv3.PutResponse, error)
	Revoke(ctx context.Context, id clientv3.LeaseID) (*clientv3.LeaseRevokeResponse, error)
	KeepAlive(ctx context.Context, id clientv3.LeaseID) (<-chan *clientv3.LeaseKeepAliveResponse, error)
}

// discoveryKV is the etcd surface the discovery half reads through.
type discoveryKV interface {
	Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error)
	Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan
}

// etcdClient adapts *clientv3.Client to both interfaces. Put is the only method
// that needs writing: the client's own Put takes the lease as a
// clientv3.WithLease OpOption, and an OpOption wraps an Op whose fields are
// unexported, so nothing outside the package can read back which lease a write
// was bound to — a double included. Taking the LeaseID directly puts the one
// value the registrar actually chooses into the signature. Everything else is
// promoted from the embedded client unchanged.
type etcdClient struct{ *clientv3.Client }

// Put writes val under key bound to lease. See the type comment for why the
// lease is a parameter here and an OpOption on the client.
func (c etcdClient) Put(ctx context.Context, key, val string, lease clientv3.LeaseID) (*clientv3.PutResponse, error) {
	return c.Client.Put(ctx, key, val, clientv3.WithLease(lease))
}

// compile-time contract: the adapter is the production implementation of both
// seams, and the backend hands the same value to both halves.
var (
	_ registrarKV = etcdClient{}
	_ discoveryKV = etcdClient{}
)
