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

package config

// Config holds the goframe HTTP server settings.
//
// In a stock goframe project these come from manifest/config/config.yaml and
// are loaded implicitly by g.Server() via g.Cfg(). Here they are bound from
// Go-Spring properties (see conf/app.properties) under the "${goframe}" prefix
// using `value` tags, so the config source moves out of goframe's own loader.
//
// Name and RegistryAddr are what turn the earlier single-process example into a
// real provider/consumer split: the provider registers itself into etcd under
// Name, and the consumer resolves that same name from etcd instead of dialing
// a hard-coded host:port.
type Config struct {
	Address      string `value:"${address:=:8000}"`
	Name         string `value:"${name:=goframe.hello}"`
	RegistryAddr string `value:"${registry.etcd:=127.0.0.1:2379}"`
}
