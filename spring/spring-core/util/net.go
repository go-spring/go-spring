/*
 * Copyright 2012-2019 the original author or authors.
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

package util

import (
	"net"
	"sync"
)

var localIPv4Str = "0.0.0.0"
var localIPv4Once = new(sync.Once)

// LocalIPv4 获取本机的 IPv4 地址
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
