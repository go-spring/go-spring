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

// Package greet defines the JSON-RPC GreetService contract.
//
// Like the classic-Dubbo sibling (see ../../dubbo), the JSON-RPC protocol
// in dubbo-go has no protobuf IDL and no code generator: the service is a
// plain Go struct whose exported method signatures are reflected over at
// registration time. What is different on the wire: JSON-RPC frames the
// request/response as a JSON body carried over a bare HTTP/1.1 connection,
// so the "IDL" is this Go file plus the JSON-serializable shape of the
// method's parameters and return.
//
// The provider registers the service under a Java-style interface name via
// server.WithInterface(GreetServiceInterface); the consumer dials the same
// interface name, and Dubbo resolves a live provider address from the
// registry (etcd). Method parameters here are primitives (string), which
// JSON handles natively — no extra type registration is required.
package greet

// GreetServiceInterface is the Java-style dotted interface name under which
// the provider registers into etcd; the consumer dials by the same name.
const GreetServiceInterface = "com.example.GreetService"

// MethodGreet is the RPC method name looked up on the provider via
// reflection over its exported methods.
const MethodGreet = "Greet"
