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

package netutil

import (
	"net"
	"sync"
)

var localIPv4Str = "0.0.0.0"
var localIPv4Once = new(sync.Once)

// LocalIPv4 returns the first non-loopback IPv4 address of the local machine.
//
// The result is cached after the first call using sync.Once, so subsequent
// calls are fast. If no non-loopback IPv4 address is found, "0.0.0.0" is returned.
//
// Note:
//   - Only IPv4 addresses are considered; IPv6 addresses are ignored.
//   - The result does not update if the network interfaces change after the first call.
//   - For more robust use, consider returning (string, error) to handle the case
//     where no valid IPv4 address is available.
func LocalIPv4() string {
	localIPv4Once.Do(func() {
		if ias, err := net.InterfaceAddrs(); err == nil {
			for _, address := range ias {
				if ipNet, ok := address.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
					if ipNet.IP.To4() != nil {
						localIPv4Str = ipNet.IP.String()
						return
					}
				}
			}
		}
	})
	return localIPv4Str
}
