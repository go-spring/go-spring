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
	"io"
	"runtime"

	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/spring/cloud/discovery"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register one Kubernetes discovery backend per entry under
	// "${spring.discovery.k8s}", keyed by name. The registration happens in the
	// module callback (the bean-registration phase), which runs before any
	// client starter's bean constructor — so by the time a Redis/GORM client
	// calls discovery.MustGet("<name>") the backend is already present.
	//
	// A backend is *not* an injectable bean; like a discovery adapter it lives
	// in the stdlib/discovery registry. We register a single lifecycle bean
	// (manager) purely so informer goroutines are stopped on shutdown.
	_, file, line, _ := runtime.Caller(0)
	gs.Module(gs.OnProperty("spring.discovery.k8s"), func(r gs.BeanProvider, p flatten.Storage) error {
		var m map[string]Config
		if err := conf.Bind(p, &m, "${spring.discovery.k8s}"); err != nil {
			return err
		}
		mgr := &manager{}
		for name, c := range m {
			// Skip a name already claimed by another adapter (e.g. a company's
			// own discovery.Register), rather than panicking on the duplicate.
			if _, ok := discovery.Get(name); ok {
				return errutil.Explain(nil, "discovery-k8s: backend %q already registered", name)
			}
			b, err := newBackend(c)
			if err != nil {
				return errutil.Explain(err, "discovery-k8s: build backend %q", name)
			}
			discovery.Register(name, b)
			mgr.add(b)
		}
		bean := r.Provide(func() *manager { return mgr }).Destroy((*manager).Stop)
		bean.SetFileLine(file, line)
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
func (m *manager) Stop() error {
	for _, b := range m.backends {
		if c, ok := b.(io.Closer); ok {
			_ = c.Close()
		}
	}
	return nil
}
