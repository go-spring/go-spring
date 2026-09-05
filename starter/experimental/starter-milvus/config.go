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

package StarterMilvus

import ()

// Config defines one Milvus connection.
type Config struct {
	// Addr is the Milvus gRPC endpoint, "host:19530". Required.
	Addr string `value:"${addr}" expr:"$ != ''"`

	// Database is the default database for this client (default "default").
	Database string `value:"${database:=default}"`

	// Username / Password authenticate when the Milvus cluster has auth on.
	Username string `value:"${username:=}"`
	Password string `value:"${password:=}"`
}
