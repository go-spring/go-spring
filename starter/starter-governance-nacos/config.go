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

package StarterGovernanceNacos

import (
	"strings"
	"time"
)

// governanceNacosConfig binds ${spring.governance.source.nacos.*}.
type governanceNacosConfig struct {
	// Server is the Nacos server address, host:port.
	Server string `value:"${server}" expr:"$ != ''"`

	// DataID is the dataId holding the rules document.
	DataID string `value:"${data-id}" expr:"$ != ''"`

	// Group and Namespace locate the dataId; Group defaults to DEFAULT_GROUP.
	Group     string `value:"${group:=DEFAULT_GROUP}"`
	Namespace string `value:"${namespace:=}"`

	// Username and Password authenticate against Nacos; empty means no auth.
	Username string `value:"${username:=}"`
	Password string `value:"${password:=}"`

	// DialTimeout bounds the source's own client calls (default 5s).
	DialTimeout time.Duration `value:"${dial-timeout:=5s}"`

	// Format overrides document-format detection; by default it is inferred
	// from the dataId's extension, else properties.
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
