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

	"github.com/go-zookeeper/zk"
	"go-spring.org/spring/discovery"
	"go-spring.org/stdlib/errutil"
)

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
// removes it automatically when the process dies without Deregister.
// It implements discovery.Registrar.
type zkRegistrar struct {
	conn     *zk.Conn
	basePath string
	acl      []zk.ACL
}

// newZookeeperRegistrar connects to the ensemble and returns a registrar. The
// connection is verified with a probe so an unreachable ensemble fails startup
// rather than surfacing on the first Register.
func newZookeeperRegistrar(c ZookeeperConfig) (*zkRegistrar, error) {
	if len(c.Servers) == 0 {
		return nil, errutil.Explain(nil, "registry-zookeeper: servers is required")
	}
	conn, _, err := zk.Connect(c.Servers, c.SessionTimeout)
	if err != nil {
		return nil, errutil.Explain(err, "registry-zookeeper: connect to %v", c.Servers)
	}
	if c.Username != "" || c.Password != "" {
		if err := conn.AddAuth("digest", []byte(c.Username+":"+c.Password)); err != nil {
			conn.Close()
			return nil, errutil.Explain(err, "registry-zookeeper: add digest auth")
		}
	}
	// Fail-fast probe: an Exists call blocks until the session connects (or the
	// session timeout elapses), so an unreachable ensemble surfaces at boot.
	if _, _, err := conn.Exists("/"); err != nil {
		conn.Close()
		return nil, errutil.Explain(err, "registry-zookeeper: startup probe failed for %v", c.Servers)
	}
	return &zkRegistrar{
		conn:     conn,
		basePath: strings.TrimRight(c.BasePath, "/"),
		acl:      zk.WorldACL(zk.PermAll),
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

// pathFor returns the znode an instance is written to: basePath/service/id.
func (r *zkRegistrar) pathFor(reg discovery.Registration) string {
	return r.basePath + "/" + reg.ServiceName + "/" + instanceID(reg)
}

// Register writes reg as an ephemeral znode, creating the persistent parent
// directories on demand. Re-registering the same instance replaces the node so
// the entry is refreshed rather than duplicated.
func (r *zkRegistrar) Register(_ context.Context, reg discovery.Registration) error {
	if reg.Addr == "" {
		return errutil.Explain(nil, "registry-zookeeper: addr is required")
	}
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

// Deregister removes the instance znode. It is idempotent: deregistering an
// instance that is not registered (ErrNoNode) is a no-op.
func (r *zkRegistrar) Deregister(_ context.Context, reg discovery.Registration) error {
	path := r.pathFor(reg)
	if err := r.conn.Delete(path, -1); err != nil && !errors.Is(err, zk.ErrNoNode) {
		return errutil.Explain(err, "registry-zookeeper: deregister %q", reg.ServiceName)
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
