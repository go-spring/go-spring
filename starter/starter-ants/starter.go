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

package StarterAnts

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register multiple pools under ${spring.ants}. Each map key becomes a
	// named Pool bean. Observers registered via RegisterObserver are applied
	// to every pool so task submissions flow through the observer chain.
	//
	// We use gs.Module instead of gs.Group so that the pool's bean name is
	// available to pass to observers.
	gs.Module(gs.OnProperty("spring.ants"), func(r gs.BeanProvider, p flatten.Storage) error {
		var m map[string]Config
		if err := conf.Bind(p, &m, "${spring.ants}"); err != nil {
			return err
		}
		for name, c := range m {
			// createPool returns Pool (interface), but gs.Provide registers
			// the concrete type. Export(gs.As[Pool]()) makes it available
			// for autowire by the Pool interface.
			r.Provide(func() (Pool, error) {
				return createPool(name, c)
			}).Name(name).Destroy(destroyPool)
		}
		return nil
	})

	// Register the built-in metrics observer. It implements PoolObserver and
	// self-registers via RegisterObserver so createPool picks it up. Users can
	// autowire *MetricsObserver to read aggregated stats.
	gs.Provide(newMetricsObserver).Export(gs.As[PoolObserver]())
}

// createPool resolves the configured Driver and wraps the resulting pool
// with all registered observers for the given name.
func createPool(name string, c Config) (Pool, error) {
	d, ok := driverRegistry[c.Driver]
	if !ok {
		return nil, errutil.Explain(nil, "ants driver not found: %s", c.Driver)
	}
	pool, err := d.CreatePool(c)
	if err != nil {
		return nil, err
	}
	// Wrap the pool's Submit to route through the observer chain.
	return &observedPool{
		Pool: pool,
		name: name,
	}, nil
}

// destroyPool releases the pool, stopping its background purge goroutine
// and reclaiming all workers.
func destroyPool(pool Pool) error {
	pool.Release()
	return nil
}

// ---------------------------------------------------------------------------
// Pool — abstract goroutine pool
// ---------------------------------------------------------------------------

// Pool is the abstract interface for a goroutine pool. It exposes the minimal
// surface needed to submit work and observe pool state, without tying callers
// to *ants.Pool. This lets users swap pool implementations (or wrap them via a
// custom Driver) without changing autowire targets.
type Pool interface {
	// Submit enqueues a task. Returns an error when the pool is closed or
	// overloaded (in nonblocking mode).
	Submit(task func()) error

	// Running returns the number of currently executing workers.
	Running() int

	// Free returns the number of idle workers, or -1 for unbounded pools.
	Free() int

	// Cap returns the pool capacity, or -1 for unbounded pools.
	Cap() int

	// Waiting returns the number of tasks blocked waiting for a worker.
	Waiting() int

	// Release closes the pool and releases all workers.
	Release()
}

// observedPool wraps a Pool, passing every Submit through the observer chain.
type observedPool struct {
	Pool
	name string
}

func (p *observedPool) Submit(task func()) error {
	return p.Pool.Submit(wrapTask(p.name, task))
}

func (p *observedPool) String() string {
	return fmt.Sprintf("pool:%s", p.name)
}

// ---------------------------------------------------------------------------
// PoolObserver — task lifecycle callbacks
// ---------------------------------------------------------------------------

// PoolObserver observes task lifecycle events on a pool. Multiple observers
// can be registered as beans implementing this interface — the starter
// collects them and wraps every task through the chain.
//
// The OnSubmit callback is the key extension point: it receives the raw task
// and returns a (possibly wrapped) task. Typical use cases:
//
//   - Metrics: wrap task to record start/end time, count running tasks.
//   - Tracing: wrap task to inject a span context.
//   - Logging: wrap task to log duration or panics.
//   - Rate limiting: wrap task to apply a per-pool rate limiter.
type PoolObserver interface {
	// OnSubmit wraps a task before it is submitted to the pool.
	// name is the pool's bean name (the map key under spring.ants).
	// The returned func() replaces the original task; return the original
	// unchanged for pass-through.
	OnSubmit(name string, task func()) func()
}

// ---------------------------------------------------------------------------
// Observer registry — bridge between gs.Provide beans and pool creation
// ---------------------------------------------------------------------------

var (
	observerMu    sync.RWMutex
	observerBeans []PoolObserver
)

// RegisterObserver registers a PoolObserver. Called automatically by
// gs.Provide when a PoolObserver bean is created; users can also call it
// directly before the container starts to register non-bean observers.
func RegisterObserver(o PoolObserver) {
	observerMu.Lock()
	defer observerMu.Unlock()
	observerBeans = append(observerBeans, o)
}

// Observers returns a snapshot of all registered PoolObservers.
func Observers() []PoolObserver {
	observerMu.RLock()
	defer observerMu.RUnlock()
	out := make([]PoolObserver, len(observerBeans))
	copy(out, observerBeans)
	return out
}

// wrapTask passes a task through all registered observers and returns the
// final wrapped task. When no observers are registered, returns the original.
func wrapTask(name string, task func()) func() {
	observers := Observers()
	wrapped := task
	for _, o := range observers {
		wrapped = o.OnSubmit(name, wrapped)
	}
	return wrapped
}

// ---------------------------------------------------------------------------
// PoolStats — aggregated metrics snapshot
// ---------------------------------------------------------------------------

// PoolStat holds a point-in-time snapshot of a single pool's metrics.
type PoolStat struct {
	Name    string `json:"name"`
	Cap     int    `json:"cap"`
	Running int    `json:"running"`
	Waiting int    `json:"waiting"`
	Free    int    `json:"free"`
}

// PoolStats is the aggregated snapshot of all managed pools.
type PoolStats struct {
	Pools []PoolStat `json:"pools"`
	Time  time.Time  `json:"time"`
}

// MetricsObserver is the built-in PoolObserver that tracks per-pool running
// counts and exposes an aggregated snapshot on demand.
//
// Usage:
//
//	type MyService struct {
//	    Metrics *StarterAnts.MetricsObserver `autowire:""`
//	}
//
//	func (s *MyService) handleMetrics(w http.ResponseWriter, r *http.Request) {
//	    stats := s.Metrics.Snapshot()
//	    json.NewEncoder(w).Encode(stats)
//	}
type MetricsObserver struct {
	mu    sync.RWMutex
	pools map[string]*poolMetrics
}

// poolMetrics holds the counters for a single pool.
type poolMetrics struct {
	running atomic.Int64
}

func newMetricsObserver() *MetricsObserver {
	m := &MetricsObserver{
		pools: make(map[string]*poolMetrics),
	}
	// Self-register so createPool picks it up.
	RegisterObserver(m)
	return m
}

// OnSubmit wraps the task to track the running count. The observer only
// tracks Running; Cap/Free/Waiting are read directly from the pool at
// snapshot time.
func (m *MetricsObserver) OnSubmit(name string, task func()) func() {
	pm := m.getOrCreate(name)
	return func() {
		pm.running.Add(1)
		defer pm.running.Add(-1)
		task()
	}
}

func (m *MetricsObserver) getOrCreate(name string) *poolMetrics {
	m.mu.RLock()
	pm, ok := m.pools[name]
	m.mu.RUnlock()
	if ok {
		return pm
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	// Double-check after acquiring write lock.
	if pm, ok = m.pools[name]; ok {
		return pm
	}
	pm = &poolMetrics{}
	m.pools[name] = pm
	return pm
}

// Snapshot scans all known pools and returns the latest aggregated metrics.
// Cap/Free/Waiting are zero — call Enrich with the actual pools to fill them.
func (m *MetricsObserver) Snapshot() PoolStats {
	m.mu.RLock()
	names := make([]string, 0, len(m.pools))
	for name := range m.pools {
		names = append(names, name)
	}
	m.mu.RUnlock()

	stats := PoolStats{
		Pools: make([]PoolStat, 0, len(names)),
		Time:  time.Now(),
	}
	for _, name := range names {
		m.mu.RLock()
		pm := m.pools[name]
		m.mu.RUnlock()
		stats.Pools = append(stats.Pools, PoolStat{
			Name:    name,
			Running: int(pm.running.Load()),
		})
	}
	return stats
}

// Enrich fills in Cap/Free/Waiting by reading from the actual pools.
func (m *MetricsObserver) Enrich(stats *PoolStats, pools map[string]Pool) {
	for i := range stats.Pools {
		name := stats.Pools[i].Name
		if p, ok := pools[name]; ok {
			stats.Pools[i].Cap = p.Cap()
			stats.Pools[i].Free = p.Free()
			stats.Pools[i].Waiting = p.Waiting()
		}
	}
}
