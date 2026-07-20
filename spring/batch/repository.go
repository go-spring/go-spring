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
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"maps"
	"sort"
	"strings"
	"sync"
	"time"
)

// JobExecution records one run of a job instance. A restart reuses the same
// JobExecution (same ID) so its step history is carried forward.
type JobExecution struct {
	// ID uniquely identifies this execution. It is stable across a restart of the
	// same (name, params) instance.
	ID string
	// JobName is the job's name.
	JobName string
	// Params are the instance parameters; together with JobName they identify the
	// job instance.
	Params Params
	// Status is the current lifecycle state.
	Status BatchStatus
	// StartTime is when the execution first started; EndTime is when it reached a
	// terminal state (zero while running).
	StartTime time.Time
	EndTime   time.Time
	// FailureMsg carries the error message when Status is StatusFailed.
	FailureMsg string
}

// StepExecution records the progress of one step within a [JobExecution]. It is
// the durable checkpoint that makes restart possible: ReadCount/WriteCount and
// Checkpoint are updated after every committed chunk.
type StepExecution struct {
	// JobExecutionID ties this step to its owning [JobExecution].
	JobExecutionID string
	// StepName is the step's name, unique within a job.
	StepName string
	// Status is the current lifecycle state.
	Status BatchStatus
	// ReadCount is how many items have been read and ReadCount how many written,
	// across all committed chunks so far.
	ReadCount  int64
	WriteCount int64
	// Checkpoint is the reader position persisted after the last committed chunk;
	// it is handed back to the reader on restart.
	Checkpoint Checkpoint
	// StartTime is when the step first started; EndTime is when it reached a
	// terminal state (zero while running).
	StartTime time.Time
	EndTime   time.Time
	// FailureMsg carries the error message when Status is StatusFailed.
	FailureMsg string
}

// JobRepository stores job and step execution state. It is the single seam
// through which the batch engine persists progress; a durable implementation
// (Redis, a database) makes restart survive a process crash. Implementations
// must be safe for concurrent use.
//
// The interface is the backend seam by design (no global driver registry): a
// backend needs a live client, so switching backends is a bean-type swap, the
// same choice the lock package makes.
type JobRepository interface {
	// ObtainExecution returns the execution to run for the (name, params)
	// instance. If a prior execution exists and did not complete it is returned
	// with restart=true so the engine resumes it; otherwise a fresh execution is
	// created (restart=false). A completed instance is re-run as a new execution.
	ObtainExecution(ctx context.Context, name string, params Params) (je *JobExecution, restart bool, err error)

	// SaveJobExecution persists the (possibly updated) job execution.
	SaveJobExecution(ctx context.Context, je *JobExecution) error

	// SaveStepExecution persists the (possibly updated) step execution. It is
	// called once per committed chunk, so it must be the durable commit point.
	SaveStepExecution(ctx context.Context, se *StepExecution) error

	// FindStepExecution returns the step execution for a job execution, or
	// ok=false if the step has not started yet.
	FindStepExecution(ctx context.Context, jobExecutionID, stepName string) (se *StepExecution, ok bool, err error)

	// ListStepExecutions returns all step executions of a job execution, for
	// progress queries. Order is unspecified.
	ListStepExecutions(ctx context.Context, jobExecutionID string) ([]*StepExecution, error)
}

// instanceKey derives a stable identifier for a job instance from its name and
// params, so ObtainExecution can find a prior execution of the same instance.
func instanceKey(name string, params Params) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(name)
	for _, k := range keys {
		b.WriteByte(0)
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(params[k])
	}
	sum := sha1.Sum([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// memoryRepository is an in-process [JobRepository] backed by maps. It keeps
// execution state only for the lifetime of the process, so it cannot recover a
// crashed run — it is meant for tests and single-shot in-memory jobs. Durable
// restart uses a backend that survives the process.
type memoryRepository struct {
	mu sync.RWMutex
	// byInstance maps an instance key to the current execution ID.
	byInstance map[string]string
	// jobs maps execution ID to its JobExecution.
	jobs map[string]*JobExecution
	// steps maps execution ID to stepName to StepExecution.
	steps map[string]map[string]*StepExecution
	seq   int64
}

// NewMemoryRepository returns an in-process [JobRepository]. State lives only as
// long as the process, so it is for tests and single-process runs; use a durable
// backend to survive a crash and restart from a checkpoint.
func NewMemoryRepository() JobRepository {
	return &memoryRepository{
		byInstance: map[string]string{},
		jobs:       map[string]*JobExecution{},
		steps:      map[string]map[string]*StepExecution{},
	}
}

func (r *memoryRepository) ObtainExecution(_ context.Context, name string, params Params) (*JobExecution, bool, error) {
	key := instanceKey(name, params)
	r.mu.Lock()
	defer r.mu.Unlock()

	if id, ok := r.byInstance[key]; ok {
		if je, ok := r.jobs[id]; ok && je.Status != StatusCompleted {
			return cloneJob(je), true, nil
		}
	}

	r.seq++
	je := &JobExecution{
		ID:        fmt.Sprintf("%s-%d", key[:8], r.seq),
		JobName:   name,
		Params:    params,
		Status:    StatusPending,
		StartTime: time.Now(),
	}
	r.byInstance[key] = je.ID
	r.jobs[je.ID] = cloneJob(je)
	r.steps[je.ID] = map[string]*StepExecution{}
	return cloneJob(je), false, nil
}

func (r *memoryRepository) SaveJobExecution(_ context.Context, je *JobExecution) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs[je.ID] = cloneJob(je)
	return nil
}

func (r *memoryRepository) SaveStepExecution(_ context.Context, se *StepExecution) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.steps[se.JobExecutionID]
	if !ok {
		m = map[string]*StepExecution{}
		r.steps[se.JobExecutionID] = m
	}
	m[se.StepName] = cloneStep(se)
	return nil
}

func (r *memoryRepository) FindStepExecution(_ context.Context, jobExecutionID, stepName string) (*StepExecution, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.steps[jobExecutionID]
	if !ok {
		return nil, false, nil
	}
	se, ok := m[stepName]
	if !ok {
		return nil, false, nil
	}
	return cloneStep(se), true, nil
}

func (r *memoryRepository) ListStepExecutions(_ context.Context, jobExecutionID string) ([]*StepExecution, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m := r.steps[jobExecutionID]
	out := make([]*StepExecution, 0, len(m))
	for _, se := range m {
		out = append(out, cloneStep(se))
	}
	return out, nil
}

// cloneJob and cloneStep return deep-enough copies so callers cannot mutate the
// repository's stored state by holding onto a returned pointer.
func cloneJob(je *JobExecution) *JobExecution {
	cp := *je
	if je.Params != nil {
		cp.Params = make(Params, len(je.Params))
		maps.Copy(cp.Params, je.Params)
	}
	return &cp
}

func cloneStep(se *StepExecution) *StepExecution {
	cp := *se
	if se.Checkpoint != nil {
		cp.Checkpoint = append(Checkpoint(nil), se.Checkpoint...)
	}
	return &cp
}
