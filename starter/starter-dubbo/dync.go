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
	"strconv"
	"sync"
	"time"

	"go-spring.org/spring/gs"
	mapconfig "go-spring.org/starter-dubbo/internal/mapconfig"
)

const pollInterval = 5 * time.Second

func init() {
	gs.Provide(newDyncPoller, gs.IndexArg(0, gs.TagArg("${spring.dubbo.application}")))
}

// dyncPoller watches ${spring.dubbo.consumer} (the entire consumer node:
// consumer-level defaults + per-reference overrides + per-method tuning) via
// gs.Dync. When the config changes (hot-reload), the poller diffs against the
// last snapshot and pushes the dynamically-applicable fields into the in-memory
// config center as flat dubbo URL params.
//
// Dynamic fields are those dubbo-go reads from URL params at call time:
// timeout, retries, loadbalance, cluster, group, version, serialization,
// sticky, force.tag, weight, and per-method tps/execute tuning.
// Filter is NOT dynamic — it is frozen into the invoker chain at Refer time.
type dyncPoller struct {
	dc      *mapconfig.MapDynamicConfiguration
	appName string

	// Consumer is the entire consumer node under ${spring.dubbo.consumer},
	// hot-reloaded by go-spring on RefreshProperties.
	Consumer gs.Dync[DubboConsumer] `value:"${spring.dubbo.consumer}"`

	mu   sync.Mutex
	last map[string]map[string]string
	done chan struct{}
}

// newDyncPoller creates the poller bean.
func newDyncPoller(app DubboApplication) *dyncPoller {
	return &dyncPoller{
		dc:      mapconfig.Singleton(),
		appName: app.Name,
		last:    make(map[string]map[string]string),
		done:    make(chan struct{}),
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
	consumer := p.Consumer.Value()

	rules := consumerToOverrideRules(p.appName, &consumer)

	if len(rules) == 0 {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.changed(rules) {
		return
	}

	p.dc.RefreshOverrideRules(rules)
}

// consumerToOverrideRules converts a DubboConsumer into the flat
// map[string]map[string]string format expected by RefreshOverrideRules.
//
// Consumer-level defaults are published as an application-level override
// (<appName>.configurators), picked up by consumerConfigurationListener.
// Each reference with a non-empty Interface is published as a service-level
// override (<interface>:<version>:<group>.configurators), picked up by
// referenceConfigurationListener, so each reference gets independent overrides
// instead of being merged into a single last-wins rule.
func consumerToOverrideRules(appName string, c *DubboConsumer) map[string]map[string]string {
	rules := make(map[string]map[string]string)

	// Consumer-level defaults → application-level override.
	appParams := make(map[string]string)
	addIfSet(appParams, "timeout", c.RequestTimeout)
	addIfSet(appParams, "retries", retriesStr(c.Retries))
	addIfSet(appParams, "loadbalance", c.LoadBalance)
	addIfSet(appParams, "cluster", c.Cluster)
	addIfSet(appParams, "group", c.Group)
	addIfSet(appParams, "version", c.Version)
	addIfSet(appParams, "serialization", c.Serialization)
	if c.Sticky {
		appParams["sticky"] = "true"
	}
	if c.ForceTag {
		appParams["force.tag"] = "true"
	}
	if len(appParams) > 0 {
		rules[appName] = appParams
	}

	// Per-reference overrides → service-level override (one per reference).
	for _, ref := range c.References {
		if ref.Interface == "" {
			continue
		}
		// Each reference gets its own service-level override keyed by
		// the interface name. Version and group are set as params on
		// the reference, not baked into the lookup key.
		key := ref.Interface

		refParams := make(map[string]string)
		addIfSet(refParams, "timeout", ref.Timeout)
		addIfSet(refParams, "retries", retriesStrOverride(ref.Retries))
		addIfSet(refParams, "loadbalance", ref.LoadBalance)
		addIfSet(refParams, "cluster", ref.Cluster)
		addIfSet(refParams, "group", ref.Group)
		addIfSet(refParams, "version", ref.Version)
		addIfSet(refParams, "serialization", ref.Serialization)
		if ref.Sticky {
			refParams["sticky"] = "true"
		}
		if ref.ForceTag {
			refParams["force.tag"] = "true"
		}
		for methodName, m := range ref.Methods {
			prefix := "methods." + methodName + "."
			addIfSet(refParams, prefix+"timeout", m.Timeout)
			addIfSet(refParams, prefix+"retries", retriesStrOverride(m.Retries))
			addIfSet(refParams, prefix+"loadbalance", m.LoadBalance)
			addIfSet(refParams, prefix+"weight", strconv.FormatInt(m.Weight, 10))
			addIfSet(refParams, prefix+"sticky", boolToStr(m.Sticky))
			addIfSet(refParams, prefix+"tps.limit.interval", strconv.Itoa(m.TpsLimitInterval))
			addIfSet(refParams, prefix+"tps.limit.rate", strconv.Itoa(m.TpsLimitRate))
			addIfSet(refParams, prefix+"tps.limit.strategy", m.TpsLimitStrategy)
			addIfSet(refParams, prefix+"execute.limit", strconv.Itoa(m.ExecuteLimit))
			addIfSet(refParams, prefix+"execute.limit.rejected.handler", m.ExecuteLimitRejectedHandler)
		}
		if len(refParams) > 0 {
			rules[key] = refParams
		}
	}

	return rules
}

func addIfSet(m map[string]string, key, val string) {
	if val != "" {
		m[key] = val
	}
}

// retriesStr returns the retries value as a string, or empty for 0 (not set).
// Consumer-level defaults use this — retries=0 means "not configured".
func retriesStr(r int) string {
	if r <= 0 {
		return ""
	}
	return strconv.Itoa(r)
}

// retriesStrOverride returns the retries value as a string, allowing 0.
// Per-reference and per-method overrides use this — retries=0 means "disable retries".
func retriesStrOverride(r int) string {
	if r < 0 {
		return ""
	}
	return strconv.Itoa(r)
}

func boolToStr(b bool) string {
	if b {
		return "true"
	}
	return ""
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