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

package server

import (
	"go-spring.org/spring/gs"
)

func init() {
	// The scaffold exposed these via wire.NewSet and let kratos.App own their
	// lifecycle. Here each server is exported as a gs.Server so gs.Run() drives
	// startup and graceful shutdown. Config is bound from the
	// ${spring.kratos.http} / ${spring.kratos.grpc} prefixes.
	gs.Provide(NewHTTPServer, gs.IndexArg(0, gs.TagArg("${spring.kratos.http}"))).
		Export(gs.As[gs.Server]())
	gs.Provide(NewGRPCServer, gs.IndexArg(0, gs.TagArg("${spring.kratos.grpc}"))).
		Export(gs.As[gs.Server]())
}
