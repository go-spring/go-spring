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
	"time"

	kfault "go-spring.org/cloud/governance/capability/fault"
	"go-spring.org/spring/conf"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/testing/assert"
)

// TestClientRule_Binds pins that a client rule's shared core and its own effect
// bind from the flattened properties a platform writes, even though the core is
// embedded: the binder recurses into the anonymous field. Match and Action are
// fault's own types, not chain's, so what matters is the spelling:
// match.<key>.<op> per metadata dimension, action.exact/action.prefix for the
// interface.
func TestClientRule_Binds(t *testing.T) {
	storage := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
		"rules[0].name":               "gin-users",
		"rules[0].service":            "gin",
		"rules[0].action.exact":       "GET /users/{id}",
		"rules[0].match.tenant.exact": "acme",
		"rules[0].match.region.exact": "cn",
		"rules[0].rate":               "1",
		"rules[0].error":              "timeout",
		"rules[0].class":              "permanent",
		"rules[0].status":             "503",
		"rules[0].start-after":        "1s",
		"rules[0].on":                 "5s",
		"rules[0].off":                "5s",
		"rules[1].name":               "redis",
		"rules[1].service":            "redis:cache",
		"rules[1].action.prefix":      "GET /users",
	}))
	var c ClientConfig
	assert.Error(t, conf.Bind(storage, &c)).Nil()

	assert.Number(t, len(c.Rules)).Equal(2)
	r0 := c.Rules[0]
	assert.String(t, r0.Service).Equal("gin")
	assert.String(t, r0.ActionMatch.Exact).Equal("GET /users/{id}")
	assert.String(t, r0.ActionMatch.Prefix).Equal("")
	assert.String(t, r0.MetadataMatch["tenant"].Exact).Equal("acme")
	assert.String(t, r0.MetadataMatch["region"].Exact).Equal("cn")
	assert.String(t, r0.Error).Equal(kfault.ErrorTimeout)
	assert.String(t, r0.Class).Equal(kfault.ClassPermanent)
	assert.Number(t, r0.Status).Equal(503)
	assert.That(t, r0.StartAfter).Equal(time.Second)
	assert.That(t, r0.On).Equal(5 * time.Second)
	assert.That(t, r0.Off).Equal(5 * time.Second)
	assert.Number(t, int(c.Rules[1].Rate)).Equal(0)
	assert.String(t, c.Rules[1].Error).Equal(kfault.ErrorGeneric)   // tag default
	assert.String(t, c.Rules[1].Class).Equal(kfault.ClassTransient) // tag default

	// The bound rule governs exactly the call it names: same service, same
	// interface, matching metadata — and nothing else.
	assert.That(t, r0.Matches(kfault.Target{Service: "gin", Action: "GET /users/{id}",
		Metadata: map[string]string{"tenant": "acme", "region": "cn"}})).True()
	assert.That(t, r0.Matches(kfault.Target{Service: "gin", Action: "GET /users/{id}",
		Metadata: map[string]string{"tenant": "acme"}})).False() // region missing
	assert.That(t, r0.Matches(kfault.Target{Service: "gin", Action: "POST /orders",
		Metadata: map[string]string{"tenant": "acme", "region": "cn"}})).False()
}

// TestClientRule_ExprRejectsBadValues pins the single-field boundary on the path
// that owns it: the binder evaluates each field's expr tag, so a bad dose, an
// unknown vocabulary word or an empty name never becomes a rule. `business` is
// rejected here because the outbound direction cannot produce that answer.
func TestClientRule_ExprRejectsBadValues(t *testing.T) {
	bad := []map[string]any{
		{"rules[0].name": "", "rules[0].rate": "0.5"},
		{"rules[0].name": "r", "rules[0].rate": "1.5"},
		{"rules[0].name": "r", "rules[0].rate": "-0.1"},
		{"rules[0].name": "r", "rules[0].latency": "-1ms"},
		{"rules[0].name": "r", "rules[0].latency-jitter": "-1ms"},
		{"rules[0].name": "r", "rules[0].max-duration": "-1s"},
		{"rules[0].name": "r", "rules[0].max-affected": "-1"},
		{"rules[0].name": "r", "rules[0].error": "boom"},
		{"rules[0].name": "r", "rules[0].error": ""},
		{"rules[0].name": "r", "rules[0].class": "sometimes"},
		{"rules[0].name": "r", "rules[0].class": "business"},
		{"rules[0].name": "r", "rules[0].class": ""},
		{"rules[0].name": "r", "rules[0].rate": "0.5", "rules[0].every": "2"}, // mutually exclusive
		{"rules[0].name": "r", "rules[0].on": "1s"},                           // on without off
		{"rules[0].name": "r", "rules[0].off": "1s"},                          // off without on
		{"rules[0].name": "r", "rules[0].status": "200"},                      // not an error status
		{"rules[0].name": "r", "rules[0].status": "600"},
	}
	for _, props := range bad {
		var c ClientConfig
		assert.Error(t, conf.Bind(flatten.NewPropertiesStorage(flatten.MapProperties(props)), &c)).NotNil()
	}
}
