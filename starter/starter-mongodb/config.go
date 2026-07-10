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

package StarterMongoDB

import "time"

// Config defines MongoDB client connection configuration.
type Config struct {
	// URI is the MongoDB connection string,
	// e.g., "mongodb://127.0.0.1:27017".
	URI string `value:"${uri}"`

	// ConnectTimeout is the timeout for establishing the initial connection,
	// 0 uses the driver default, e.g., "10s".
	ConnectTimeout time.Duration `value:"${connect-timeout:=10s}"`

	// MaxPoolSize is the maximum number of connections in the pool,
	// 0 uses the driver default, e.g., "100".
	MaxPoolSize uint64 `value:"${max-pool-size:=100}"`

	// MinPoolSize is the minimum number of connections in the pool, default is 0.
	MinPoolSize uint64 `value:"${min-pool-size:=0}"`
}
