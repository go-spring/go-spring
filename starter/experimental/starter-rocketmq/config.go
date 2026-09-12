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

package StarterRocketmq

import (
	"time"
)

// Config defines RocketMQ client configuration.
//
// Fields are intentionally flat, mirroring rocketmq-client-go's option model:
// the underlying library has no client-wide options struct — name servers,
// credentials and instance name are repeated on every producer/consumer — so
// this starter holds them once on the Client wrapper and reapplies them to
// every producer and consumer it creates.
type Config struct {
	// NameServers is the RocketMQ NameServer address list,
	// e.g., "127.0.0.1:9876". At least one address is required.
	NameServers []string `value:"${name-servers}" expr:"len($) > 0"`

	// InstanceName distinguishes multiple client instances running in the same
	// process or on the same host. Leave empty for the SDK default ("DEFAULT").
	// Set it (e.g., to the PID) when several producers/consumers must not share
	// one underlying remoting client.
	InstanceName string `value:"${instance-name:=}"`

	// AccessKey is the ACL access key. Both AccessKey and SecretKey must be set
	// together to enable credentials; leave both empty to disable ACL.
	AccessKey string `value:"${access-key:=}"`

	// SecretKey is the ACL secret key that pairs with AccessKey.
	SecretKey string `value:"${secret-key:=}"`

	// SendTimeout is the producer send timeout, e.g., "3s".
	SendTimeout time.Duration `value:"${send-timeout:=3s}"`

	// Retry is the number of retries performed internally by the producer
	// before a synchronous send fails. 2 means up to 3 attempts in total. Like
	// the Elasticsearch client's, this retry sits inside the client and so
	// multiplies with govern's `max-retries` for the same resource when both are
	// non-zero — pick one place to retry.
	Retry int `value:"${retry:=2}"`

	// FailFast enables a startup connectivity probe. When true, a TCP dial is
	// issued against the first NameServer address right after the client is
	// built, so a wrong address list fails at boot instead of on first use.
	// RocketMQ's remoting layer connects lazily and would otherwise swallow
	// the error until the first produce/consume.
	FailFast bool `value:"${fail-fast:=true}"`
}
