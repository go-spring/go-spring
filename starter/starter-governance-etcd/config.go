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

package StarterGovernanceEtcd

import (
	"strings"
	"time"
)

// governanceEtcdConfig binds ${source.etcd.*}.
type governanceEtcdConfig struct {
	// Endpoints lists the etcd cluster nodes to dial. Required.
	Endpoints []string `value:"${endpoints}" expr:"len($) > 0"`

	// Key is the etcd key holding the rules document.
	Key string `value:"${key}" expr:"$ != ''"`

	// Username and Password authenticate against etcd; empty means no auth.
	Username string `value:"${username:=}"`
	Password string `value:"${password:=}"`

	// DialTimeout bounds the client dial (default 5s).
	DialTimeout time.Duration `value:"${dial-timeout:=5s}"`

	// Format overrides document-format detection; by default it is inferred
	// from the key's extension, else properties.
	Format string `value:"${format:=}"`
}

// sourceFormat resolves the rule document format: an explicit format wins, else
// the name's extension, else properties.
func sourceFormat(explicit, name string) string {
	if explicit != "" {
		return explicit
	}
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		if ext := name[i+1:]; ext != "" {
			return ext
		}
	}
	return "properties"
}
