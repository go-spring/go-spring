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

package batch

import (
	"context"
	"fmt"
	"time"
)

// StepContext carries the state a [Step] needs to run and to record progress. A
// step reads its resume point from StepExecution and commits progress by saving
// it back through Repo.
type StepContext struct {
	// JobExecutionID is the owning job execution's ID.
	JobExecutionID string
	// Repo is where the step commits progress after each chunk.
	Repo JobRepository
	// StepExecution is this step's record, pre-loaded with any progress from a
	// previous run (ReadCount/WriteCount/Checkpoint) on restart.
	StepExecution *StepExecution
}

// Step is one unit of a [Job]. A [ChunkStep] implements it for chunk processing;
// [Func] wraps a plain function as a single-run step for Cloud Task semantics.
// Generics are erased behind this interface so a Job can hold steps of differing
// item types.
type Step interface {
	// StepName identifies the step within its job; it must be unique.
	StepName() string
	// Run executes the step, committing progress through rc.Repo. It should
	// honour ctx for cancellation and leave the step restartable on failure.
	Run(ctx context.Context, rc *StepContext) error
}

// Job is an ordered sequence of [Step]s run as one unit. Steps run in order;
// the first failing step fails the job, and a restart of the same instance skips
// steps that already completed and resumes the one that did not.
type Job struct {
	// Name identifies the job; together with [Params] it identifies an instance.
	Name string
	// Steps run in order.
	Steps []Step
}

// Run executes the job against repo for the (Name, params) instance. If a prior
// execution of the instance did not complete, it resumes: completed steps are
// skipped and the interrupted step continues from its checkpoint. It returns the
// (updated) [JobExecution] so callers can inspect status and counts.
func (j *Job) Run(ctx context.Context, repo JobRepository, params Params) (*JobExecution, error) {
	if repo == nil {
		return nil, ErrNoRepository
	}

	je, _, err := repo.ObtainExecution(ctx, j.Name, params)
	if err != nil {
		return nil, err
	}

	je.Status = StatusStarted
	je.FailureMsg = ""
	je.EndTime = time.Time{}
	if je.StartTime.IsZero() {
		je.StartTime = time.Now()
	}
	if err := repo.SaveJobExecution(ctx, je); err != nil {
		return je, err
	}

	for _, step := range j.Steps {
		se, ok, err := repo.FindStepExecution(ctx, je.ID, step.StepName())
		if err != nil {
			return je, err
		}
		if ok && se.Status == StatusCompleted {
			continue // already done on a previous run; skip it on restart
		}
		if !ok {
			se = &StepExecution{
				JobExecutionID: je.ID,
				StepName:       step.StepName(),
				Status:         StatusStarted,
				StartTime:      time.Now(),
			}
		} else {
			se.Status = StatusStarted
			se.FailureMsg = ""
			se.EndTime = time.Time{}
			if se.StartTime.IsZero() {
				se.StartTime = time.Now()
			}
		}
		if err := repo.SaveStepExecution(ctx, se); err != nil {
			return je, err
		}

		rc := &StepContext{JobExecutionID: je.ID, Repo: repo, StepExecution: se}
		runErr := step.Run(ctx, rc)

		se.EndTime = time.Now()
		if runErr != nil {
			// Distinguish a clean stop (context cancelled) from a failure so a
			// deliberate shutdown is restartable without looking like an error.
			if ctx.Err() != nil {
				se.Status = StatusStopped
			} else {
				se.Status = StatusFailed
				se.FailureMsg = runErr.Error()
			}
			_ = repo.SaveStepExecution(ctx, se)

			je.EndTime = time.Now()
			je.Status = se.Status
			je.FailureMsg = fmt.Sprintf("step %q: %v", step.StepName(), runErr)
			_ = repo.SaveJobExecution(ctx, je)
			return je, runErr
		}

		se.Status = StatusCompleted
		if err := repo.SaveStepExecution(ctx, se); err != nil {
			return je, err
		}
	}

	je.Status = StatusCompleted
	je.EndTime = time.Now()
	if err := repo.SaveJobExecution(ctx, je); err != nil {
		return je, err
	}
	return je, nil
}

// funcStep is a [Step] that runs a plain function once, with no chunking. It is
// the Cloud Task building block: the step completes when fn returns nil and
// fails when it returns an error, and it does not resume mid-way (a restart
// re-runs fn), so fn should be idempotent if it might be retried.
type funcStep struct {
	name string
	fn   func(ctx context.Context) error
}

func (s *funcStep) StepName() string { return s.name }

func (s *funcStep) Run(ctx context.Context, rc *StepContext) error {
	if err := s.fn(ctx); err != nil {
		return err
	}
	rc.StepExecution.ReadCount = 1
	rc.StepExecution.WriteCount = 1
	return rc.Repo.SaveStepExecution(ctx, rc.StepExecution)
}

// Func wraps a one-shot function as a single-run [Step], the Go-idiomatic
// equivalent of a Spring Cloud Task: a short-lived job that runs once and
// records its outcome in the [JobRepository]. Compose it into a one-step [Job]:
//
//	job := &batch.Job{Name: "report", Steps: []batch.Step{
//	    batch.Func("generate", svc.GenerateReport),
//	}}
//
// It panics on an empty name or nil function, since either makes the step
// unusable.
func Func(name string, fn func(ctx context.Context) error) Step {
	if name == "" {
		panic("batch: func step name must not be empty")
	}
	if fn == nil {
		panic("batch: func step function must not be nil")
	}
	return &funcStep{name: name, fn: fn}
}
