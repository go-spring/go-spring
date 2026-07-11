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

package data

import (
	"os"

	"github.com/go-kratos/kratos/v2/log"
	"go-spring.org/spring/gs"
)

func init() {
	// The scaffold wired these with google/wire ProviderSet. Here each becomes
	// a Go-Spring bean instead: the kratos log.Logger is shared across every
	// layer, Data is the (empty) resource holder, and the GreeterRepo is
	// exported as the biz.GreeterRepo interface the usecase depends on.
	gs.Provide(NewLogger)
	gs.Provide(NewData)
	gs.Provide(NewGreeterRepo)
}

// NewLogger provides the kratos logger used by the data/biz/server layers. The
// scaffold built this inline in main(); as a bean it can be injected anywhere.
func NewLogger() log.Logger {
	return log.NewStdLogger(os.Stdout)
}

// Data holds the data-layer resources (database/redis clients, etc.).
type Data struct {
	// TODO wrapped database client
}

// NewData builds the data resources. The scaffold returned a cleanup func and
// took a *conf.Data; since this example holds no real client, both are dropped
// and Go-Spring owns the lifecycle.
func NewData(logger log.Logger) *Data {
	log.NewHelper(logger).Info("data resources ready")
	return &Data{}
}
