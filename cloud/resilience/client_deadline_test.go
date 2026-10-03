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

package resilience

import (
	"context"
	"testing"
	"time"

	"go-spring.org/cloud/propagate"
	"go-spring.org/stdlib/testing/assert"
)

// TestInjectBudgetWritesOnlyABudget proves "no budget" and "no time left" travel
// as different statements: a context without a deadline writes nothing at all,
// because a receiver reading the field would otherwise bound itself by zero.
func TestInjectBudgetWritesOnlyABudget(t *testing.T) {
	c := propagate.MultiMap{}
	InjectBudget(context.Background(), c)
	assert.Number(t, len(c)).Equal(0)

	// A budget already spent still travels as a well-formed positive number; it
	// expires on arrival either way, which is the truth.
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	InjectBudget(ctx, c)
	assert.Slice(t, c.Values(BudgetHeader)).Equal([]string{"1"})
}

// TestBudgetRoundTrip proves the two halves agree: what a hop writes is what the
// next hop bounds itself by, and the bound never exceeds what was sent.
func TestBudgetRoundTrip(t *testing.T) {
	c := propagate.MultiMap{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	InjectBudget(ctx, c)

	got, cancelGot := WithBudget(context.Background(), c)
	defer cancelGot()
	d, ok := got.Deadline()
	assert.That(t, ok).True()
	assert.That(t, time.Until(d) <= time.Hour).True()
	assert.That(t, time.Until(d) > 59*time.Minute).True()
}

// TestWithBudgetNeverExtends proves the receiving hop can only shorten: a budget
// larger than the deadline ctx already carries is ignored, so a caller cannot
// hand itself more time by claiming one.
func TestWithBudgetNeverExtends(t *testing.T) {
	c := propagate.MultiMap{}
	c.Set(BudgetHeader, "600000") // ten minutes

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	got, cancelGot := WithBudget(ctx, c)
	defer cancelGot()
	assert.That(t, got == ctx).True() // unchanged: the tighter deadline stands

	// The other direction: a tighter budget wins over a looser ambient deadline.
	c.Set(BudgetHeader, "10")
	got2, cancelGot2 := WithBudget(ctx, c)
	defer cancelGot2()
	d, ok := got2.Deadline()
	assert.That(t, ok).True()
	assert.That(t, time.Until(d) <= 20*time.Millisecond).True()
}

// TestWithBudgetIgnoresAnUnusableValue proves a value from outside the process is
// treated as advice, not authority: malformed, zero and negative are all read as
// "no budget" rather than as a bound, and the context comes back untouched.
func TestWithBudgetIgnoresAnUnusableValue(t *testing.T) {
	for _, v := range []string{"", "abc", "0", "-5"} {
		c := propagate.MultiMap{}
		c.Set(BudgetHeader, v)
		ctx := context.Background()
		got, cancel := WithBudget(ctx, c)
		cancel()
		assert.That(t, got == ctx).True()
	}
}
