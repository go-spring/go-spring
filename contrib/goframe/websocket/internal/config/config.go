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

// Config holds the goframe WebSocket server settings.
//
// WebSocket in goframe is not a distinct server type: the *ghttp.Server owns
// the listener, and any HTTP route can upgrade the connection to WebSocket
// via ghttp.Request.WebSocket() (which wraps gorilla/websocket underneath).
// That is why the fields here mirror the sibling `../http` module almost
// verbatim — the transport differs, but the config surface (bind address,
// service name, etcd address) is the same because gsvc registration hangs
// off the HTTP server. See internal/server for the actual upgrade wiring.
//
// Values come from Go-Spring properties (see conf/app.properties) under the
// "${goframe.websocket}" prefix using `value` tags, instead of goframe's own
// manifest/config/config.yaml loader.
type Config struct {
	Address      string `value:"${address:=:8002}"`
	Name         string `value:"${name:=goframe.websocket.echo}"`
	RegistryAddr string `value:"${registry.etcd:=127.0.0.1:2379}"`
}
