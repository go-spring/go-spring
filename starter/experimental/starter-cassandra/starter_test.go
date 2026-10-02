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

package StarterCassandra

import (
	"testing"
	"time"

	"go-spring.org/spring/conf"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/testing/assert"
)

// bindInstances binds the instances bucket the way starter.go does, with the
// family "default" bucket as the fallback.
func bindInstances(data map[string]string) (map[string]Config, error) {
	p := flatten.WithFallback(flatten.NewPropertiesStorage(flatten.NewProperties(data)),
		"spring.cassandra.instances", "spring.cassandra.default")
	var m map[string]Config
	err := conf.Bind(p, &m, "${spring.cassandra.instances}")
	return m, err
}

// Every field an instance leaves unset is inherited from the family-wide
// default bucket — this is what the two-bucket layout exists for.
func TestInstancesInheritDefaults(t *testing.T) {
	m, err := bindInstances(map[string]string{
		"spring.cassandra.default.timeout":         "5s",
		"spring.cassandra.default.consistency":     "one",
		"spring.cassandra.default.health":          "false",
		"spring.cassandra.instances.main.keyspace": "ks",
		"spring.cassandra.instances.main.hosts[0]": "10.0.0.1",
	})
	assert.Error(t, err).Nil()

	c := m["main"]
	assert.That(t, c.Keyspace).Equal("ks")           // instance's own
	assert.That(t, c.Timeout).Equal(5 * time.Second) // inherited leaf
	assert.That(t, c.Consistency).Equal("one")       // inherited leaf
	assert.That(t, c.Health).False()                 // inherited bool
	assert.That(t, c.Hosts).Equal([]string{"10.0.0.1"})
}

// An instance's own value wins over the default, key by key.
func TestInstanceOverridesDefault(t *testing.T) {
	m, err := bindInstances(map[string]string{
		"spring.cassandra.default.timeout":         "5s",
		"spring.cassandra.default.consistency":     "one",
		"spring.cassandra.instances.main.timeout":  "9s",
		"spring.cassandra.instances.main.hosts[0]": "10.0.0.1",
	})
	assert.Error(t, err).Nil()

	c := m["main"]
	assert.That(t, c.Timeout).Equal(9 * time.Second) // instance wins
	assert.That(t, c.Consistency).Equal("one")       // still inherited
}
