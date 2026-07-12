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

// Package greet defines the REST GreetService contract.
//
// Unlike the Triple sibling in ../../triple, REST in dubbo-go has no
// protobuf IDL and no code generator; unlike the classic-Dubbo and JSON-RPC
// siblings, however, REST is not "just Go reflection + wire codec": it also
// needs an HTTP-method-and-URL routing table because the wire is plain HTTP
// and every Go method must be pinned to a (verb, path, param-source) tuple.
//
// The "IDL" therefore lives in two places:
//   - this Go file, which pins the Java-style interface name and method
//     names (used for etcd registration and RPCInvocation dispatch), and
//   - the rest_config.RestServiceConfig maps registered on both the
//     provider and consumer sides (see provider/rest_service.go and
//     consumer/main.go) which pin every method's HTTP verb, path, and
//     parameter source (path / query / header / body).
//
// Method parameters here are primitives (string), so JSON body decoding
// is not needed; we route the single string argument as a query parameter
// (`GET /greet?name=...`), which is the least ambiguous shape for a scalar
// arg in the dubbo-go REST server.
package greet

// GreetServiceInterface is the Java-style dotted interface name under which
// the provider registers into etcd; the consumer dials by the same name.
const GreetServiceInterface = "com.example.GreetService"

// MethodGreet is the RPC method name looked up on the provider via
// reflection over its exported methods, and also the key into the
// RestMethodConfigsMap on both provider and consumer.
const MethodGreet = "Greet"

// BeanName is the id under which the RestServiceConfig maps are keyed on
// both the provider and consumer. dubbo-go derives it from
// common.GetReference(handler) — the Go struct name of the handler — and
// stamps it onto the URL as `bean.name` before publishing into etcd. Both
// sides must agree, so we pin it here to remove the "hidden coupling to
// the handler struct name" footgun.
const BeanName = "GreetProvider"

// GreetPath is the URL path served by the provider and dialled by the
// consumer's rest client.
const GreetPath = "/greet"

// GreetHTTPMethod is the HTTP verb bound to GreetPath.
const GreetHTTPMethod = "GET"

// GreetQueryName is the query-string parameter name that carries the
// single `name string` argument of GreetService.Greet.
const GreetQueryName = "name"
