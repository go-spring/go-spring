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

package StarterScheduler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go-spring.org/cloud/scheduling"
	"go-spring.org/stdlib/testing/assert"
)

func noop(context.Context) error { return nil }

// A job that cannot fire must not be registrable: every one of these is a mistake
// that would otherwise become a job which silently never runs, so each panics at
// registration — that is, during startup.
func TestNewJobPanicsOnAnUnusableRegistration(t *testing.T) {
	cases := []struct {
		what    string
		panicIn func()
		matches string
	}{
		{"empty name", func() { NewJob("", noop, Every(time.Second)) },
			"job name must not be empty"},
		{"nil function", func() { NewJob("j", nil, Every(time.Second)) },
			"job function must not be nil"},
		{"no trigger", func() { NewJob("j", noop) },
			"has no trigger"},
		{"two triggers", func() { NewJob("j", noop, Every(time.Second), Cron("* * * * *")) },
			"sets more than one trigger"},
		{"non-positive rate", func() { NewJob("j", noop, Every(0)) },
			"FixedRate requires a positive duration"},
		{"non-positive delay", func() { NewJob("j", noop, After(0)) },
			"FixedDelay requires a positive duration"},
		{"unparsable cron", func() { NewJob("j", noop, Cron("not a cron")) },
			"must have 5 fields, got 3"},
		{"six-field cron", func() { NewJob("j", noop, Cron("0 */5 * * * *")) },
			"must have 5 fields, got 6"},
	}
	for _, c := range cases {
		assert.Panic(t, c.panicIn, c.matches, c.what)
	}
}

// The schedule travels on the bean, so build() can read it back without a config
// lookup — this is what replaced the jobs.<name>.* block.
func TestNewJobCarriesItsSchedule(t *testing.T) {
	j := NewJob("cleanup", noop,
		After(time.Minute),
		WithTimeout(5*time.Second),
		WithConcurrency(scheduling.Replace),
		WithLock("redis"), WithLockKey("k"), WithLockTTL(30*time.Second),
	)

	assert.String(t, j.JobName()).Equal("cleanup")
	// An After trigger is a fixed-delay one: its next fire is measured from the
	// previous run's completion.
	done := time.Now()
	next := j.Trigger().Next(scheduling.TriggerContext{
		Now:            done,
		LastScheduled:  done,
		LastCompletion: done,
	})
	assert.That(t, next.Equal(done.Add(time.Minute))).
		True(fmt.Sprintf("After(d) must schedule d past the previous completion, got %v", next))

	spec := j.Spec()
	assert.Number(t, spec.Timeout).Equal(5 * time.Second)
	assert.Number(t, spec.Concurrency).Equal(scheduling.Replace)
	assert.String(t, spec.Lock).Equal("redis")
	assert.String(t, spec.LockKey).Equal("k")
	assert.Number(t, spec.LockTTL).Equal(30 * time.Second)
}

// Every(d) is a fixed-rate trigger: fire times stay on the grid anchored at the
// first fire, so a late run does not push later fires back.
func TestEveryAnchorsOnThePlannedFireTime(t *testing.T) {
	j := NewJob("tick", noop, Every(time.Second))
	first := time.Now().Add(time.Second)

	second := j.Trigger().Next(scheduling.TriggerContext{
		Now:           first.Add(200 * time.Millisecond), // the run overran a little
		LastScheduled: first,
	})
	assert.That(t, second.Equal(first.Add(time.Second))).
		True(fmt.Sprintf("the next fire must stay on the original grid, got %v", second))
}
