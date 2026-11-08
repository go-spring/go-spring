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

// Package StarterGovernanceSentinel contributes sentinel-golang to the outbound
// resilience framework as a RATE-LIMIT ALGORITHM.
//
// The contribution is a [ratelimit.Provider] bean: a policy selects it with
// `limiter: sentinel`, and the resilience engine keeps ownership of timeout,
// retry, breaker, rule matching, per-rule keys and metrics. Sentinel answers
// the flow decision only — there is no second executor and no duplicated
// pipeline.
//
// Importing this package initializes sentinel once, which makes the provider
// usable.
package StarterGovernanceSentinel

import (
	"context"

	sentinel "github.com/alibaba/sentinel-golang/api"

	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
)

func init() {
	// Initialise sentinel once at import time. A misconfigured environment fails
	// loudly here rather than on first use.
	if err := sentinel.InitDefault(); err != nil {
		panic("starter-governance-sentinel: sentinel init failed: " + err.Error())
	}
	// Contribute the flow algorithm to the process limiter factory. A rule that
	// writes `limiter: sentinel` now routes through sentinel; an unknown name
	// still fails document validation.
	gs.Provide(func() ratelimit.Provider { return Provider{} }).
		Name(LimiterAlgorithm).Caller(1)
	log.Info(context.Background(), log.TagAppDef, "sentinel rate-limit provider available")
}
