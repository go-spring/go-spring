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

package StarterDubbo

import (
	"maps"
	"sync"
	"time"

	"go-spring.org/spring/gs"
	mapconfig "go-spring.org/starter-dubbo/internal/mapconfig"
)

const pollInterval = 5 * time.Second

func init() {
	gs.Provide(newDyncPoller)
}

// dyncPoller periodically reads its Dync-backed rules and pushes changes into
// the in-memory config center via RefreshOverrideRules.
//
// The Refs field is a gs.Dync[map[string]map[string]string] bound from
// ${spring.dubbo.client.references}. go-spring hot-reloads it on
// RefreshProperties; the poller diffs against the last known snapshot
// on each tick and only pushes when something changed.
type dyncPoller struct {
	dc *mapconfig.MapDynamicConfiguration

	// Refs is the single Dync-wrapped map of all dynamic reference overrides.
	// Outer key = reference name, inner map = dubbo URL params (e.g. "timeout",
	// "retries", "methods.X.timeout"). Empty maps are skipped.
	//
	// Example configuration:
	//
	//	spring.dubbo.client.references.greet.timeout=3s
	//	spring.dubbo.client.references.greet.retries=2
	//	spring.dubbo.client.references.greet.methods.GetUser.timeout=1s
	Refs gs.Dync[map[string]map[string]string] `value:"${spring.dubbo.client.references}"`

	mu   sync.Mutex
	last map[string]map[string]string
	done chan struct{}
}

// newDyncPoller creates the poller bean.
func newDyncPoller() *dyncPoller {
	return &dyncPoller{
		dc:   mapconfig.Singleton(),
		last: make(map[string]map[string]string),
		done: make(chan struct{}),
	}
}

// Init starts the background polling goroutine.
func (p *dyncPoller) Init() error {
	go p.loop()
	return nil
}

// Close stops the polling goroutine.
func (p *dyncPoller) Close() error {
	close(p.done)
	return nil
}

func (p *dyncPoller) loop() {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	p.poll()
	for {
		select {
		case <-ticker.C:
			p.poll()
		case <-p.done:
			return
		}
	}
}

func (p *dyncPoller) poll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	rules := p.Refs.Value()
	if len(rules) == 0 {
		return
	}

	if !p.changed(rules) {
		return
	}

	p.dc.RefreshOverrideRules(rules)
}

// changed returns true if rules differ from the last known snapshot,
// and updates the snapshot atomically.
func (p *dyncPoller) changed(rules map[string]map[string]string) bool {
	if len(rules) != len(p.last) {
		p.last = maps.Clone(rules)
		return true
	}
	for k, v := range rules {
		if !maps.Equal(v, p.last[k]) {
			p.last = maps.Clone(rules)
			return true
		}
	}
	return false
}

// snapshotLast returns a copy of the last-known snapshot for testing.
func (p *dyncPoller) snapshotLast() map[string]map[string]string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return maps.Clone(p.last)
}
