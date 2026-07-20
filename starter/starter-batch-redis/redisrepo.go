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

package StarterBatchRedis

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go-spring.org/spring/batch"
)

// redisRepository implements batch.JobRepository over a *redis.Client. It is
// the durable counterpart of batch.NewMemoryRepository: JobExecution records
// live at a per-instance key, and every StepExecution of an execution shares a
// single hash keyed by the execution ID. Both layouts survive a process crash
// so a restart resumes from the last committed chunk.
//
// Key layout (all optionally prefixed with Config.KeyPrefix):
//
//	<prefix>job:<instanceKey>       string, JSON-encoded JobExecution
//	<prefix>steps:<jobExecutionID>  hash, field=stepName → JSON StepExecution
//	<prefix>seq                     counter used to mint fresh execution IDs
//
// The instanceKey is derived from JobName + sorted Params (SHA-1 hex), which
// mirrors the unexported instanceKey helper in stdlib/batch/repository.go so
// the Redis backend and the in-memory backend agree on what "same instance"
// means.
type redisRepository struct {
	cfg    Config
	client *redis.Client
}

// newRedisRepository builds a repository over an already-constructed
// *redis.Client. The client's lifecycle (Ping, Close, ...) is owned by
// starter-go-redis; this repository never closes it. Constructor validation is
// deliberately minimal: the only unrecoverable configuration error is a nil
// client, which would surface as a nil-deref on the first call otherwise.
func newRedisRepository(c Config, client *redis.Client) (batch.JobRepository, error) {
	if client == nil {
		return nil, errors.New("batch-redis: nil *redis.Client")
	}
	return &redisRepository{cfg: c, client: client}, nil
}

// jobKey returns the string key that stores the JobExecution for an instance.
// It is written by ObtainExecution / SaveJobExecution and read by
// ObtainExecution to decide restart vs fresh.
func (r *redisRepository) jobKey(instanceKey string) string {
	return r.cfg.KeyPrefix + "job:" + instanceKey
}

// stepsKey returns the hash key that stores every StepExecution of a job
// execution as its own field. HSET / HGET / HGETALL then give us
// SaveStepExecution / FindStepExecution / ListStepExecutions in one round-trip
// each — this is why steps share a hash instead of using one key per step.
func (r *redisRepository) stepsKey(jobExecutionID string) string {
	return r.cfg.KeyPrefix + "steps:" + jobExecutionID
}

// seqKey is the INCR counter used to make execution IDs monotonically unique.
// Redis INCR is atomic, so ObtainExecution can mint IDs concurrently without
// coordination.
func (r *redisRepository) seqKey() string {
	return r.cfg.KeyPrefix + "seq"
}

// instanceKey mirrors the unexported helper in stdlib/batch/repository.go: it
// sorts params by key and SHA-1s "<name>\0<k>=<v>\0..." so two runs with the
// same JobName + Params hash to the same key, regardless of map iteration
// order. Keeping the exact algorithm in lockstep with the memory repo means
// callers see identical instance semantics on either backend.
func instanceKey(name string, params batch.Params) string {
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

// applyTTL sets EXPIRE on `key` when Config.TTL > 0. TTL == 0 leaves the key
// without an expiry so long-running jobs never lose their history mid-run;
// callers that care about GC set a positive TTL to bound Redis growth.
func (r *redisRepository) applyTTL(ctx context.Context, key string) error {
	if r.cfg.TTL <= 0 {
		return nil
	}
	return r.client.Expire(ctx, key, r.cfg.TTL).Err()
}

// ObtainExecution finds or creates the JobExecution for (name, params).
//
// If the stored execution exists AND is not StatusCompleted, it is returned
// with restart=true so the engine resumes it. If it is Completed (or missing),
// a fresh execution is written with a monotonically unique ID and
// restart=false. This matches the contract of the memory repo and the
// interface docstring in stdlib/batch/repository.go.
func (r *redisRepository) ObtainExecution(ctx context.Context, name string, params batch.Params) (*batch.JobExecution, bool, error) {
	ik := instanceKey(name, params)
	jkey := r.jobKey(ik)

	raw, err := r.client.Get(ctx, jkey).Bytes()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, false, err
	}
	if err == nil {
		var je batch.JobExecution
		if err := json.Unmarshal(raw, &je); err != nil {
			return nil, false, fmt.Errorf("batch-redis: decode job %s: %w", ik, err)
		}
		if je.Status != batch.StatusCompleted {
			return &je, true, nil
		}
	}

	// No existing execution to resume — mint a new one. INCR is atomic so
	// concurrent ObtainExecution calls on different instances still get
	// distinct IDs.
	n, err := r.client.Incr(ctx, r.seqKey()).Result()
	if err != nil {
		return nil, false, err
	}
	je := &batch.JobExecution{
		ID:        fmt.Sprintf("%s-%d", ik[:8], n),
		JobName:   name,
		Params:    cloneParams(params),
		Status:    batch.StatusPending,
		StartTime: time.Now(),
	}
	if err := r.writeJob(ctx, ik, je); err != nil {
		return nil, false, err
	}
	return cloneJob(je), false, nil
}

// SaveJobExecution overwrites the stored JobExecution record. It is called on
// every status transition so a crash between transitions still leaves a
// coherent snapshot in Redis.
func (r *redisRepository) SaveJobExecution(ctx context.Context, je *batch.JobExecution) error {
	if je == nil {
		return errors.New("batch-redis: SaveJobExecution nil execution")
	}
	return r.writeJob(ctx, instanceKey(je.JobName, je.Params), je)
}

// writeJob is the single JSON+SET+EXPIRE path used by both ObtainExecution
// (fresh creation) and SaveJobExecution (updates), so TTL bookkeeping and
// encoding stay in one place.
func (r *redisRepository) writeJob(ctx context.Context, ik string, je *batch.JobExecution) error {
	buf, err := json.Marshal(je)
	if err != nil {
		return fmt.Errorf("batch-redis: encode job %s: %w", je.ID, err)
	}
	jkey := r.jobKey(ik)
	if err := r.client.Set(ctx, jkey, buf, 0).Err(); err != nil {
		return err
	}
	return r.applyTTL(ctx, jkey)
}

// SaveStepExecution is the durable commit point of a chunk: the engine calls
// it after each committed chunk, so its HSET has to be atomic (it is, in
// Redis) and idempotent for the same StepName. We refresh TTL on every save
// so a long-running step keeps its records alive.
func (r *redisRepository) SaveStepExecution(ctx context.Context, se *batch.StepExecution) error {
	if se == nil {
		return errors.New("batch-redis: SaveStepExecution nil execution")
	}
	buf, err := json.Marshal(se)
	if err != nil {
		return fmt.Errorf("batch-redis: encode step %s/%s: %w", se.JobExecutionID, se.StepName, err)
	}
	skey := r.stepsKey(se.JobExecutionID)
	if err := r.client.HSet(ctx, skey, se.StepName, buf).Err(); err != nil {
		return err
	}
	return r.applyTTL(ctx, skey)
}

// FindStepExecution reads one field of the per-execution hash. redis.Nil on
// the field is translated to ok=false so callers do not have to know the
// backend to differentiate "not started" from a transport error.
func (r *redisRepository) FindStepExecution(ctx context.Context, jobExecutionID, stepName string) (*batch.StepExecution, bool, error) {
	raw, err := r.client.HGet(ctx, r.stepsKey(jobExecutionID), stepName).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var se batch.StepExecution
	if err := json.Unmarshal(raw, &se); err != nil {
		return nil, false, fmt.Errorf("batch-redis: decode step %s/%s: %w", jobExecutionID, stepName, err)
	}
	return &se, true, nil
}

// ListStepExecutions returns every step of one job execution. It is used for
// progress queries so order is unspecified — HGETALL gives us all fields in a
// single round-trip, which is cheaper than paginating.
func (r *redisRepository) ListStepExecutions(ctx context.Context, jobExecutionID string) ([]*batch.StepExecution, error) {
	m, err := r.client.HGetAll(ctx, r.stepsKey(jobExecutionID)).Result()
	if err != nil {
		return nil, err
	}
	out := make([]*batch.StepExecution, 0, len(m))
	for name, raw := range m {
		var se batch.StepExecution
		if err := json.Unmarshal([]byte(raw), &se); err != nil {
			return nil, fmt.Errorf("batch-redis: decode step %s/%s: %w", jobExecutionID, name, err)
		}
		out = append(out, &se)
	}
	return out, nil
}

// cloneJob / cloneParams keep the "returned pointer is not the stored pointer"
// discipline of the memory repo, so an ObtainExecution caller mutating the
// returned struct cannot corrupt an in-flight write.
func cloneJob(je *batch.JobExecution) *batch.JobExecution {
	cp := *je
	cp.Params = cloneParams(je.Params)
	return &cp
}

func cloneParams(p batch.Params) batch.Params {
	if p == nil {
		return nil
	}
	out := make(batch.Params, len(p))
	for k, v := range p {
		out[k] = v
	}
	return out
}

// Compile-time interface check.
var _ batch.JobRepository = (*redisRepository)(nil)
