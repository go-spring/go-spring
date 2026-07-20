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

package transaction

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"go-spring.org/spring/resilience"
)

// Option configures the in-process [Coordinator] built by [NewCoordinator].
type Option func(*coordinator)

// WithStore sets the saga-log [Store]. Without it the coordinator keeps no
// durable log and a crash strands any in-flight saga; pass a durable Store in
// production.
func WithStore(s Store) Option { return func(c *coordinator) { c.store = s } }

// WithObserver sets the [Observer] used to instrument every step phase, e.g. a
// starter that opens otel spans.
func WithObserver(o Observer) Option { return func(c *coordinator) { c.observer = o } }

// NewCoordinator returns the bundled in-process [Coordinator]: it runs a saga's
// steps synchronously and, on the first failure, compensates the already-
// succeeded steps in reverse order. With no options it uses no store and no
// observer, which is a valid transparent setup for tests and development.
func NewCoordinator(opts ...Option) Coordinator {
	c := &coordinator{}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type coordinator struct {
	store    Store
	observer Observer
}

// completedStep records a step that succeeded, so compensation can replay it in
// reverse with the exact value its Action produced.
type completedStep struct {
	step   Step
	result any
}

func (c *coordinator) Execute(ctx context.Context, s Saga) (Result, error) {
	res := Result{Status: StatusCommitted, StepResults: make(map[string]any, len(s.Steps))}
	completed := make([]completedStep, 0, len(s.Steps))

	for _, step := range s.Steps {
		// Record the intent before running: the Action may cause a side effect a
		// crash would strand, so a recovering process must know this step was in
		// flight and compensate it.
		c.persistRunning(ctx, s, completed, step.Name)

		result, err := c.runPhase(ctx, s.ID, step, PhaseAction, func(ctx context.Context) (any, error) {
			return step.Action(ctx)
		})
		if err != nil {
			// The failing action's error leads Result.Errors so a compensated
			// saga still explains why it rolled back.
			res.Errors = append(res.Errors, StepError{Step: step.Name, Phase: PhaseAction, Err: err})
			if result != nil {
				res.StepResults[step.Name] = result
			}
			c.compensate(ctx, s.ID, completed, &res)
			c.finish(ctx, s, &res, completed)
			return res, err
		}
		res.StepResults[step.Name] = result
		completed = append(completed, completedStep{step: step, result: result})
		// Confirm completion: fold this step into the log and clear the in-flight
		// marker, still StatusRunning until the saga terminates.
		c.persistRunning(ctx, s, completed, "")
	}

	c.finish(ctx, s, &res, completed)
	return res, nil
}

// Recover resumes an interrupted saga by compensating everything it might have
// effected, in reverse order (backward recovery). It loads the saga log by
// s.ID; a missing log or an already-terminal one is a no-op, so recovery is
// idempotent. The in-flight step (if any) is compensated first with a nil
// result — its Action return value was never recorded — followed by the
// completed steps in reverse. s supplies the step definitions the log cannot.
func (c *coordinator) Recover(ctx context.Context, s Saga) (Result, error) {
	if c.store == nil {
		return Result{}, errors.New("transaction: Recover requires a Store")
	}

	snap, err := c.store.Load(ctx, s.ID)
	if err != nil {
		if errors.Is(err, ErrSnapshotNotFound) {
			// Nothing persisted (committed sagas delete their log): nothing to do.
			return Result{Status: StatusCommitted}, nil
		}
		return Result{}, err
	}
	if snap.Status != StatusRunning {
		// Already reached a terminal outcome; report it without acting again.
		return Result{Status: snap.Status}, nil
	}

	byName := make(map[string]Step, len(s.Steps))
	for _, step := range s.Steps {
		byName[step.Name] = step
	}

	// Build the set to compensate, most-recent first: the in-flight step (result
	// unknown, so nil) then the completed steps in reverse execution order.
	var toCompensate []completedStep
	if snap.InProgress != "" {
		if step, ok := byName[snap.InProgress]; ok {
			toCompensate = append(toCompensate, completedStep{step: step, result: nil})
		}
	}
	for _, name := range slices.Backward(snap.Completed) {
		if step, ok := byName[name]; ok {
			toCompensate = append(toCompensate, completedStep{step: step, result: snap.StepResults[name]})
		}
	}

	res := Result{Status: StatusRunning, StepResults: snap.StepResults}
	if res.StepResults == nil {
		res.StepResults = make(map[string]any)
	}
	c.compensateOrdered(ctx, s.ID, toCompensate, &res)

	// The completed set (in the log's forward order) is what finish persists as
	// the terminal snapshot.
	completed := make([]completedStep, 0, len(snap.Completed))
	for _, name := range snap.Completed {
		if step, ok := byName[name]; ok {
			completed = append(completed, completedStep{step: step, result: snap.StepResults[name]})
		}
	}
	c.finish(ctx, s, &res, completed)
	return res, nil
}

// compensate undoes the succeeded steps in reverse order, collecting every
// failure into res.Errors and downgrading the status when one occurs.
func (c *coordinator) compensate(ctx context.Context, sagaID string, completed []completedStep, res *Result) {
	ordered := make([]completedStep, 0, len(completed))
	for _, cs := range slices.Backward(completed) {
		ordered = append(ordered, cs)
	}
	c.compensateOrdered(ctx, sagaID, ordered, res)
}

// compensateOrdered compensates the steps in the order given (already
// most-recent-first), collecting every failure into res.Errors and downgrading
// the status. An irreversible step (nil Compensate) is a compensation failure.
func (c *coordinator) compensateOrdered(ctx context.Context, sagaID string, ordered []completedStep, res *Result) {
	res.Status = StatusCompensated
	for _, cs := range ordered {
		if cs.step.Compensate == nil {
			// An irreversible step reached during rollback leaves the system
			// inconsistent; surface it rather than silently skip.
			res.Status = StatusCompensationFailed
			res.Errors = append(res.Errors, StepError{
				Step:  cs.step.Name,
				Phase: PhaseCompensate,
				Err:   fmt.Errorf("transaction: step %q is irreversible (no Compensate)", cs.step.Name),
			})
			continue
		}
		_, err := c.runPhase(ctx, sagaID, cs.step, PhaseCompensate, func(ctx context.Context) (any, error) {
			return nil, cs.step.Compensate(ctx, cs.result)
		})
		if err != nil {
			res.Status = StatusCompensationFailed
			res.Errors = append(res.Errors, StepError{Step: cs.step.Name, Phase: PhaseCompensate, Err: err})
		}
	}
}

// runPhase runs one step phase under its retry policy and observer. The observer
// end func always fires with the phase's final error so failures are observable.
func (c *coordinator) runPhase(ctx context.Context, sagaID string, step Step, phase Phase,
	fn func(context.Context) (any, error)) (any, error) {

	end := func(error) {}
	if c.observer != nil {
		ctx, end = c.observer.Begin(ctx, sagaID, step.Name, phase)
	}

	var result any
	err := runWithPolicy(ctx, step.Retry, step.Name, func(ctx context.Context) error {
		var e error
		result, e = fn(ctx)
		return e
	})
	end(err)
	return result, err
}

// persistRunning writes the in-flight saga log while steps run, when a store is
// configured: Status is StatusRunning, completed holds the steps whose Actions
// succeeded and inProgress names the step currently running (empty between
// steps). A persistence error is intentionally swallowed here: the saga has
// already made progress and failing the whole operation on a log write would be
// worse than a gap in the log. Durable-store implementations should surface such
// problems through their own monitoring.
func (c *coordinator) persistRunning(ctx context.Context, s Saga, completed []completedStep, inProgress string) {
	if c.store == nil {
		return
	}
	_ = c.store.Save(ctx, s.ID, c.snapshot(s, StatusRunning, completed, inProgress))
}

// finish writes the terminal saga log. A committed saga's log is deleted (the
// work is done and needs no recovery); a compensated or failed saga's log is
// kept so operators and recovery can inspect it.
func (c *coordinator) finish(ctx context.Context, s Saga, res *Result, completed []completedStep) {
	if c.store == nil {
		return
	}
	if res.Status == StatusCommitted {
		_ = c.store.Delete(ctx, s.ID)
		return
	}
	_ = c.store.Save(ctx, s.ID, c.snapshot(s, res.Status, completed, ""))
}

func (c *coordinator) snapshot(s Saga, status Status, completed []completedStep, inProgress string) Snapshot {
	names := make([]string, len(completed))
	results := make(map[string]any, len(completed))
	for i, cs := range completed {
		names[i] = cs.step.Name
		results[cs.step.Name] = cs.result
	}
	return Snapshot{
		ID:          s.ID,
		Method:      s.Method,
		Status:      status,
		Completed:   names,
		InProgress:  inProgress,
		StepResults: results,
		UpdatedAt:   time.Now(),
	}
}

// runWithPolicy runs fn once when the policy is zero, or under the bundled
// resilience "default" executor (retry, per-attempt timeout, ...) otherwise. It
// reuses [resilience.Policy] so Saga step retries and outbound resilience share
// one knob set instead of duplicating retry logic here.
func runWithPolicy(ctx context.Context, p RetryPolicy, resource string, fn func(context.Context) error) error {
	if p == (RetryPolicy{}) {
		return fn(ctx)
	}
	drv, err := resilience.MustGetDriver("default")
	if err != nil {
		return err
	}
	exec, err := drv.NewExecutor(p)
	if err != nil {
		return err
	}
	defer func() { _ = exec.Close() }()
	return exec.Execute(ctx, resource, fn)
}
