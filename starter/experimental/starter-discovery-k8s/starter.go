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
	"io"

	"go-spring.org/cloud/discovery"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register one Kubernetes discovery backend per entry under
	// "${spring.discovery.k8s}", keyed by name. The registration happens in the
	// module callback (the bean-registration phase), which runs before any
	// client starter's bean constructor — so by the time a Redis/GORM client
	// calls discovery.GetDiscovery("<name>") the backend is already present.
	//
	// A backend is *not* an injectable bean; like a discovery adapter it lives
	// in the cloud/discovery registry. We register a single lifecycle bean
	// (manager) purely so informer goroutines are stopped on shutdown.
	gs.Module(gs.OnProperty("spring.discovery.k8s"), func(r gs.BeanProvider, p flatten.Storage) error {
		mgr := &manager{}
		if err := conf.BindEach(p, "${spring.discovery.k8s}", func(name string, c Config) error {
			// Skip a name already claimed by another adapter (e.g. a company's
			// own discovery.Register), rather than panicking on the duplicate.
			if _, err := discovery.GetDiscovery(name); err == nil {
				return errutil.Explain(nil, "discovery-k8s: backend %q already registered", name)
			}
			log.Debugf(context.Background(), log.TagAppDef, "creating k8s discovery backend name=%s mode=%s namespace=%s", name, c.Mode, c.Namespace)
			b, err := newBackend(&gs.ContextProvider{Context: context.Background()}, c)
			if err != nil {
				return errutil.Explain(err, "discovery-k8s: build backend %q", name)
			}
			discovery.RegisterDiscovery(name, b)
			log.Infof(context.Background(), log.TagAppDef, "registered k8s discovery backend name=%s mode=%s", name, c.Mode)
			mgr.add(b)
			return nil
		}); err != nil {
			return err
		}
		r.Provide(func() *manager { return mgr }).Destroy((*manager).Destroy).Caller(1)
		return nil
	})
}

// manager owns the backends registered by this starter so their background
// resources (client-go informers) are released on container shutdown. dns-mode
// backends hold nothing to close; endpointslice-mode backends expose Close.
type manager struct {
	backends []discovery.Discovery
}

func (m *manager) add(b discovery.Discovery) {
	m.backends = append(m.backends, b)
}

// Stop closes every backend that owns background resources. It is the bean
// destructor, invoked once by the container on shutdown.
func (m *manager) Destroy() error {
	for _, b := range m.backends {
		if c, ok := b.(io.Closer); ok {
			_ = c.Close()
		}
	}
	return nil
}
