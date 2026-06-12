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

package example

import (
	"context"
	"fmt"
	"io"

	exp "github.com/go-spring/gs-mock/example/inner"
)

//go:generate gs mock -o src_mock.go -i '!RepositoryV2,,GenericService,Service,,Repository'

var _ = fmt.Println

type Response struct {
	Value int
}

type GenericService[R any, S any] interface {
	io.Writer
	Init()
	Default() S
	TryDefault() (S, bool)
	Accept(R)
	Convert(R) S
	TryConvert(R) (S, bool)
	Process(context.Context, map[string]R) (S, error)
	Printf(format string, args ...any)
}

type Service interface {
	io.Writer
	Init()
	Default() *Response
	TryDefault() (*Response, bool)
	Accept(*exp.Request)
	Convert(*exp.Request) *Response
	TryConvert(*exp.Request) (*Response, bool)
	Process(context.Context, map[string]*exp.Request) (*Response, error)
	Printf(format string, args ...any)
}
