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

// Package StarterDubbo integrates dubbo-go with go-spring. The registration
// below is the whole of the starter's wiring: the shared *Instance, the
// consumer *client.Client, the provider server, the dynamic-configuration
// poller and the two dubbo filters (fault, loadtest).
package StarterDubbo

import (
	"dubbo.apache.org/dubbo-go/v3/common/config"
	"dubbo.apache.org/dubbo-go/v3/common/extension"
	"go-spring.org/spring/gs"
	"go-spring.org/starter-dubbo/internal/logger"
	mapconfig "go-spring.org/starter-dubbo/internal/mapconfig"
	"go-spring.org/stdlib/flatten"
)

// Install the dubbo-go integration points: the log bridge (dubbo-go's own logs
// into go-spring's pipeline) and the map-backed configuration center. Both are
// plain package functions rather than subpackage inits, so everything this
// starter registers is readable here.
func init() {
	logger.Install()
	mapconfig.Install()
}

// The shared *Instance: one bean holding the whole ${spring.dubbo} node, from
// which the consumer and provider beans derive their configuration.
func init() {
	// Activate mapconfig as dubbo-go's DynamicConfiguration so the dyncPoller
	// can push override rules into the configurator pipeline at runtime.
	config.GetEnvInstance().SetDynamicConfiguration(mapconfig.Singleton())

	gs.Provide(
		NewInstance,
		gs.IndexArg(0, gs.TagArg("${spring.dubbo}")),
	).Condition(gs.OnProperty("spring.dubbo.registries"))
}

// The consumer *client.Client, built from the shared *Instance.
func init() {
	gs.Provide(
		NewClient,
	).Condition(gs.OnBean[*Instance]())
}

// The dynamic-configuration poller (see dync.go).
func init() {
	// The governance center is the family's sole injection point: the poller
	// reads its resilience authority and its ready signal from here.
	gs.Provide(newDyncPoller,
		gs.IndexArg(0, gs.TagArg("${spring.dubbo.application}")),
	).Init((*dyncPoller).Init).Export(gs.As[gs.Rooter]()).Caller(1)
}

// The provider server, exported as a gs.Server.
func init() {
	enableSimpleDubboServer := gs.OnProperty("spring.dubbo.provider.enabled").
		HavingValue("true").MatchIfMissing()
	gs.Module(enableSimpleDubboServer, func(r gs.BeanProvider, p flatten.Storage) error {
		r.Provide(
			NewSimpleDubboServer,
		).Export(gs.As[gs.Server]()).Condition(
			gs.OnBean[ServiceRegister](),
			gs.OnBean[*Instance](),
		)
		return nil
	})
}

// The inbound fault-injection filter (see fault.go).
func init() {
	// Register the inbound fault-injection filter under "fault". Add "fault" to
	// a provider's filter chain to activate it. dubbo's filter registry hands out
	// filters via a no-arg constructor, so the injector cannot arrive as a
	// constructor parameter: a bean installs it into the package handle below
	// instead.
	extension.SetFilter(faultFilterKey, newFaultFilter)

	// The governance center is the family's sole injection point: the hook
	// installs its fault authority into the package handle the filter reads.
	gs.Provide(newInjectorHook).Export(gs.As[gs.Rooter]()).Caller(1)
}

// The load-test identification filter (see loadtest.go).
func init() {
	// Register the load-test identification filter under the dubbo filter name
	// "loadtest". Activate it by adding "loadtest" to a provider's filter chain
	// (ideally first), so the marker is on the context before later filters and
	// the service impl run — letting downstream code branch on the load-test
	// convention.
	extension.SetFilter(loadTestFilterKey, newLoadTestFilter)

	// The install-and-hold bean. dubbo's filter registry hands out filters via a
	// no-arg constructor, so the propagator cannot arrive as a constructor
	// parameter: a bean installs it into the package handle below instead.
	// Exported as a gs.Rooter so gs instantiates it even though nothing injects
	// it — without a collected-type export an unreachable bean is never created
	// and the filter would keep reading go-spring's default.
	gs.Provide(newPropagatorHook, gs.IndexArg(0, gs.TagArg("?"))).
		Export(gs.As[gs.Rooter]()).Caller(1)
}
