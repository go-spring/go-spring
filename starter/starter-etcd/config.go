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

package StarterEtcd

import (
	"strings"
	"time"
)

// Config defines etcd client connection configuration.
type Config struct {
	// Endpoints is a comma-separated list of etcd server addresses,
	// e.g., "127.0.0.1:2379" or "127.0.0.1:2379,127.0.0.1:2380".
	Endpoints string `value:"${endpoints}"`

	// Username is the etcd auth username, default is empty.
	Username string `value:"${username:=}"`

	// Password is the etcd auth password, default is empty.
	Password string `value:"${password:=}"`

	// DialTimeout is the timeout for failing to establish a connection, e.g., "5s".
	DialTimeout time.Duration `value:"${dial-timeout:=5s}"`

	// AutoSyncInterval is the interval to update endpoints with its latest members,
	// 0 disables auto-sync, e.g., "0".
	AutoSyncInterval time.Duration `value:"${auto-sync-interval:=0}"`

	// DialKeepAliveTime is the time after which client pings the server
	// to see if the transport is alive, e.g., "0".
	DialKeepAliveTime time.Duration `value:"${dial-keep-alive-time:=0}"`

	// DialKeepAliveTimeout is the time that the client waits for a response
	// for the keep-alive probe, e.g., "0".
	DialKeepAliveTimeout time.Duration `value:"${dial-keep-alive-timeout:=0}"`
}

// EndpointList splits the comma-separated Endpoints into a slice,
// trimming spaces and dropping empty items.
func (c Config) EndpointList() []string {
	var out []string
	for _, s := range strings.Split(c.Endpoints, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
