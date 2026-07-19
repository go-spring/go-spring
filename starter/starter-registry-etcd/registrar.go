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
	"encoding/json"
	"sync"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go-spring.org/stdlib/discovery"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/starter"
)

// instanceValue is the JSON payload stored at an instance key. A discovery
// backend reading the same prefix reconstructs an Endpoint from it.
type instanceValue struct {
	ServiceName string            `json:"service_name"`
	Addr        string            `json:"addr"`
	Weight      int               `json:"weight,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// etcdRegistrar publishes instances to an etcd cluster. Each instance is written
// under a key bound to its own lease and kept alive by a background keep-alive;
// if the process dies the lease expires and etcd deletes the key automatically.
// It implements discovery.Registrar.
type etcdRegistrar struct {
	client    *clientv3.Client
	keyPrefix string
	ttlSecs   int64

	mu    sync.Mutex
	holds map[string]*hold // instance key -> its lease keep-alive
}

// hold tracks the lease and keep-alive goroutine backing one registered key.
type hold struct {
	leaseID clientv3.LeaseID
	cancel  context.CancelFunc
}

// newEtcdRegistrar builds a *clientv3.Client from c and returns a registrar. It
// fails fast when the cluster is unreachable within DialTimeout so a
// misconfigured application never boots with a silently broken registry.
func newEtcdRegistrar(c EtcdConfig) (*etcdRegistrar, error) {
	if len(c.Endpoints) == 0 {
		return nil, errutil.Explain(nil, "registry-etcd: endpoints is required")
	}
	tlsCfg, err := c.TLS.Build()
	if err != nil {
		return nil, errutil.Explain(err, "registry-etcd: build TLS")
	}
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   c.Endpoints,
		Username:    c.Username,
		Password:    c.Password,
		DialTimeout: c.DialTimeout,
		TLS:         tlsCfg,
	})
	if err != nil {
		return nil, errutil.Explain(err, "registry-etcd: failed to create etcd client")
	}
	// Fail-fast readiness probe: a Status against the first endpoint proves the
	// credentials and TLS material work, so a bad configuration surfaces at boot.
	ctx, cancel := context.WithTimeout(context.Background(), c.DialTimeout)
	defer cancel()
	if _, err := cli.Status(ctx, c.Endpoints[0]); err != nil {
		_ = cli.Close()
		return nil, errutil.Explain(err, "registry-etcd: startup probe failed for %s", c.Endpoints[0])
	}
	return &etcdRegistrar{
		client:    cli,
		keyPrefix: c.KeyPrefix,
		ttlSecs:   c.ttlSeconds(),
		holds:     map[string]*hold{},
	}, nil
}

// instanceID returns the instance id within the service: the caller-supplied ID,
// or a stable one derived from the service name and advertised address.
func instanceID(reg discovery.Registration) string {
	if reg.ID != "" {
		return reg.ID
	}
	return reg.ServiceName + "-" + reg.Addr
}

// keyFor returns the etcd key an instance is written under: prefix + service +
// "/" + instance id.
func (r *etcdRegistrar) keyFor(reg discovery.Registration) string {
	return r.keyPrefix + reg.ServiceName + "/" + instanceID(reg)
}

// Register grants a lease, writes the instance under it, and starts a keep-alive
// so the entry stays live until Deregister or process death. Registering the
// same instance again refreshes it: the previous lease is revoked first.
func (r *etcdRegistrar) Register(ctx context.Context, reg discovery.Registration) error {
	if err := starter.RequireField("registry-etcd", "addr", reg.Addr); err != nil {
		return err
	}
	val, err := json.Marshal(instanceValue{
		ServiceName: reg.ServiceName,
		Addr:        reg.Addr,
		Weight:      reg.Weight,
		Metadata:    reg.Metadata,
	})
	if err != nil {
		return errutil.Explain(err, "registry-etcd: marshal instance %q", reg.ServiceName)
	}

	grant, err := r.client.Grant(ctx, r.ttlSecs)
	if err != nil {
		return errutil.Explain(err, "registry-etcd: grant lease for %q", reg.ServiceName)
	}
	key := r.keyFor(reg)
	if _, err := r.client.Put(ctx, key, string(val), clientv3.WithLease(grant.ID)); err != nil {
		_, _ = r.client.Revoke(context.Background(), grant.ID)
		return errutil.Explain(err, "registry-etcd: put %q", key)
	}

	// KeepAlive runs until its context is cancelled (on Deregister). The returned
	// channel must be drained or the lease will not be renewed.
	kaCtx, cancel := context.WithCancel(context.Background())
	ka, err := r.client.KeepAlive(kaCtx, grant.ID)
	if err != nil {
		cancel()
		_, _ = r.client.Revoke(context.Background(), grant.ID)
		return errutil.Explain(err, "registry-etcd: keepalive for %q", reg.ServiceName)
	}
	go func() {
		for range ka {
			// Drain renewals; the lease is kept alive as long as we consume them.
		}
	}()

	r.mu.Lock()
	// Re-registering the same instance refreshes it: retire the old lease.
	if old, ok := r.holds[key]; ok {
		old.cancel()
		_, _ = r.client.Revoke(context.Background(), old.leaseID)
	}
	r.holds[key] = &hold{leaseID: grant.ID, cancel: cancel}
	r.mu.Unlock()
	return nil
}

// Deregister stops the keep-alive and revokes the lease, which deletes the key.
// It is idempotent: deregistering an instance that is not registered is a no-op.
func (r *etcdRegistrar) Deregister(ctx context.Context, reg discovery.Registration) error {
	key := r.keyFor(reg)
	r.mu.Lock()
	h, ok := r.holds[key]
	if ok {
		delete(r.holds, key)
	}
	r.mu.Unlock()
	if !ok {
		return nil
	}
	h.cancel()
	if _, err := r.client.Revoke(ctx, h.leaseID); err != nil {
		return errutil.Explain(err, "registry-etcd: revoke lease for %q", reg.ServiceName)
	}
	return nil
}
