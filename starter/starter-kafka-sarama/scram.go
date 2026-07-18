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

package StarterKafkaSarama

import (
	"github.com/IBM/sarama"
	"github.com/xdg-go/scram"
)

// xdgSCRAMClient adapts github.com/xdg-go/scram to sarama.SCRAMClient so the
// sarama client can perform SCRAM-SHA-256 / SCRAM-SHA-512 authentication.
// sarama does not ship a built-in SCRAM client; the adapter is the standard
// wiring shown in the sarama examples.
type xdgSCRAMClient struct {
	*scram.Client
	*scram.ClientConversation
	scram.HashGeneratorFcn
}

// Begin initializes the SCRAM conversation for the given credentials.
func (x *xdgSCRAMClient) Begin(userName, password, authzID string) (err error) {
	x.Client, err = x.HashGeneratorFcn.NewClient(userName, password, authzID)
	if err != nil {
		return err
	}
	x.ClientConversation = x.Client.NewConversation()
	return nil
}

// Step feeds a server challenge into the conversation and returns the reply.
func (x *xdgSCRAMClient) Step(challenge string) (string, error) {
	return x.ClientConversation.Step(challenge)
}

// Done reports whether the SCRAM handshake has completed.
func (x *xdgSCRAMClient) Done() bool {
	return x.ClientConversation.Done()
}

// scramSHA256Generator produces a fresh SCRAM-SHA-256 client per handshake, as
// required by sarama.Net.SASL.SCRAMClientGeneratorFunc.
func scramSHA256Generator() sarama.SCRAMClient {
	return &xdgSCRAMClient{HashGeneratorFcn: scram.SHA256}
}

// scramSHA512Generator produces a fresh SCRAM-SHA-512 client per handshake.
func scramSHA512Generator() sarama.SCRAMClient {
	return &xdgSCRAMClient{HashGeneratorFcn: scram.SHA512}
}
