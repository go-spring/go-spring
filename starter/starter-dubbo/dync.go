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
	"strings"
	"sync"
	"time"

	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/resilience"
	"go-spring.org/spring/gs"
	mapconfig "go-spring.org/starter-dubbo/internal/mapconfig"
)

// dyncPoller watches ${spring.dubbo.consumer} (the entire consumer node:
// consumer-level defaults + per-reference overrides + per-method tuning) via
// gs.Dync. On each hot-reload (delivered as a Dync change callback), it diffs
// against the last snapshot and pushes the dynamically-applicable fields into
// the in-memory config center as flat dubbo URL params.
//
// Note the two dynamic mechanisms at play here: this Dync binds ORDINARY app
// config (dubbo's own consumer settings), NOT the governance Source contract —
// governance policy (timeout/retries from the center) flows in through the
// injected resilience manager (Enabled/ClientPolicyFor/Subscribe) and is merged into
// the same override push in consumerToOverrideRules.
//
// Dynamic fields are those dubbo-go reads from URL params at call time:
// timeout, retries, loadbalance, cluster, group, version, serialization,
// sticky, force.tag, weight, and per-method tps/execute tuning.
// Filter is NOT dynamic — it is frozen into the invoker chain at Refer time.
type dyncPoller struct {
	dynCfg  *mapconfig.MapDynamicConfiguration // in-memory config center overrides are pushed into
	appName string                             // application name, used as the app-level override key

	// mgr is the governance starter's resilience manager bean, nil when
	// no center is linked. It is the module authority the poller
	// reads policies from and subscribes to for hot-reload. A nil (or disabled)
	// manager leaves governance off: no overrides are merged, and the
	// ${spring.dubbo.consumer}-only behavior stands.
	mgr *resilience.Manager

	// ctr is the governance starter's center bean, nil when the governance center is
	// not imported. It is used only for its ready signal — gs wires Rooters before
	// Runners, so this poller may initialize before the center is live; OnReady
	// re-runs the poll once governance is armed.
	ctr *governance.Center

	// Consumer is the entire consumer node under ${spring.dubbo.consumer},
	// hot-reloaded by go-spring on RefreshProperties.
	Consumer gs.Dync[DubboConsumer] `value:"${spring.dubbo.consumer}"`

	mu     sync.Mutex                   // guards last and regged
	last   map[string]map[string]string // last pushed override snapshot, for change detection
	regged map[string]bool              // dubbo service labels already subscribed on the center
}

// newDyncPoller creates the poller bean. mgr and ctr are the governance beans
// the governance center provides; both are nil when it is not imported, which leaves
// the poller on the legacy ${spring.dubbo.consumer}-only behavior.
func newDyncPoller(app DubboApplication, mgr *resilience.Manager, ctr *governance.Center) *dyncPoller {
	return &dyncPoller{
		dynCfg:  mapconfig.Singleton(),
		appName: app.Name,
		mgr:     mgr,
		ctr:     ctr,
		last:    make(map[string]map[string]string),
		regged:  make(map[string]bool),
	}
}

// Init registers a change callback on the consumer Dync and pushes the current
// override rules once. Subsequent hot-reloads fire the callback, which re-runs
// poll; poll's internal diff skips no-op refreshes.
//
// gs wires Rooters before Runners, and the governance center is a Rooter while
// this poller is one too — so the first poll below can run BEFORE the center has
// gone live (mgr.Enabled is false while the authority is unarmed).
// ctr.OnReady guarantees a re-poll once governance goes live, so overrides are
// still pushed at startup regardless of init ordering; that re-poll is also what
// first subscribes the poller to the center's policy changes.
func (p *dyncPoller) Init() error {
	p.Consumer.OnChanged(func(_, _ DubboConsumer) { p.poll() })
	if p.ctr != nil {
		p.ctr.OnReady(func() { p.poll() })
	}
	p.poll() // initial push: OnChanged does not fire on the init bind
	return nil
}

func (p *dyncPoller) poll() {
	consumer := p.Consumer.Value()

	rules := p.consumerToOverrideRules(&consumer)

	// Subscribe to governance policy changes for each dubbo service label we
	// publish, so a governance hot-reload re-runs poll and re-pushes the merged
	// rules. Collected under p.mu (dedup via regged) but subscribed OUTSIDE the
	// lock: Subscribe arms its callback synchronously, and that callback re-enters
	// poll - holding p.mu across Subscribe would self-deadlock. The re-entrant poll
	// is a no-op (changed() finds the same snapshot). When the governance center is
	// absent (nil manager) or governance is not armed (Enabled false) the whole
	// block is skipped.
	if p.mgr != nil && p.mgr.Enabled() {
		labels := dubboServiceLabels(p.appName, &consumer)
		var toReg []string
		p.mu.Lock()
		for l := range labels {
			if !p.regged[l] {
				p.regged[l] = true
				toReg = append(toReg, l)
			}
		}
		p.mu.Unlock()
		for _, l := range toReg {
			l := l
			p.mgr.Subscribe(l, func(resilience.ClientPolicy) { p.poll() })
		}
	}

	if len(rules) == 0 {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.changed(rules) {
		return
	}

	p.dynCfg.RefreshOverrideRules(rules)
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
func (p *dyncPoller) consumerToOverrideRules(c *DubboConsumer) map[string]map[string]string {
	appName := p.appName
	// governed is read once: the same "is governance on?" answer gates both the
	// app-level and the per-reference override, and the manager's Enabled is a
	// locked read.
	governed := p.mgr != nil && p.mgr.Enabled()
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
	if governed {
		applyGovernOverride(appParams, p.mgr.ClientPolicyFor(dubboAppLabel(appName)))
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
		// colonSeparatedKey(interface, version, group) — the exact key dubbo-go's
		// referenceConfigurationListener subscribes to (url.ColonSeparatedKey() +
		// ".configurators"). Version/group ARE baked into the lookup key (a bare
		// interface name never matches: ColonSeparatedKey always emits the two ":"
		// separators), in addition to being set as params below.
		key := colonSeparatedKey(ref.Interface, ref.Version, ref.Group)

		refParams := make(map[string]string)
		addIfSet(refParams, "timeout", ref.Timeout)
		addIfSet(refParams, "retries", retriesStr(ref.Retries))
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
			addIfSet(refParams, prefix+"retries", retriesStr(m.Retries))
			addIfSet(refParams, prefix+"loadbalance", m.LoadBalance)
			addIfSet(refParams, prefix+"weight", strconv.FormatInt(m.Weight, 10))
			addIfSet(refParams, prefix+"sticky", boolToStr(m.Sticky))
			addIfSet(refParams, prefix+"tps.limit.interval", strconv.Itoa(m.TpsLimitInterval))
			addIfSet(refParams, prefix+"tps.limit.rate", strconv.Itoa(m.TpsLimitRate))
			addIfSet(refParams, prefix+"tps.limit.strategy", m.TpsLimitStrategy)
			addIfSet(refParams, prefix+"execute.limit", strconv.Itoa(m.ExecuteLimit))
			addIfSet(refParams, prefix+"execute.limit.rejected.handler", m.ExecuteLimitRejectedHandler)
		}
		if governed {
			applyGovernOverride(refParams, p.mgr.ClientPolicyFor(dubboRefLabel(key)))
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

// colonSeparatedKey builds the service-level override lookup key the same way
// dubbo-go's referenceConfigurationListener derives its subscription key
// (common.URL.ColonSeparatedKey): "{interface}:{version}:{group}", where version
// is omitted when empty or the "0.0.0" sentinel but its ":" separator is always
// written, and group is omitted when empty but its ":" separator is always
// written. An override must be published under exactly this key (the
// .configurators suffix is appended by RefreshOverrideRules) or the per-reference
// listener — which subscribes to ColonSeparatedKey()+".configurators" — never
// receives it.
func colonSeparatedKey(intf, version, group string) string {
	var b strings.Builder
	b.WriteString(intf)
	b.WriteByte(':')
	if version != "" && version != "0.0.0" {
		b.WriteString(version)
	}
	b.WriteByte(':')
	if group != "" {
		b.WriteString(group)
	}
	return b.String()
}

// dubboAppLabel / dubboRefLabel are the governance service labels for dubbo's
// two override levels: the application (consumer defaults) and each reference.
// They are the keys passed to Manager.ClientPolicyFor and Manager.Subscribe so one
// governance rules document can tune dubbo's timeout/retry alongside every other
// client.
func dubboAppLabel(appName string) string  { return "dubbo:" + appName }
func dubboRefLabel(colonKey string) string { return "dubbo:" + colonKey }

// dubboServiceLabels returns every dubbo service label the poller should
// subscribe to for the given consumer: the app label (always) plus one per
// reference with a non-empty Interface.
func dubboServiceLabels(appName string, c *DubboConsumer) map[string]bool {
	out := map[string]bool{dubboAppLabel(appName): true}
	for _, ref := range c.References {
		if ref.Interface != "" {
			out[dubboRefLabel(colonSeparatedKey(ref.Interface, ref.Version, ref.Group))] = true
		}
	}
	return out
}

// applyGovernOverride overrides timeout and retries in params from the
// governance policy. Policy.Timeout (a Duration) is written as milliseconds
// (dubbo's canonical timeout unit); Policy.MaxRetries as an integer. Both are
// applied only when > 0: a 0 means "no override", so a center that does not
// configure them leaves the dubbo-native value untouched.
//
// Note: dubbo retries is cluster-failover-level (retries across providers), so
// Policy.MaxRetries here maps to dubbo cluster retries, not resilience-layer
// retry - the center documents dubbo's timeout/retry in dubbo's own semantics.
func applyGovernOverride(params map[string]string, p resilience.ClientPolicy) {
	if p.AttemptTimeout > 0 {
		params["timeout"] = strconv.FormatInt(int64(p.AttemptTimeout/time.Millisecond), 10)
	}
	if p.MaxRetries > 0 {
		params["retries"] = strconv.Itoa(p.MaxRetries)
	}
}

// retriesStr returns the retries value as a string, or empty when unset.
// Unified semantics at every level: -1 (the default) = unset (nothing
// published, dubbo-go keeps its own default), 0 = no retry attempts
// (published as "0"), >0 = that many retries.
func retriesStr(r int) string {
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
