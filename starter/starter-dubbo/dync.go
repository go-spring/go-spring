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
	"context"
	"maps"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/cloud/governance/capability/source"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	mapconfig "go-spring.org/starter-dubbo/internal/mapconfig"
)

// consumerDocument is the consumer node's standalone source document: a
// dedicated file (or remote key) holding the ${spring.dubbo.consumer} node.
// The top-level tag declares the key space for source.Parse; DubboConsumer's
// own tags stay relative. A document carrying none of the keys binds the zero
// value — an empty file legitimately configures nothing.
type consumerDocument struct {
	Consumer DubboConsumer `value:"${spring.dubbo.consumer:=}"`
}

// dyncPoller is dubbo's local manager for dynamic configuration — the mapping
// of the cloud unified model (source pushes, the call path queries) onto a
// framework that owns its own invocation chain. It receives pushes from two
// sources — the consumer document and the governance limits document — and on
// each push re-derives the override-rule snapshot, publishing it into the
// in-memory config center as flat dubbo URL params. Dubbo's invoker chain
// "queries" per call by reading those URL params — dubbo-go's native mechanism,
// unchanged.
//
// Governance is consumed as its SOURCE, not through the resilience manager: the
// poller resolves the dubbo labels' limits from the pushed document itself, so
// the resilience package keeps its manager internals private. The cost is the
// few lines of label resolution in [labelLimits], coupled to the document
// schema (public) rather than to manager state.
//
// The two push entry points run on different goroutines, so poll may run
// concurrently with itself; mu plus the changed() diff make that a harmless
// double poll.
//
// Dynamic fields are those dubbo-go reads from URL params at call time:
// timeout, retries, loadbalance, cluster, group, version, serialization,
// sticky, force.tag, weight, and per-method tps/execute tuning.
// Filter is NOT dynamic — it is frozen into the invoker chain at Refer time;
// the Refer-frozen fields (filter, protocol, registries, check, url, async,
// generic, params) stay startup configuration, read by NewClient from the
// statically bound ${spring.dubbo.consumer} node.
type dyncPoller struct {
	dynCfg  *mapconfig.MapDynamicConfiguration // in-memory config center overrides are pushed into
	appName string                             // application name, used as the app-level override key

	// src is the consumer document source. Its constructor has read the first
	// copy before this poller exists, so Snapshot is always available.
	src source.Source[consumerDocument]

	// limSrc is the governance limits document source, or nil when no control
	// plane is linked — the poller then runs on dubbo's own document alone.
	limSrc source.Source[resilience.TimeoutRetrySettings]

	mu   sync.Mutex                   // guards last
	last map[string]map[string]string // last pushed override snapshot, for change detection
}

// newDyncPoller creates the poller bean and subscribes to both sources on the
// spot — every later push re-runs the poll, the same contract each
// control-plane center arms under.
func newDyncPoller(app DubboApplication, src source.Source[consumerDocument], limSrc source.Source[resilience.TimeoutRetrySettings]) *dyncPoller {
	p := &dyncPoller{
		dynCfg:  mapconfig.Singleton(),
		appName: app.Name,
		src:     src,
		limSrc:  limSrc,
		last:    make(map[string]map[string]string),
	}
	src.Subscribe(func(context.Context, consumerDocument) { p.poll() })
	if limSrc != nil {
		limSrc.Subscribe(func(context.Context, resilience.TimeoutRetrySettings) { p.poll() })
	}
	return p
}

// Init pushes the current override rules once. Later re-pushes come from the two
// source subscriptions; poll's internal diff skips no-op refreshes.
func (p *dyncPoller) Init() error {
	p.poll() // initial push
	return nil
}

func (p *dyncPoller) poll() {
	consumer := p.src.Snapshot().Consumer
	limits := p.limits()

	rules := p.consumerToOverrideRules(&consumer, limits)
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

// limits is the current governance document, or the zero value when no control
// plane is linked.
func (p *dyncPoller) limits() resilience.TimeoutRetrySettings {
	if p.limSrc == nil {
		return resilience.TimeoutRetrySettings{}
	}
	return p.limSrc.Snapshot()
}

// labelLimits is the label-level limits the document resolves to: the
// service-scope rule naming label, else the zero policy (no timeout, no
// retry). It is the one place dubbo duplicates a slice of the resilience
// matching rule — against the public document schema, not the manager.
func labelLimits(doc resilience.TimeoutRetrySettings, label string) resilience.TimeoutRetry {
	for i := range doc.Services {
		g := &doc.Services[i]
		if g.Service != label {
			continue
		}
		for j := range g.Rules {
			if g.Rules[j].ActionMatch.Exact == matcher.ServiceScopeAction {
				return g.Rules[j].TimeoutRetry
			}
		}
		break // first group naming the label wins
	}
	return resilience.TimeoutRetry{}
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
func (p *dyncPoller) consumerToOverrideRules(c *DubboConsumer, limits resilience.TimeoutRetrySettings) map[string]map[string]string {
	appName := p.appName
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
	// applyGovernOverride writes only set (>0) values, so a document that
	// configures nothing leaves dubbo's native values untouched.
	applyGovernOverride(appParams, labelLimits(limits, dubboAppLabel(appName)))
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
		applyGovernOverride(refParams, labelLimits(limits, dubboRefLabel(key)))
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
// They are the keys passed to Manager.PolicyFor and Manager.Subscribe so one
// governance rules document can tune dubbo's timeout/retry alongside every other
// client.
func dubboAppLabel(appName string) string  { return "dubbo:" + appName }
func dubboRefLabel(colonKey string) string { return "dubbo:" + colonKey }

// applyGovernOverride overrides timeout and retries in params from the
// governance policy. Policy.Timeout (a Duration) is written as milliseconds
// (dubbo's canonical timeout unit); Policy.Retry.MaxRetries as an integer. Both are
// applied only when > 0: a 0 means "no override", so a center that does not
// configure them leaves the dubbo-native value untouched.
//
// Note: dubbo retries is cluster-failover-level (retries across providers), so
// Policy.Retry.MaxRetries here maps to dubbo cluster retries, not resilience-layer
// retry - the center documents dubbo's timeout/retry in dubbo's own semantics.
func applyGovernOverride(params map[string]string, p resilience.TimeoutRetry) {
	if p.Timeout > 0 {
		params["timeout"] = strconv.FormatInt(int64(p.Timeout/time.Millisecond), 10)
	}
	if p.Retry.MaxRetries > 0 {
		params["retries"] = strconv.Itoa(p.Retry.MaxRetries)
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
