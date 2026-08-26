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

package loadtest

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Driver owns the scheduling strategy: WHEN ops are fired and whether their
// dispatch waits for the previous op to complete. The [Runner] hands Drive an
// invoke function that already times and records each op, so a Driver only
// decides timing and concurrency.
//
// Contract: Drive must invoke `invoke` for each op it schedules and MUST NOT
// return until every invoke it has started has completed, so the runner can
// build a complete [Result] (no lost tail latency). Drive returns nil; the run
// is ended by ctx cancellation (the runner applies the duration as a timeout).
//
// Built-in implementations: [ClosedLoop] (closed-loop), [OpenLoop], [Ramp],
// [Staircase] (open-loop variants), and the fully custom [Scheduled]. Supply
// your own to model any traffic shape (bursty, diurnal, replay-from-trace).
type Driver interface {
	Drive(ctx context.Context, invoke func(context.Context) error) error
}

// driverNamer identifies a Driver for [Result.Driver]. Implemented by the
// built-ins; a custom Driver that wants a friendly label implements it too,
// otherwise its "%T" type name is used.
type driverNamer interface {
	loadTestDriverName() string
}

func driverName(d Driver) string {
	if n, ok := d.(driverNamer); ok {
		return n.loadTestDriverName()
	}
	return fmt.Sprintf("%T", d)
}

// ClosedLoop fires op from N workers, each looping as fast as op returns. This
// is the "N concurrent users" model: throughput self-limits as latency rises,
// so it answers "how many concurrent requests can the system serve". It is what
// the legacy [Run] uses. Concurrency < 1 is treated as 1.
type ClosedLoop struct {
	Concurrency int
}

func (ClosedLoop) loadTestDriverName() string { return "closed-loop" }

func (d ClosedLoop) Drive(ctx context.Context, invoke func(context.Context) error) error {
	n := max(d.Concurrency, 1)
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if ctx.Err() != nil {
					return
				}
				_ = invoke(ctx)
			}
		}()
	}
	wg.Wait()
	return nil
}

// Schedule returns the target arrival rate (ops/sec) at a given elapsed time
// since the run started. It drives the open-loop family: a constant schedule is
// [OpenLoop], a linearly varying one is [Ramp], a step function is [Staircase].
// Returning <= 0 pauses dispatch until the rate turns positive again.
type Schedule func(elapsed time.Duration) float64

// ConstantSchedule returns a Schedule always yielding rps.
func ConstantSchedule(rps float64) Schedule {
	return func(time.Duration) float64 { return rps }
}

// LinearRamp returns a Schedule ramping RPS linearly from `from` to `to` over
// `over`; it holds at `to` afterwards. over <= 0 means start at `to` immediately.
func LinearRamp(from, to float64, over time.Duration) Schedule {
	return func(elapsed time.Duration) float64 {
		if over <= 0 || elapsed >= over {
			return to
		}
		return from + (to-from)*(float64(elapsed)/float64(over))
	}
}

// Step is one leg of a [Staircase] schedule: hold RPS for Duration.
type Step struct {
	RPS      float64
	Duration time.Duration
}

// StepSchedule returns a staircase Schedule: each Step's RPS holds from its
// cumulative start time until the next Step begins.
func StepSchedule(steps ...Step) Schedule {
	starts := make([]time.Duration, len(steps))
	rates := make([]float64, len(steps))
	var cum time.Duration
	for i, s := range steps {
		starts[i] = cum
		rates[i] = s.RPS
		cum += s.Duration
	}
	return func(elapsed time.Duration) float64 {
		for i := len(starts) - 1; i >= 0; i-- {
			if elapsed >= starts[i] {
				return rates[i]
			}
		}
		return 0
	}
}

// scheduledDriver paces dispatch at the arrival rate schedule(elapsed). It is
// open-loop: the next op is scheduled from the clock, independent of the
// previous op completing, so sustained arrival that exceeds capacity queues
// behind MaxConcurrent (a bounded semaphore). This exposes latency-tail
// behavior that closed-loop self-throttling hides. MaxConcurrent <= 0 means
// unbounded — use with care, a slow op under high RPS spawns goroutines without
// limit.
type scheduledDriver struct {
	schedule      Schedule
	MaxConcurrent int
}

func (scheduledDriver) loadTestDriverName() string { return "scheduled" }

func (d scheduledDriver) Drive(ctx context.Context, invoke func(context.Context) error) error {
	var sem chan struct{}
	if d.MaxConcurrent > 0 {
		sem = make(chan struct{}, d.MaxConcurrent)
	}
	var wg sync.WaitGroup
	start := time.Now()

	// Deadline-driven dispatch (not a fixed ticker) so a varying Schedule can
	// speed up and slow down: each iteration recomputes the target interval
	// from the current rate and sleeps to the next dispatch instant.
	for {
		if ctx.Err() != nil {
			break
		}
		rps := d.schedule(time.Since(start))
		if rps <= 0 {
			// Nothing to dispatch right now; re-evaluate shortly.
			time.Sleep(repollInterval)
			continue
		}
		interval := time.Duration(float64(time.Second) / rps)
		interval = max(interval, time.Nanosecond)
		// A plain Sleep (not a timer select) keeps this example-grade driver
		// simple; cancellation is noticed on the next loop iteration.
		time.Sleep(interval)
		if ctx.Err() != nil {
			break
		}
		if sem != nil {
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				wg.Wait()
				return nil
			}
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if sem != nil {
				defer func() { <-sem }()
			}
			_ = invoke(ctx)
		}()
	}
	wg.Wait()
	return nil
}

// repollInterval is how long the scheduled driver idles when the current rate
// is non-positive (before recomputing). Long enough to not busy-spin, short
// enough to react to a rate that turns positive.
const repollInterval = 10 * time.Millisecond

// OpenLoop drives op at a fixed target RPS, open-loop. MaxConcurrent bounds
// in-flight goroutines; 0 means unbounded (use with care).
func OpenLoop(rps float64, maxConcurrent int) Driver {
	return scheduledDriver{schedule: ConstantSchedule(rps), MaxConcurrent: maxConcurrent}
}

// Ramp drives op with RPS ramping linearly from `from` to `to` over `over` (then
// holding at `to`). MaxConcurrent bounds in-flight goroutines.
func Ramp(from, to float64, over time.Duration, maxConcurrent int) Driver {
	return scheduledDriver{schedule: LinearRamp(from, to, over), MaxConcurrent: maxConcurrent}
}

// Staircase drives op with RPS stepping through the given steps in order.
// MaxConcurrent bounds in-flight goroutines.
func Staircase(maxConcurrent int, steps ...Step) Driver {
	return scheduledDriver{schedule: StepSchedule(steps...), MaxConcurrent: maxConcurrent}
}

// Scheduled drives op at an arbitrary [Schedule] — the fully custom pacing seam
// for bursty / replay / diurnal traffic shapes. MaxConcurrent bounds in-flight.
func Scheduled(s Schedule, maxConcurrent int) Driver {
	return scheduledDriver{schedule: s, MaxConcurrent: maxConcurrent}
}
