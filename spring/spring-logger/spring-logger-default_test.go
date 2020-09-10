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
	"testing"

	"github.com/go-spring/spring-logger"
)

func TestConsole(t *testing.T) {

	c := SpringLogger.NewConsole(SpringLogger.InfoLevel)
	c.SetLevel(SpringLogger.TraceLevel)

	c.Trace("a", "=", "1")
	c.Tracef("a=%d", 1)

	c.Debug("a", "=", "1")
	c.Debugf("a=%d", 1)

	c.Info("a", "=", "1")
	c.Infof("a=%d", 1)

	c.Warn("a", "=", "1")
	c.Warnf("a=%d", 1)

	c.Error("a", "=", "1")
	c.Errorf("a=%d", 1)

	t.Run("panic#00", func(t *testing.T) {
		defer func() { fmt.Println(recover()) }()
		c.Panic("error")
	})

	t.Run("panic#01", func(t *testing.T) {
		defer func() { fmt.Println(recover()) }()
		c.Panic(errors.New("error"))
	})

	t.Run("panic#02", func(t *testing.T) {
		defer func() { fmt.Println(recover()) }()
		c.Panicf("error: %d", 404)
	})

	// c.Fatal("a", "=", "1")
	// c.Fatalf("a=%d", 1)

	c.Print("a", "=", "1")
	c.Printf("a=%d\n", 1)

	c.Output(0, SpringLogger.InfoLevel, "a=1")
	c.Outputf(0, SpringLogger.InfoLevel, "a=%d", 1)
}

func TestStdLogger(t *testing.T) {

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

	t.Run("panic#00", func(t *testing.T) {
		defer func() { fmt.Println(recover()) }()
		SpringLogger.Panic("error")
	})

	t.Run("panic#01", func(t *testing.T) {
		defer func() { fmt.Println(recover()) }()
		SpringLogger.Panic(errors.New("error"))
	})

	t.Run("panic#02", func(t *testing.T) {
		defer func() { fmt.Println(recover()) }()
		SpringLogger.Panicf("error: %d", 404)
	})

	// SpringLogger.Fatal("a", "=", "1")
	// SpringLogger.Fatalf("a=%d", 1)

	SpringLogger.Output(0, SpringLogger.InfoLevel, "a=1")
	SpringLogger.Outputf(0, SpringLogger.InfoLevel, "a=%d", 1)
}

func TestStdLoggerWrapper(t *testing.T) {

	c := SpringLogger.StdLoggerWrapper{
		StdLogger: SpringLogger.NewConsole(SpringLogger.InfoLevel),
	}

	c.SetLevel(SpringLogger.TraceLevel)

	c.Trace("a", "=", "1")
	c.Tracef("a=%d", 1)

	c.Debug("a", "=", "1")
	c.Debugf("a=%d", 1)

	c.Info("a", "=", "1")
	c.Infof("a=%d", 1)

	c.Warn("a", "=", "1")
	c.Warnf("a=%d", 1)

	c.Error("a", "=", "1")
	c.Errorf("a=%d", 1)

	t.Run("panic#00", func(t *testing.T) {
		defer func() { fmt.Println(recover()) }()
		c.Panic("error")
	})

	t.Run("panic#01", func(t *testing.T) {
		defer func() { fmt.Println(recover()) }()
		c.Panic(errors.New("error"))
	})

	t.Run("panic#02", func(t *testing.T) {
		defer func() { fmt.Println(recover()) }()
		c.Panicf("error: %d", 404)
	})

	// c.Fatal("a", "=", "1")
	// c.Fatalf("a=%d", 1)

	c.Print("a", "=", "1")
	c.Printf("a=%d\n", 1)

	c.Output(0, SpringLogger.InfoLevel, "a=1")
	c.Outputf(0, SpringLogger.InfoLevel, "a=%d", 1)
}
