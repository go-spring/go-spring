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
	"time"

	"dubbo.apache.org/dubbo-go/v3/registry"
)

// RegistryCfg configures a single service registry. The map key is a free-form
// logical ID (becomes registry.WithID), letting multiple registries of the same
// type coexist and be selected via RegistryIDs. The type comes from Protocol, or
// the map key when Protocol is empty. Empty/zero fields are skipped.
type RegistryCfg struct {
	Protocol   string            `value:"${protocol:=}"` // registry type (etcdv3|nacos|zookeeper|...); defaults to the map key
	Address    string            `value:"${address:=}"`
	Namespace  string            `value:"${namespace:=}"`
	Group      string            `value:"${group:=}"`
	Username   string            `value:"${username:=}"`
	Password   string            `value:"${password:=}"`
	Timeout    time.Duration     `value:"${timeout:=}"`  // e.g. "5s"
	TTL        time.Duration     `value:"${ttl:=}"`      // e.g. "15m"
	Weight     int64             `value:"${weight:=-1}"` // negative means unset; 0 is a valid weight
	Zone       string            `value:"${zone:=}"`
	Simplified bool              `value:"${simplified:=false}"`
	Preferred  bool              `value:"${preferred:=false}"` // try this registry first
	Params     map[string]string `value:"${params:=}"`
}

// options translates a RegistryCfg into dubbo-go registry.Options. Shared by
// both server and client, since dubbo-go takes the same registry.Option on both
// sides. Empty/zero fields are skipped so dubbo-go keeps its own defaults.
func (rc RegistryCfg) options(id string) []registry.Option {
	regType := rc.Protocol
	if regType == "" {
		regType = id
	}
	opts := []registry.Option{
		registry.WithID(id),
		registry.WithRegistry(regType),
	}
	if rc.Address != "" {
		opts = append(opts, registry.WithAddress(rc.Address))
	}
	if rc.Namespace != "" {
		opts = append(opts, registry.WithNamespace(rc.Namespace))
	}
	if rc.Group != "" {
		opts = append(opts, registry.WithGroup(rc.Group))
	}
	if rc.Username != "" {
		opts = append(opts, registry.WithUsername(rc.Username))
	}
	if rc.Password != "" {
		opts = append(opts, registry.WithPassword(rc.Password))
	}
	if rc.Timeout > 0 {
		opts = append(opts, registry.WithTimeout(rc.Timeout))
	}
	if rc.TTL > 0 {
		opts = append(opts, registry.WithTTL(rc.TTL))
	}
	if rc.Weight >= 0 {
		opts = append(opts, registry.WithWeight(rc.Weight))
	}
	if rc.Zone != "" {
		opts = append(opts, registry.WithZone(rc.Zone))
	}
	if rc.Simplified {
		opts = append(opts, registry.WithSimplified())
	}
	if rc.Preferred {
		opts = append(opts, registry.WithPreferred())
	}
	if len(rc.Params) > 0 {
		opts = append(opts, registry.WithParams(rc.Params))
	}
	return opts
}

// resolveRegistries applies the role-first, global-fallback rule: a non-empty
// role map replaces the global one wholesale; keys are not merged.
func resolveRegistries(global, role map[string]RegistryCfg) map[string]RegistryCfg {
	if len(role) > 0 {
		return role
	}
	return global
}
