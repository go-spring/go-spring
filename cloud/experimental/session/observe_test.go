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

package session

import (
	"context"
	"errors"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

// recordingObserver captures the session layer's events so the seam is tested
// without OTel.
type recordingObserver struct {
	events []string
}

func (o *recordingObserver) Operation(_ context.Context, operation, status string) {
	o.events = append(o.events, operation+":"+status)
}

func TestObserverSeam(t *testing.T) {
	obs := &recordingObserver{}
	m, err := NewManager(NewMemory(), Options{}, obs)
	assert.That(t, err).Nil()

	m.countStoreOp("load", nil, false)
	m.countStoreOp("save", errors.New("boom"), false)
	assert.Slice(t, obs.events).Equal([]string{"load:missed", "save:error"})
}

func TestNilObserverDisablesEvents(t *testing.T) {
	m, err := NewManager(NewMemory(), Options{}, nil)
	assert.That(t, err).Nil()
	m.countStoreOp("load", nil, true) // must not panic
}
