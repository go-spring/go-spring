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
	"os"
	"testing"

	"github.com/go-spring/spring-base/assert"
)

func TestSetMode(t *testing.T) {
	assert.True(t, TestMode())
	reset := SetMode(NormalModeFlag)
	assert.True(t, NormalMode())
	assert.False(t, RecordMode())
	assert.False(t, ReplayMode())
	assert.False(t, TestMode())
	reset()
	assert.False(t, NormalMode())
	assert.False(t, RecordMode())
	assert.False(t, ReplayMode())
	assert.True(t, TestMode())
}

func TestNormalMode(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = nil
	initMode()
	assert.True(t, NormalMode())
	assert.False(t, RecordMode())
	assert.False(t, ReplayMode())
	assert.False(t, TestMode())
}

func TestRecordMode(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = nil
	os.Setenv("GS_RECORD_MODE", "true")
	defer func() { os.Unsetenv("GS_RECORD_MODE") }()
	initMode()
	assert.False(t, NormalMode())
	assert.True(t, RecordMode())
	assert.False(t, ReplayMode())
	assert.False(t, TestMode())
}

func TestReplayMode(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = nil
	os.Setenv("GS_REPLAY_MODE", "local")
	defer func() { os.Unsetenv("GS_REPLAY_MODE") }()
	initMode()
	assert.False(t, NormalMode())
	assert.False(t, RecordMode())
	assert.True(t, ReplayMode())
	assert.False(t, TestMode())
}

func TestMustTestMode(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = nil
	assert.Panic(t, func() {
		MustTestMode()
	}, "should be called in test mode")
}
