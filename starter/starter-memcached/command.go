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

// command.go is the "command seam" concept of this starter: the per-operation
// methods of Client, each routed through the shared observe span (instrument)
// and the resilience executor (guard/guardErr). gomemcache exposes no hook or
// plugin point, so the command surface is hand-written here — the analog of
// starter-redigo's conn.go, only without a native chain to fold onto.
package StarterMemcached

import (
	"context"
	"errors"
	"fmt"

	"github.com/bradfitz/gomemcache/memcache"
	"go-spring.org/cloud/governance/resilience"
)

// instrument wraps a single-key operation: op names it, key is the log/span arg.
// It starts an observe span and returns an end callback the caller invokes with
// the operation's error. The resilience executor is applied by the caller via
// guard (see guard), so each method is span-instrumented here and
// resilience-protected there — two single places rather than 17 copies of each.
func (c *Client) instrument(op, key string) func(error) {
	sp := c.obs.Start(context.Background(), op, key)
	return func(err error) { sp.End(err) }
}

func (c *Client) Get(key string) (*memcache.Item, error) {
	end := c.instrument("get", key)
	it, err := guard(c.exec, c.resource, func() (*memcache.Item, error) { return c.Client.Get(key) })
	end(err)
	return it, err
}

func (c *Client) GetAndTouch(key string, expiration int32) (*memcache.Item, error) {
	end := c.instrument("get_and_touch", key)
	it, err := guard(c.exec, c.resource, func() (*memcache.Item, error) {
		return c.Client.GetAndTouch(key, expiration)
	})
	end(err)
	return it, err
}

func (c *Client) GetMulti(keys []string) (map[string]*memcache.Item, error) {
	end := c.instrument("get_multi", fmt.Sprintf("%d keys", len(keys)))
	m, err := guard(c.exec, c.resource, func() (map[string]*memcache.Item, error) {
		return c.Client.GetMulti(keys)
	})
	end(err)
	return m, err
}

func (c *Client) Touch(key string, seconds int32) error {
	end := c.instrument("touch", key)
	err := guardErr(c.exec, c.resource, func() error { return c.Client.Touch(key, seconds) })
	end(err)
	return err
}

func (c *Client) Set(item *memcache.Item) error {
	end := c.instrument("set", item.Key)
	err := guardErr(c.exec, c.resource, func() error { return c.Client.Set(item) })
	end(err)
	return err
}

func (c *Client) Add(item *memcache.Item) error {
	end := c.instrument("add", item.Key)
	err := guardErr(c.exec, c.resource, func() error { return c.Client.Add(item) })
	end(err)
	return err
}

func (c *Client) Replace(item *memcache.Item) error {
	end := c.instrument("replace", item.Key)
	err := guardErr(c.exec, c.resource, func() error { return c.Client.Replace(item) })
	end(err)
	return err
}

func (c *Client) Append(item *memcache.Item) error {
	end := c.instrument("append", item.Key)
	err := guardErr(c.exec, c.resource, func() error { return c.Client.Append(item) })
	end(err)
	return err
}

func (c *Client) Prepend(item *memcache.Item) error {
	end := c.instrument("prepend", item.Key)
	err := guardErr(c.exec, c.resource, func() error { return c.Client.Prepend(item) })
	end(err)
	return err
}

func (c *Client) CompareAndSwap(item *memcache.Item) error {
	end := c.instrument("cas", item.Key)
	err := guardErr(c.exec, c.resource, func() error { return c.Client.CompareAndSwap(item) })
	end(err)
	return err
}

func (c *Client) Delete(key string) error {
	end := c.instrument("delete", key)
	err := guardErr(c.exec, c.resource, func() error { return c.Client.Delete(key) })
	end(err)
	return err
}

func (c *Client) DeleteAll() error {
	end := c.instrument("delete_all", "")
	err := guardErr(c.exec, c.resource, func() error { return c.Client.DeleteAll() })
	end(err)
	return err
}

func (c *Client) Increment(key string, delta uint64) (uint64, error) {
	end := c.instrument("increment", key)
	n, err := guard(c.exec, c.resource, func() (uint64, error) { return c.Client.Increment(key, delta) })
	end(err)
	return n, err
}

func (c *Client) Decrement(key string, delta uint64) (uint64, error) {
	end := c.instrument("decrement", key)
	n, err := guard(c.exec, c.resource, func() (uint64, error) { return c.Client.Decrement(key, delta) })
	end(err)
	return n, err
}

func (c *Client) Ping() error {
	end := c.instrument("ping", "")
	err := guardErr(c.exec, c.resource, func() error { return c.Client.Ping() })
	end(err)
	return err
}

func (c *Client) FlushAll() error {
	end := c.instrument("flush_all", "")
	err := guardErr(c.exec, c.resource, func() error { return c.Client.FlushAll() })
	end(err)
	return err
}

// guard runs fn under the resilience executor via [resilience.Run], treating
// memcache.ErrCacheMiss as success so a cache miss never trips the breaker, and
// surfacing protection rejections (rate-limited / circuit-open / bulkhead-full)
// to the caller. When governance is off the resolved executor is a no-op, so fn
// runs with a single function-call overhead.
func guard[T any](exec resilience.Executor, resource string, fn func() (T, error)) (T, error) {
	return resilience.Run(context.Background(), exec, resource,
		func(e error) bool { return errors.Is(e, memcache.ErrCacheMiss) },
		func(context.Context) (T, error) { return fn() })
}

// guardErr is the error-only variant of [guard], for operations that return no
// payload (Set/Delete/Ping/...). It shares the same resilience + ErrCacheMiss
// semantics; the two-variant split keeps the call sites typed rather than
// routing through any + runtime assertion.
func guardErr(exec resilience.Executor, resource string, fn func() error) error {
	_, err := resilience.Run(context.Background(), exec, resource,
		func(e error) bool { return errors.Is(e, memcache.ErrCacheMiss) },
		func(context.Context) (struct{}, error) { return struct{}{}, fn() })
	return err
}
