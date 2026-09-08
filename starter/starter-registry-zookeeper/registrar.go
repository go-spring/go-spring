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

package StarterRegistryZookeeper

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/go-zookeeper/zk"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
)

// instance is the instance this process advertises to the ZooKeeper registry.
// It is the local write-side value built from RegistrationConfig in Server.Run;
// the crash-safety contract every registry starter follows lives in
// starter/DESIGN §3 (Register must self-renew so correctness never depends on
// Deregister).
type instance struct {
	ServiceName string
	ID          string
	Addr        string
	Weight      int
	Metadata    map[string]string
}

// instanceValue is the JSON payload stored at an instance znode. A discovery
// backend reading the same base path reconstructs an Endpoint from it.
type instanceValue struct {
	ServiceName string            `json:"service_name"`
	Addr        string            `json:"addr"`
	Weight      int               `json:"weight,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// zkRegistrar publishes instances to a ZooKeeper ensemble as ephemeral znodes.
// An ephemeral node lives only as long as the client session, so ZooKeeper
// removes it automatically when the process dies without Deregister. The
// mirror side of that contract: when the session dies but the process lives
// (ensemble restart, network partition longer than the session timeout), the
// nodes vanish too — so a monitor goroutine watches the session state and
// re-creates every registered node once the session is re-established, with
// exponential backoff. go-zookeeper/zk v1.0.4 exposes no state-change
// callback, so the monitor polls Conn.State once a second.
type zkRegistrar struct {
	conn     *zk.Conn
	basePath string
	acl      []zk.ACL

	// backoffBase/backoffCap pace the re-register retry loop after a session
	// recovery: 1s doubling up to 1min. Fields (not constants) so tests shrink
	// them.
	backoffBase time.Duration
	backoffCap  time.Duration

	// state and reRegister are the session monitor's seams: the real
	// implementations read Conn.State and re-run the node-creation step, and
	// tests replace them to drive loss/recovery without an ensemble.
	state      func() zk.State
	reRegister func(reg instance) error

	mu   sync.Mutex
	regs map[string]instance // znode path -> last advertised value

	done     chan struct{}
	doneOnce sync.Once
}

// Close stops the session monitor. It is idempotent. It does NOT close the
// ensemble connection — the owner (zkCenter, or the fallback connection built
// by NewServer) does.
func (r *zkRegistrar) Close() {
	r.doneOnce.Do(func() {
		close(r.done)
	})
}

// newZookeeperRegistrar returns a registrar writing through conn (the shared
// center connection; the ensemble was already probed when conn was built)
// with c's base path. It does NOT close conn — the owner (zkCenter) does.
func newZookeeperRegistrar(c ZookeeperConfig, conn *zk.Conn) (*zkRegistrar, error) {
	if conn == nil {
		return nil, errutil.Explain(nil, "registry-zookeeper: nil zookeeper connection")
	}
	r := &zkRegistrar{
		conn:        conn,
		basePath:    strings.TrimRight(c.BasePath, "/"),
		acl:         zk.WorldACL(zk.PermAll),
		backoffBase: time.Second,
		backoffCap:  time.Minute,
		regs:        map[string]instance{},
		done:        make(chan struct{}),
	}
	r.state = r.conn.State
	r.reRegister = func(reg instance) error { return r.createNode(reg) }
	go r.monitorSession()
	return r, nil
}

// instanceID returns the instance id within the service: the caller-supplied ID,
// or a stable one derived from the service name and advertised address.
func instanceID(reg instance) string {
	if reg.ID != "" {
		return reg.ID
	}
	return reg.ServiceName + "-" + reg.Addr
}

// pathFor returns the znode an instance is written to: basePath/service/id.
func (r *zkRegistrar) pathFor(reg instance) string {
	return r.basePath + "/" + reg.ServiceName + "/" + instanceID(reg)
}

// Register writes reg as an ephemeral znode, creating the persistent parent
// directories on demand. Re-registering the same instance replaces the node so
// the entry is refreshed rather than duplicated. The advertised value is also
// remembered so the session monitor can re-create the node (with its latest
// weight) after a session loss.
func (r *zkRegistrar) Register(_ context.Context, reg instance) error {
	if reg.Addr == "" {
		return errutil.Explain(nil, "registry-zookeeper: addr is required")
	}
	// An unset (or misconfigured negative) weight is normalized at write time
	// so "default" is never stored as 0 — 0 is reserved for the runtime drain
	// signal, only reachable through UpdateWeight.
	if reg.Weight <= 0 {
		reg.Weight = 1
	}
	if err := r.createNode(reg); err != nil {
		return err
	}
	r.mu.Lock()
	r.regs[r.pathFor(reg)] = reg
	r.mu.Unlock()
	return nil
}

// createNode performs the raw znode write: marshal, create persistent parents,
// then create (or replace) the ephemeral leaf. It is the single (re-)creation
// step used by Register and by the session monitor's recovery loop.
func (r *zkRegistrar) createNode(reg instance) error {
	val, err := json.Marshal(instanceValue{
		ServiceName: reg.ServiceName,
		Addr:        reg.Addr,
		Weight:      reg.Weight,
		Metadata:    reg.Metadata,
	})
	if err != nil {
		return errutil.Explain(err, "registry-zookeeper: marshal instance %q", reg.ServiceName)
	}

	path := r.pathFor(reg)
	if err := r.ensureParents(path); err != nil {
		return err
	}
	// An ephemeral node from a previous session may linger briefly; replace it so
	// a restart refreshes the entry instead of failing on ErrNodeExists.
	if _, err := r.conn.Create(path, val, zk.FlagEphemeral, r.acl); err != nil {
		if !errors.Is(err, zk.ErrNodeExists) {
			return errutil.Explain(err, "registry-zookeeper: create %q", path)
		}
		if err := r.conn.Delete(path, -1); err != nil && !errors.Is(err, zk.ErrNoNode) {
			return errutil.Explain(err, "registry-zookeeper: replace %q", path)
		}
		if _, err := r.conn.Create(path, val, zk.FlagEphemeral, r.acl); err != nil {
			return errutil.Explain(err, "registry-zookeeper: recreate %q", path)
		}
	}
	return nil
}

// UpdateWeight rewrites reg's znode with a new weight. The node is set in
// place — not deleted and recreated — so the ephemeral owner and the watchers
// are undisturbed and a discovery backend simply sees the new value. A weight
// of 0 is the drain signal: it serializes as an omitted weight field, which
// readers reconstruct as 0 and exclude from picking.
func (r *zkRegistrar) UpdateWeight(_ context.Context, reg instance, weight int) error {
	if weight < 0 {
		weight = 1
	}
	val, err := json.Marshal(instanceValue{
		ServiceName: reg.ServiceName,
		Addr:        reg.Addr,
		Weight:      weight,
		Metadata:    reg.Metadata,
	})
	if err != nil {
		return errutil.Explain(err, "registry-zookeeper: marshal instance %q", reg.ServiceName)
	}
	path := r.pathFor(reg)
	if _, stat, err := r.conn.Exists(path); err != nil {
		return errutil.Explain(err, "registry-zookeeper: stat %q", path)
	} else if stat == nil {
		return errutil.Explain(nil, "registry-zookeeper: update weight for unregistered instance %q", path)
	}
	if _, err := r.conn.Set(path, val, -1); err != nil {
		return errutil.Explain(err, "registry-zookeeper: update weight set %q", path)
	}
	// Remember the new weight so a post-recovery re-create advertises it.
	r.mu.Lock()
	if last, ok := r.regs[path]; ok {
		last.Weight = weight
		r.regs[path] = last
	}
	r.mu.Unlock()
	return nil
}

// Deregister removes the instance znode. It is idempotent: deregistering an
// instance that is not registered (ErrNoNode) is a no-op.
func (r *zkRegistrar) Deregister(_ context.Context, reg instance) error {
	path := r.pathFor(reg)
	r.mu.Lock()
	delete(r.regs, path)
	r.mu.Unlock()
	if err := r.conn.Delete(path, -1); err != nil && !errors.Is(err, zk.ErrNoNode) {
		return errutil.Explain(err, "registry-zookeeper: deregister %q", reg.ServiceName)
	}
	return nil
}

// monitorSession polls the connection state once a second. Leaving
// StateHasSession means the session is going away (or already expired) and the
// ephemeral nodes will vanish with it; returning to StateHasSession after a
// loss triggers re-creation of every registered node. Re-creating after a mere
// blip (a reconnect that never expired the session) is harmless: Register
// replaces the node, so the entry is refreshed rather than duplicated.
func (r *zkRegistrar) monitorSession() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	degraded := false
	for {
		select {
		case <-r.done:
			return
		case <-ticker.C:
			wasDegraded := degraded
			// Plain assignment, not :=, so degraded carries across ticks.
			var heal bool
			degraded, heal = reconcileSession(degraded, r.state())
			if degraded && !wasDegraded {
				log.Errorf(context.Background(), log.TagAppDef,
					"zookeeper session lost (state=%s); registered nodes are gone or going, they will be re-created once the session is re-established", r.state())
			}
			if heal {
				r.healAll()
			}
		}
	}
}

// reconcileSession folds the observed connection state into the degraded flag.
// It reports whether the session must be considered lost, and whether the
// session just recovered (a degraded session observed alive again) — the signal
// to re-create the registered nodes.
func reconcileSession(degraded bool, st zk.State) (stillDegraded, heal bool) {
	if st == zk.StateHasSession {
		return false, degraded
	}
	return true, false
}

// healAll re-creates every registered node after a session recovery. A failed
// attempt (the session is not usable yet) is retried with exponential backoff
// (1s doubling, capped at 1min) until all nodes are back or Close is called.
func (r *zkRegistrar) healAll() {
	backoff := r.backoffBase
	for {
		select {
		case <-r.done:
			return
		default:
		}
		if err := r.reRegisterAll(); err == nil {
			log.Infof(context.Background(), log.TagAppDef, "re-created registered zookeeper node(s) after session recovery")
			return
		} else {
			log.Errorf(context.Background(), log.TagAppDef,
				"re-create zookeeper node(s) failed: %v; retrying in %s", err, backoff)
		}
		select {
		case <-r.done:
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > r.backoffCap {
			backoff = r.backoffCap
		}
	}
}

// reRegisterAll re-runs the node-creation step for every tracked instance.
// The first failure aborts the pass; healAll retries the whole pass, and
// createNode is idempotent so partial progress is not a problem.
func (r *zkRegistrar) reRegisterAll() error {
	r.mu.Lock()
	regs := make([]instance, 0, len(r.regs))
	for _, reg := range r.regs {
		regs = append(regs, reg)
	}
	r.mu.Unlock()
	for _, reg := range regs {
		if err := r.reRegister(reg); err != nil {
			return err
		}
	}
	return nil
}

// ensureParents creates every persistent ancestor of path that does not yet
// exist (the leaf itself is created separately as ephemeral).
func (r *zkRegistrar) ensureParents(path string) error {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	cur := ""
	// Every segment except the last is a persistent directory znode.
	for _, seg := range segments[:len(segments)-1] {
		cur += "/" + seg
		ok, _, err := r.conn.Exists(cur)
		if err != nil {
			return errutil.Explain(err, "registry-zookeeper: stat parent %q", cur)
		}
		if ok {
			continue
		}
		if _, err := r.conn.Create(cur, nil, 0, r.acl); err != nil && !errors.Is(err, zk.ErrNodeExists) {
			return errutil.Explain(err, "registry-zookeeper: create parent %q", cur)
		}
	}
	return nil
}
