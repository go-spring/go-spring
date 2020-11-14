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

package SpringLogger_test

import (
	"errors"
	"fmt"

	"github.com/go-spring/spring-logger"
)

func Example_stdLogger() {
	SpringLogger.SetLogger(SpringLogger.NewConsole(SpringLogger.InfoLevel))
	SpringLogger.SetLevel(SpringLogger.TraceLevel)

	SpringLogger.Trace("a", "=", "1")
	SpringLogger.Tracef("a=%d", 1)

	SpringLogger.Debug("a", "=", "1")
	SpringLogger.Debugf("a=%d", 1)

	SpringLogger.Info("a", "=", "1")
	SpringLogger.Infof("a=%d", 1)

	SpringLogger.Warn("a", "=", "1")
	SpringLogger.Warnf("a=%d", 1)

	SpringLogger.Error("a", "=", "1")
	SpringLogger.Errorf("a=%d", 1)

	func() {
		defer func() { fmt.Println(recover()) }()
		SpringLogger.Panic("error")
	}()

	func() {
		defer func() { fmt.Println(recover()) }()
		SpringLogger.Panic(errors.New("error"))
	}()

	func() {
		defer func() { fmt.Println(recover()) }()
		SpringLogger.Panicf("error: %d", 404)
	}()

	// SpringLogger.Fatal("a", "=", "1")
	// SpringLogger.Fatalf("a=%d", 1)

	SpringLogger.Output(0, SpringLogger.InfoLevel, "a=1")
	SpringLogger.Outputf(0, SpringLogger.InfoLevel, "a=%d", 1)
}
