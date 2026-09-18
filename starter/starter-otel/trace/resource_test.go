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

package trace

import (
	"testing"

	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
)

// attrString reads value of k out of a resource, yielding "" when the key is
// absent so a dropped attribute fails an equality assertion instead of
// panicking.
func attrString(res *resource.Resource, k string) string {
	set := res.Set()
	v, _ := set.Value(attribute.Key(k))
	return v.AsString()
}

// TestNewResourceKeepsConfiguredServiceName pins the trap the merge order
// exists to avoid: OTel's default resource always carries a service.name of its
// own -- "unknown_service:<binary>" -- so a merge that let the default win
// would silently replace the configured name on every process that does not set
// OTEL_SERVICE_NAME.
func TestNewResourceKeepsConfiguredServiceName(t *testing.T) {
	t.Setenv("OTEL_SERVICE_NAME", "")
	res, err := NewResource("configured-name")
	assert.Error(t, err).Nil()
	assert.String(t, attrString(res, "service.name")).Equal("configured-name")
}

// TestNewResourceEnvServiceNameWins proves OTEL_SERVICE_NAME overrides the
// configured name: the environment variable is the operator-level override, the
// same precedence OTel itself gives it.
func TestNewResourceEnvServiceNameWins(t *testing.T) {
	t.Setenv("OTEL_SERVICE_NAME", "from-env")
	res, err := NewResource("configured-name")
	assert.Error(t, err).Nil()
	assert.String(t, attrString(res, "service.name")).Equal("from-env")
}

// TestNewResourceCarriesOTelDefault proves the OTel default resource is really
// merged in rather than the service name alone: telemetry.sdk.* comes from that
// resource and go-spring sets none of it. This is what gives every span and
// metric the process-level dimensions -- and OTEL_RESOURCE_ATTRIBUTES, the
// standard way an operator attaches env/cluster/tenant -- without go-spring
// exposing an API of its own.
func TestNewResourceCarriesOTelDefault(t *testing.T) {
	res, err := NewResource("configured-name")
	assert.Error(t, err).Nil()
	assert.String(t, attrString(res, "telemetry.sdk.language")).Equal("go")
	assert.String(t, attrString(res, "telemetry.sdk.name")).Equal("opentelemetry")
}
