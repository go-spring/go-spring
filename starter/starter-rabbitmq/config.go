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

package StarterRabbitMQ

import (
	"time"

	"go-spring.org/stdlib/starter"
)

// Config defines RabbitMQ connection configuration.
type Config struct {
	// URL is the AMQP connection URL,
	// e.g., "amqp://guest:guest@127.0.0.1:5672/".
	// Use the "amqps://" scheme (or set TLS.Enabled) to negotiate TLS.
	URL string `value:"${url}"`

	// Vhost overrides the virtual host parsed from the URL, default is empty.
	Vhost string `value:"${vhost:=}"`

	// Heartbeat is the interval for connection heartbeats,
	// 0 uses the URL/server default, e.g., "10s".
	Heartbeat time.Duration `value:"${heartbeat:=10s}"`

	// TLS configures transport encryption. It is activated either explicitly by
	// TLS.Enabled=true or implicitly when the URL uses the "amqps://" scheme;
	// the certificate files are optional and only needed for a custom CA or
	// mTLS. Uses the shared stdlib/starter TLS block so property keys are
	// uniform across starters (cert-file / key-file / ca-file / server-name /
	// insecure-skip-verify).
	TLS starter.TLSConfig `value:"${tls}"`
}
