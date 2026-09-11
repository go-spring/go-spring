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

// User-contributed server interceptors are injected, not registered: an
// application gs.Provides each interceptor and exports it As
// grpc.UnaryServerInterceptor / grpc.StreamServerInterceptor, and
// NewSimpleGrpcServer receives them all as bean collections. There is no
// package-level registration API — each container carries its own stack, so
// two containers in one process never share interceptors.
package StarterGrpc
