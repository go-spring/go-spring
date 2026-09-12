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
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register multiple pools under ${spring.ants}. Each map key becomes a
	// named Pool bean. Observers are collected from the container: every bean
	// exported as PoolObserver is injected into each pool's constructor as a
	// []PoolObserver, so task submissions flow through the observer chain.
	//
	// We use gs.Module instead of gs.Group so that the pool's bean name is
	// available to pass to observers.
	gs.Module(gs.OnProperty("spring.ants.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.ants.instances}", func(name string, c Config) error {
			// createPool returns Pool (interface), but gs.Provide registers
			// the concrete type. Export(gs.As[Pool]()) makes it available
			// for autowire by the Pool interface.
			//
			// The observers parameter is a container-collected collection of
			// every PoolObserver bean; declaring it here forces those beans to
			// be instantiated before any pool, and lets createPool snapshot the
			// chain at build time. "?" makes the collection nullable so a pool
			// still builds when the list is empty. The Driver bean (index 1) is
			// selected by the entry's ${driver} key: unset → "?" (nullable
			// by-type — injects the single Driver bean when one is provided,
			// nil otherwise, and createPool falls back to the bundled
			// DefaultDriver); set → that bean name, and naming a bean that
			// does not exist fails loud.
			r.Provide(func(ctx *gs.ContextProvider, d Driver, observers []PoolObserver) (Pool, error) {
				return createPool(ctx.Context, name, c, d, observers)
			}, gs.IndexArg(1, gs.TagArg("${spring.ants.instances."+name+".driver:=${spring.ants.default.driver:=?}}")), gs.IndexArg(2, gs.TagArg("?"))).Name(name).Destroy(destroyPool)
			return nil
		})
	})

	// Register the built-in metrics observer as a PoolObserver bean. The
	// container collects it into every pool's observer chain; users can also
	// autowire *MetricsObserver directly to read aggregated stats.
	gs.Provide(newMetricsObserver).Export(gs.As[PoolObserver]())
}

// createPool builds the pool through the supplied Driver, falling back to the
// bundled DefaultDriver when no Driver bean is present, and wraps the
// resulting pool with the observer chain folded from observers.
func createPool(ctx context.Context, name string, c Config, d Driver, observers []PoolObserver) (Pool, error) {
	log.Debugf(ctx, log.TagAppDef, "creating ants pool %q, size=%d", name, c.Size)

	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	pool, err := d.CreatePool(c)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "ants: create pool %q failed: %v", name, err)
		return nil, err
	}
	log.Infof(ctx, log.TagAppDef, "ants pool %q initialized, size=%d", name, c.Size)
	// Wrap the pool's Submit to route through the observer chain, snapshotting
	// the resolved observers once at build time.
	return &observedPool{
		Pool:  pool,
		name:  name,
		chain: foldObservers(name, observers),
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

// observedPool wraps a Pool, passing every Submit through the observer chain
// folded from the observer beans at build time.
type observedPool struct {
	Pool
	name  string
	chain func(task func()) func()
}

func (p *observedPool) Submit(task func()) error {
	return p.Pool.Submit(p.chain(task))
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

// foldObservers folds the resolved observer beans into a single wrap function
// that applies every observer's OnSubmit in order, with the caller's original
// task innermost. Observers come from the container as a collected []PoolObserver
// and are snapshotted once at pool build time, so the chain is fixed for the
// pool's lifetime. With no observers it returns a pass-through wrapper.
func foldObservers(name string, observers []PoolObserver) func(task func()) func() {
	return func(task func()) func() {
		wrapped := task
		for _, o := range observers {
			wrapped = o.OnSubmit(name, wrapped)
		}
		return wrapped
	}
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
	return &MetricsObserver{
		pools: make(map[string]*poolMetrics),
	}
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
