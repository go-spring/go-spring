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

// Package run provides methods to query the running mode of program.
package run

import (
	"errors"
	"os"
	"strings"
)

var mode int

const (
	NormalModeFlag = 0x0000
	RecordModeFlag = 0x0001
	ReplayModeFlag = 0x0002
	TestModeFlag   = 0x0004
)

func init() {
	initMode()
}

// initMode for unit test.
func initMode() {
	mode = NormalModeFlag
	if os.Getenv("GS_RECORD_MODE") != "" {
		mode |= RecordModeFlag
	}
	if os.Getenv("GS_REPLAY_MODE") != "" {
		mode |= ReplayModeFlag
	}
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.") {
			mode |= TestModeFlag
			break
		}
	}
}

// SetMode sets the running mode, only in unit test mode.
func SetMode(flag int) (reset func()) {
	MustTestMode()
	old := mode
	reset = func() {
		mode = old
	}
	mode = flag
	return
}

// NormalMode returns whether it is running in normal mode.
func NormalMode() bool {
	return mode == 0
}

// RecordMode returns whether it is running in record mode.
func RecordMode() bool {
	return mode&RecordModeFlag == RecordModeFlag
}

// ReplayMode returns whether it is running in replay mode.
func ReplayMode() bool {
	return mode&ReplayModeFlag == ReplayModeFlag
}

// TestMode returns whether it is running in test mode.
func TestMode() bool {
	return mode&TestModeFlag == TestModeFlag
}

// MustTestMode panic occurs when calling not in test mode.
func MustTestMode() {
	if !TestMode() {
		panic(errors.New("should be called in test mode"))
	}
}
