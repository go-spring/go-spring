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
	"testing"

	kfault "go-spring.org/cloud/governance/capability/fault"
	"go-spring.org/spring/conf"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/testing/assert"
)

// TestServerConfig_StatusBoundary pins the two layers: the field's expr tag
// judges the SHAPE (0 or 400–599), and [ServerConfig.Validate] judges the
// cross-field rule (a firing rule must say what to answer).
func TestServerConfig_StatusBoundary(t *testing.T) {
	bind := func(props map[string]any) error {
		var c ServerConfig
		return conf.Bind(flatten.NewPropertiesStorage(flatten.MapProperties(props)), &c)
	}

	// A firing rule must declare a status.
	assert.Error(t, bind(map[string]any{
		"rules[0].name": "fire", "rules[0].service": "gin", "rules[0].rate": "1",
	})).NotNil()

	// A latency-only rule may omit it.
	assert.Error(t, bind(map[string]any{
		"rules[0].name": "slow", "rules[0].service": "gin", "rules[0].rate": "0",
		"rules[0].latency": "10ms",
	})).Nil()

	// A declared status is an ERROR answer: 400–599 only.
	for _, status := range []string{"400", "429", "503", "599"} {
		assert.Error(t, bind(map[string]any{
			"rules[0].name": "fire", "rules[0].service": "gin", "rules[0].rate": "1",
			"rules[0].status": status,
		})).Nil()
	}

	// A content type requires a body.
	assert.Error(t, bind(map[string]any{
		"rules[0].name": "fire", "rules[0].service": "gin", "rules[0].rate": "1",
		"rules[0].status": "503", "rules[0].content-type": "application/json",
	})).NotNil()
	assert.Error(t, bind(map[string]any{
		"rules[0].name": "fire", "rules[0].service": "gin", "rules[0].rate": "1",
		"rules[0].status": "503", "rules[0].content-type": "application/json",
		"rules[0].body": `{"error":"injected"}`,
	})).Nil()
	for _, status := range []string{"200", "302", "399", "600"} {
		assert.Error(t, bind(map[string]any{
			"rules[0].name": "fire", "rules[0].service": "gin", "rules[0].rate": "1",
			"rules[0].status": status,
		})).NotNil()
	}
}

// TestServerRule_RollSetsStatus pins the effect: the roll records the declared
// status and returns an error only to short-circuit the handler.
func TestServerRule_RollSetsStatus(t *testing.T) {
	r := ServerRule{
		RuleCore:    kfault.RuleCore{Name: "fire", Service: "gin", Rate: 1},
		Status:      429,
		Body:        `{"error":"injected"}`,
		ContentType: "application/json",
	}
	res, err := r.Roll(kfault.Dose{Inject: true})
	assert.That(t, kfault.IsInjected(err)).True()
	assert.Number(t, res.Status).Equal(429)
	assert.String(t, res.Body).Equal(`{"error":"injected"}`)
	assert.String(t, res.ContentType).Equal("application/json")
	assert.String(t, res.Class).Equal("") // inbound has no class
	assert.String(t, res.Name).Equal("fire")
}
