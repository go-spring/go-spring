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

package StarterBatch

import (
	"context"

	"go-spring.org/log"
	"go-spring.org/stdlib/batch"
	"go-spring.org/stdlib/errutil"
)

// Launcher launches registered [JobDefinition]s on demand against the shared
// [batch.JobRepository]. It is the injectable seam a scheduler.Job (or an HTTP
// handler, or another server) uses to trigger a batch run:
//
//	type NightlyReconcile struct {
//	    Launcher *batch.Launcher `autowire:""`
//	}
//
//	func (n *NightlyReconcile) JobName() string { return "nightly-reconcile" }
//
//	func (n *NightlyReconcile) Run(ctx context.Context) error {
//	    _, err := n.Launcher.Launch(ctx, "reconcile", batch.Params{
//	        "date": time.Now().Format("2006-01-02"),
//	    })
//	    return err
//	}
//
// Launch is the same call the runner uses for `run-on-startup` jobs — a manual
// launch and a startup launch share the exact same repository and code path, so
// restart semantics are consistent regardless of how the run was triggered.
//
// Its exported fields are populated by the IoC container. The Init method wires
// them into the (repo, defs) pair before any Launch call.
type Launcher struct {
	// Config is bound from ${spring.batch} so the launcher can honour the
	// operator's repository choice.
	Config Config `value:"${spring.batch}"`

	// Defs are all beans exported as JobDefinition — the batch jobs the
	// application registered with NewJob / Provide.
	Defs []JobDefinition `autowire:"?"`

	// Repos are all batch.JobRepository beans, keyed by bean name, so
	// Config.Repository can pick one by name; when only one is present it is
	// picked implicitly.
	Repos map[string]batch.JobRepository `autowire:"?"`

	repo batch.JobRepository
	defs map[string]JobDefinition
}

// Init wires the launcher's exported fields into the resolved (repo, defs)
// pair. It is called by the container as a bean-init lifecycle hook, so a
// mis-named repository or a duplicate JobDefinition surfaces at boot rather
// than at launch. Init is safe to call more than once — subsequent calls
// re-resolve.
func (l *Launcher) Init() error {
	repo, err := pickRepository(l.Config, l.Repos)
	if err != nil {
		return err
	}
	m := make(map[string]JobDefinition, len(l.Defs))
	for _, d := range l.Defs {
		name := d.JobName()
		if name == "" {
			return errutil.Explain(nil, "batch: JobDefinition bean has empty name")
		}
		if _, dup := m[name]; dup {
			return errutil.Explain(nil, "batch: duplicate JobDefinition bean named %q", name)
		}
		// Materialise once to fail fast on builder errors (missing writer,
		// duplicated step name, ...); the built job is discarded — Launch
		// rebuilds on demand so subsequent runs pick up any per-run state a
		// caller's builder wants to inject.
		if _, err := d.Build(); err != nil {
			return errutil.Explain(err, "batch: build job %q", name)
		}
		m[name] = d
	}
	// Validate every configured job references a real definition — a
	// run-on-startup=true entry with no bean is a clear mistake.
	for name := range l.Config.Jobs {
		if _, ok := m[name]; !ok {
			return errutil.Explain(nil,
				"batch: job %q is configured under spring.batch.jobs but no JobDefinition bean of that name is registered", name)
		}
	}
	l.repo = repo
	l.defs = m
	return nil
}

// Launch resolves the named [JobDefinition], builds its [*batch.Job], and runs
// it against the shared repository. It returns the (updated) [batch.JobExecution]
// so callers can inspect status and counts. An unknown job name is a clear
// error rather than a silent no-op.
func (l *Launcher) Launch(ctx context.Context, name string, params batch.Params) (*batch.JobExecution, error) {
	if l == nil {
		return nil, errutil.Explain(nil, "batch: launcher is nil")
	}
	def, ok := l.defs[name]
	if !ok {
		return nil, errutil.Explain(nil, "batch: unknown job %q (no JobDefinition bean of that name is registered)", name)
	}
	job, err := def.Build()
	if err != nil {
		return nil, errutil.Explain(err, "batch: build job %q", name)
	}
	if params == nil {
		params = batch.Params{}
	}
	return job.Run(ctx, l.repo, params)
}

// Repository exposes the repository the launcher runs jobs against, so callers
// that need to query execution status (for a `/jobs/<id>` endpoint, say) can
// share the same store the runner uses.
func (l *Launcher) Repository() batch.JobRepository {
	if l == nil {
		return nil
	}
	return l.repo
}

// Names returns the names of every job the launcher can launch. It is intended
// for diagnostics (an actuator endpoint, a log line at boot); the order is
// unspecified.
func (l *Launcher) Names() []string {
	if l == nil {
		return nil
	}
	out := make([]string, 0, len(l.defs))
	for name := range l.defs {
		out = append(out, name)
	}
	return out
}

// pickRepository resolves the [batch.JobRepository] to use, in three steps:
//
//  1. If cfg.Repository is set, it must name an existing repo bean; missing is
//     a fail-fast error (not a silent fallback to memory).
//  2. Otherwise, if exactly one repo bean exists, use it — the common case for
//     an app that imports one starter-batch-<backend>.
//  3. Otherwise, fall back to [batch.NewMemoryRepository]. If there are
//     multiple repo beans and none was named, the operator has to disambiguate
//     — silently picking one would surprise them.
func pickRepository(cfg Config, repos map[string]batch.JobRepository) (batch.JobRepository, error) {
	if cfg.Repository != "" {
		repo, ok := repos[cfg.Repository]
		if !ok {
			return nil, errutil.Explain(nil,
				"batch: spring.batch.repository=%q but no batch.JobRepository bean of that name is registered", cfg.Repository)
		}
		return repo, nil
	}
	switch len(repos) {
	case 0:
		log.Infof(context.Background(), log.TagAppDef,
			"batch: no JobRepository bean present; using in-process NewMemoryRepository (not durable across restarts)")
		return batch.NewMemoryRepository(), nil
	case 1:
		for _, repo := range repos {
			return repo, nil
		}
	}
	return nil, errutil.Explain(nil,
		"batch: %d batch.JobRepository beans present but spring.batch.repository is empty; name one explicitly to disambiguate", len(repos))
}
