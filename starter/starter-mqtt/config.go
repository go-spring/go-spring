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

package StarterMQTT

import (
	"time"

	"go-spring.org/spring/cloud/tlsconf"
)

// Config defines MQTT client connection configuration.
type Config struct {
	// Broker is the MQTT broker address,
	// e.g., "tcp://127.0.0.1:1883".
	Broker string `value:"${broker}"`

	// ClientID is the client identifier presented to the broker,
	// default is empty (the client library generates one).
	ClientID string `value:"${client-id:=}"`

	// Username is the auth username, default is empty.
	Username string `value:"${username:=}"`

	// Password is the auth password, default is empty.
	Password string `value:"${password:=}"`

	// CleanSession controls whether the broker discards session state
	// on disconnect, default is true.
	CleanSession bool `value:"${clean-session:=true}"`

	// KeepAlive is the interval between PING packets, default is "30s".
	KeepAlive time.Duration `value:"${keep-alive:=30s}"`

	// ConnectTimeout bounds how long Connect waits for the broker,
	// 0 disables the timeout, default is "10s".
	ConnectTimeout time.Duration `value:"${connect-timeout:=10s}"`

	// TLS configures transport security for MQTTS. Use a "ssl://" or "tls://"
	// broker URL together with TLS.Enabled.
	TLS tlsconf.TLSConfig `value:"${tls}"`

	// Will configures the Last Will and Testament (LWT) message the broker
	// publishes on the client's behalf if it disconnects ungracefully.
	Will WillConfig `value:"${will}"`
}

// WillConfig configures the Last Will and Testament message. The will is
// registered only when Topic is non-empty.
type WillConfig struct {
	// Topic is the topic the broker publishes the will message to. Empty
	// disables the will, default is empty.
	Topic string `value:"${topic:=}"`

	// Payload is the will message body, default is empty.
	Payload string `value:"${payload:=}"`

	// QoS is the delivery quality of service for the will (0, 1, or 2),
	// default is 0.
	QoS byte `value:"${qos:=0}"`

	// Retained controls whether the broker retains the will message,
	// default is false.
	Retained bool `value:"${retained:=false}"`
}
