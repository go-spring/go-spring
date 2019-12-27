/*
 * Copyright 2012-2019 the original author or authors.
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

package BootStarter_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/go-spring/go-spring/boot-starter"
)

type MyApp struct {
}

func (app *MyApp) Start() {
	fmt.Println("app start")
}

func (app *MyApp) ShutDown() {
	fmt.Println("app shutdown")
}

func TestBootStarter(t *testing.T) {

	go func() {
		defer fmt.Println("go stop")
		fmt.Println("go start")

		time.Sleep(200 * time.Millisecond)
		BootStarter.Exit()
	}()

	BootStarter.Run(new(MyApp))
}

type Condition struct {
}

func NewCondition() *Condition {
	return &Condition{}
}

func (c *Condition) On() *Condition {
	return c
}

type Constriction Condition

func NewConstriction() *Constriction {
	return &Constriction{}
}

func TestRetype(t *testing.T) {

	t.Run("Condition", func(t *testing.T) {
		c := NewCondition()
		typ := reflect.TypeOf(c)
		fmt.Println(typ, typ.NumMethod())
	})

	t.Run("Constriction", func(t *testing.T) {
		c := NewConstriction()
		typ := reflect.TypeOf(c)
		fmt.Println(typ, typ.NumMethod())
	})
}
