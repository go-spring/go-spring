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

package fault

import (
	"time"

	kfault "go-spring.org/cloud/governance/capability/fault"
	"go-spring.org/cloud/governance/capability/matcher"
	serverfault "go-spring.org/cloud/governance/dataplane/inbound/fault"
)

// trule is the FLAT test spelling of a direction rule. The production types
// embed the shared core, so a flat composite literal (all the knobs in one
// place) is far easier to scan in a test; these helpers convert it.
type trule struct {
	Name, Service string
	Rate          float64
	Class, Error  string
	Status        int
	Latency       time.Duration
	LatencyJitter time.Duration
	MaxDuration   time.Duration
	MaxAffected   int64
	MetadataMatch map[string]matcher.StringMatch
	ActionMatch   matcher.ActionMatch
}

func (r trule) core() kfault.RuleCore {
	return kfault.RuleCore{
		Name:          r.Name,
		Service:       r.Service,
		MetadataMatch: r.MetadataMatch,
		ActionMatch:   r.ActionMatch,
		Rate:          r.Rate,
		Latency:       r.Latency,
		LatencyJitter: r.LatencyJitter,
		MaxDuration:   r.MaxDuration,
		MaxAffected:   r.MaxAffected,
	}
}

// clientRules converts flat specs to the production outbound rules.
func clientRules(rs []trule) []ClientRule {
	out := make([]ClientRule, len(rs))
	for i, r := range rs {
		out[i] = ClientRule{RuleCore: r.core(), Error: r.Error, Class: r.Class, Status: r.Status}
	}
	return out
}

// serverRules converts flat specs to the production inbound rules.
func serverRules(rs []trule) []serverfault.ServerRule {
	out := make([]serverfault.ServerRule, len(rs))
	for i, r := range rs {
		out[i] = serverfault.ServerRule{RuleCore: r.core(), Status: r.Status}
	}
	return out
}
