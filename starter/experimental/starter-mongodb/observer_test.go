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

package StarterMongoDB

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// recordingObserver captures the mongodb layer's events so the seam is tested
// without OTel.
type recordingObserver struct {
	operations []string
	active     int64
}

func (o *recordingObserver) Operation(_ context.Context, operation, status string, _ time.Duration) {
	o.operations = append(o.operations, operation+":"+status)
}

func (o *recordingObserver) Active(_ context.Context, _ string, delta int64) { o.active += delta }

func TestObserverSeam(t *testing.T) {
	obs := &recordingObserver{}
	d := newDBObserverWith(obs)
	_, span := d.Start(context.Background(), "find", "{}")

	boom := errors.New("boom")
	span.End(boom)

	if got := fmt.Sprint(obs.operations); got != "[find:error]" {
		t.Fatalf("operations = %s, want [find:error]", got)
	}
	if obs.active != 0 {
		t.Fatalf("active = %d, want 0 after a balanced operation", obs.active)
	}
}
