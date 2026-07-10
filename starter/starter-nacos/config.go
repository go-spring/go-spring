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

package StarterNacos

// Config defines Nacos client connection configuration.
type Config struct {
	// IpAddr is the Nacos server IP address, e.g., "127.0.0.1".
	IpAddr string `value:"${ip-addr}"`

	// Port is the Nacos server port, default is 8848.
	Port uint64 `value:"${port:=8848}"`

	// Namespace is the Nacos namespace ID, default is empty (public).
	Namespace string `value:"${namespace:=}"`

	// Username is the Nacos auth username, default is empty.
	Username string `value:"${username:=}"`

	// Password is the Nacos auth password, default is empty.
	Password string `value:"${password:=}"`

	// TimeoutMs is the request timeout in milliseconds, default is 5000.
	TimeoutMs uint64 `value:"${timeout-ms:=5000}"`

	// LogLevel is the SDK log level, e.g., "info".
	LogLevel string `value:"${log-level:=info}"`

	// LogDir is the directory for SDK logs.
	LogDir string `value:"${log-dir:=/tmp/nacos/log}"`

	// CacheDir is the directory for SDK cache files.
	CacheDir string `value:"${cache-dir:=/tmp/nacos/cache}"`
}
