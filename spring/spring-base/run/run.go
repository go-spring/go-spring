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

package run

import (
	"errors"
	"os"
	"strings"
)

var mode int

const (
	recordMode = 0x0001
	replayMode = 0x0002
	testMode   = 0x0004
)

func init() {
	if os.Getenv("GS_RECORD_MODE") != "" {
		mode |= recordMode
	}
	if os.Getenv("GS_REPLAY_MODE") != "" {
		mode |= replayMode
	}
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.") {
			mode |= testMode
			break
		}
	}
}

// NormalMode returns whether it is in normal mode.
func NormalMode() bool {
	return mode == 0
}

// RecordMode returns whether it is in record mode.
func RecordMode() bool {
	return mode&0x00ff == recordMode
}

// ReplayMode returns whether it is in replay mode.
func ReplayMode() bool {
	return mode&0x00ff == replayMode
}

// TestMode returns whether it is in test mode.
func TestMode() bool {
	return mode&0x00ff == testMode
}

// MustTestMode panic occurs when calling in non-test mode.
func MustTestMode() {
	if !TestMode() {
		panic(errors.New("should be called in test mode"))
	}
}
