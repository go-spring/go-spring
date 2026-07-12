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

// Config holds the goframe raw-TCP server settings.
//
// gtcp has no notion of gsvc: unlike ghttp.Server / grpcx.GrpcServer, it does
// not snapshot a Registry at construction time and it does not publish itself
// on Start. Registration into etcd is therefore done by hand in
// internal/server (see the Register/Deregister calls there), which is why
// this Config carries an explicit AdvertiseHost — with gtcp there is no
// framework-side "detect my outbound IP" step to fall back on.
//
// Values come from Go-Spring properties (see conf/app.properties) under the
// "${goframe.tcp}" prefix using `value` tags, replacing goframe's own
// manifest/config/config.yaml loader.
type Config struct {
	// Address is the gtcp.Server bind address, e.g. ":8003".
	Address string `value:"${address:=:8003}"`

	// AdvertiseHost is the host the provider publishes into etcd. Because
	// gtcp binds on Address and never asks the OS for a public IP, the
	// consumer would fail to dial "0.0.0.0" or "" — the provider has to
	// name the address it wants clients to connect on. In real deployments
	// this is the pod/host IP; in this example it defaults to 127.0.0.1.
	AdvertiseHost string `value:"${advertise.host:=127.0.0.1}"`

	// AdvertisePort is the port half of the endpoint published into etcd.
	// Kept separate from Address so ":8003" (bind everywhere) can coexist
	// with a single advertised port; parsing Address at runtime would work
	// but leaves less room to override in tests/deploys.
	AdvertisePort int `value:"${advertise.port:=8003}"`

	// Name is the service name the provider registers under; the consumer
	// resolves this same name from etcd via gsvc.Search.
	Name string `value:"${name:=goframe.tcp.echo}"`

	// RegistryAddr is the etcd address; matches docker-compose.yml.
	RegistryAddr string `value:"${registry.etcd:=127.0.0.1:2379}"`
}
