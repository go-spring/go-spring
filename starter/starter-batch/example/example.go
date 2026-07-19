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

// Command example is the acceptance smoke test for starter-batch +
// starter-batch-redis. It runs a chunk-oriented batch job that copies a range
// of integers into a Redis set, and proves the framework's restart guarantee
// across a *process crash*:
//
//   - PHASE=1: the writer os.Exit(1)s once about half the items have committed,
//     simulating a process dying mid-run. The already-committed chunks (and the
//     step checkpoint) are durable in Redis via starter-batch-redis.
//   - PHASE=2: a fresh process launches the same (name, params) job instance.
//     The Redis JobRepository reports the prior run as incomplete, so the step
//     resumes from the last committed checkpoint instead of restarting. When it
//     finishes, every item has been written exactly once (ReadCount ==
//     WriteCount == total and SCARD(done) == total).
//
// check.sh drives both phases against a docker Redis.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/batch"

	StarterBatch "go-spring.org/starter-batch"

	// Blank-import the redis client starter (publishes *redis.Client under
	// spring.go-redis.<name>) and the redis-backed batch repository (contributes
	// a batch.JobRepository under spring.batch-repository.<name>). Sibling
	// modules resolve through the go.work workspace with no require directives.
	_ "go-spring.org/starter-batch-redis"
	_ "go-spring.org/starter-go-redis"
)

const (
	jobName   = "reconcile"
	stepName  = "load"
	total     = 10000
	chunkSize = 100
	// doneKey is the Redis set the writer SADDs each processed id into. It is
	// separate from the repository's own keys (which live under job:/steps:).
	doneKey = "starter-batch:example:done"
)

// phase selects the run behaviour; "1" makes the writer crash midway.
var phase = os.Getenv("PHASE")

// reconcileJob is a JobDefinition whose writer needs a live *redis.Client, so
// it cannot be expressed as a plain Provide(name, steps...) — it autowires the
// client and builds its ChunkStep in Build.
type reconcileJob struct {
	Client *redis.Client `autowire:"cache"`
}

func (j *reconcileJob) JobName() string { return jobName }

func (j *reconcileJob) Build() (*batch.Job, error) {
	step := &batch.ChunkStep[int, int]{
		Name:      stepName,
		Reader:    &seqReader{n: total},
		Processor: batch.Passthrough[int](),
		Writer: batch.WriterFunc[int](func(ctx context.Context, items []int) error {
			// PHASE=1: crash once half the items have committed. We exit BEFORE
			// writing this chunk, so the crash lands cleanly between commits: the
			// committed chunks are durable and this chunk simply replays on
			// restart.
			if phase == "1" {
				n, err := j.Client.SCard(ctx, doneKey).Result()
				if err != nil {
					return err
				}
				if n >= total/2 {
					fmt.Printf("PHASE 1: simulating crash after %d items committed\n", n)
					os.Exit(1)
				}
			}
			args := make([]any, len(items))
			for i, v := range items {
				args[i] = v
			}
			return j.Client.SAdd(ctx, doneKey, args...).Err()
		}),
		ChunkSize: chunkSize,
	}
	return &batch.Job{Name: jobName, Steps: []batch.Step{step}}, nil
}

// seqReader emits the integers 1..n and is a batch.Checkpointer: its checkpoint
// is the count already read, so Open(cp) resumes just past cp. This is what
// lets a restart pick up where the crash left off.
type seqReader struct {
	n   int
	pos int
}

func (r *seqReader) Open(_ context.Context, cp batch.Checkpoint) error {
	if len(cp) == 0 {
		r.pos = 0
		return nil
	}
	p, err := strconv.Atoi(string(cp))
	if err != nil {
		return err
	}
	r.pos = p
	return nil
}

func (r *seqReader) Read(_ context.Context) (int, bool, error) {
	if r.pos >= r.n {
		return 0, false, nil
	}
	r.pos++
	return r.pos, true, nil
}

func (r *seqReader) Checkpoint() batch.Checkpoint {
	return batch.Checkpoint(strconv.Itoa(r.pos))
}

// Runner drives the smoke test. It injects the shared *Launcher (the same seam
// a scheduler.Job would use) and the redis client (to observe the result set).
type Runner struct {
	Launcher *StarterBatch.Launcher `autowire:""`
	Client   *redis.Client          `autowire:"cache"`
}

func main() {
	gs.Provide(&reconcileJob{}).
		Name(jobName).
		Export(gs.As[StarterBatch.JobDefinition]())

	runnerBean := gs.Provide(&Runner{}).Export(gs.As[gs.Rooter]())

	go func() {
		time.Sleep(500 * time.Millisecond)
		runTest(runnerBean.Interface().(*Runner))
	}()

	gs.Run()
}

func runTest(r *Runner) {
	ctx := context.Background()

	if phase != "1" {
		// PHASE 2: show the durable checkpoint left behind by the crashed run.
		n, err := r.Client.SCard(ctx, doneKey).Result()
		if err != nil {
			fail(ctx, "SCARD before resume failed: %v", err)
		}
		fmt.Printf("PHASE 2: found %d items already committed from the crashed run; resuming\n", n)
	}

	// Launch the job. In PHASE 1 the writer os.Exit(1)s inside this call, so it
	// never returns; check.sh expects the process to exit non-zero. In PHASE 2
	// the same (name, params) instance is resumed to completion.
	je, err := r.Launcher.Launch(ctx, jobName, nil)
	if err != nil {
		fail(ctx, "launch failed: %v", err)
	}
	if je.Status != batch.StatusCompleted {
		fail(ctx, "expected job status Completed, got %s (%s)", je.Status, je.FailureMsg)
	}

	// Verify the step processed every item exactly once. ReadCount/WriteCount
	// accumulate across the restart (they are loaded from the repository), so
	// exact equality with total proves no committed chunk was reprocessed.
	se, ok, err := r.Launcher.Repository().FindStepExecution(ctx, je.ID, stepName)
	if err != nil || !ok {
		fail(ctx, "step execution not found: ok=%v err=%v", ok, err)
	}
	if se.ReadCount != total || se.WriteCount != total {
		fail(ctx, "expected read=write=%d, got read=%d write=%d", total, se.ReadCount, se.WriteCount)
	}

	// Verify the observable side effect: every id landed in the set exactly once.
	card, err := r.Client.SCard(ctx, doneKey).Result()
	if err != nil {
		fail(ctx, "SCARD after completion failed: %v", err)
	}
	if card != total {
		fail(ctx, "expected SCARD(done)=%d, got %d", total, card)
	}

	fmt.Printf("Completed: read=%d write=%d SCARD=%d (no item reprocessed)\n", se.ReadCount, se.WriteCount, card)
	fmt.Println("starter-batch smoke test passed")
	_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

func fail(ctx context.Context, format string, args ...any) {
	log.Errorf(ctx, log.TagAppDef, format, args...)
	os.Exit(1)
}

// init pins the working directory to this source file's directory so relative
// config paths resolve regardless of how the binary is invoked.
func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot resolve source file directory")
	}
	if err := os.Chdir(filepath.Dir(filename)); err != nil {
		panic(err)
	}
}
