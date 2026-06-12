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

package gs_test

import (
	"fmt"
	"testing"

	"github.com/go-spring/spring-core/gs"
)

func init() {
	gs.Provide(func(ctx *gs.ContextProvider) *GlobalService {
		return &GlobalService{}
	})
}

type GlobalService struct {
	Name string `value:"${name:=global}"`
}

type App1Service struct {
	Name string         `value:"${name:=app1}"`
	Svr  *GlobalService `autowire:""`
}

func TestApp1(t *testing.T) {
	gs.RunTest(t, func(s *App1Service) {
		fmt.Println(s.Name, s.Svr.Name)
	})
}

func TestApp2(t *testing.T) {
	gs.Configure(func(g gs.App) {
		g.Property("name", "myapp2")
	}).RunTest(t, func(s *struct {
		Name string         `value:"${name:=app2}"`
		Svr  *GlobalService `autowire:""`
		App1 *App1Service   `autowire:"?"`
	}) {
		fmt.Println(s.Name, s.Svr.Name, s.App1)
	})
}
