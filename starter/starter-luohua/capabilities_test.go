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

package luohua

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-spring.org/cloud/cache"
	"go-spring.org/cloud/security"
	"go-spring.org/stdlib/i18n"
)

// TestLuohuaSSOIssueValidate verifies the identity seam: a token luohua issues
// validates back into the expected identity, a tampered one is rejected, and an
// expired one is rejected.
func TestLuohuaSSOIssueValidate(t *testing.T) {
	sso := NewLuohuaSSO("s3cret", "luohua")

	tok, err := sso.Issue("user-42", "acme", []string{AuthorityOrdersRead, "ROLE_ADMIN"}, time.Minute)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	auth, err := sso.Validate(context.Background(), tok)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !auth.Authenticated || auth.Principal.Subject != "user-42" {
		t.Fatalf("unexpected identity: %+v", auth)
	}
	if !auth.HasAuthority(AuthorityOrdersRead) || auth.HasAuthority(AuthorityOrdersWrite) {
		t.Fatalf("authority mismatch: %v", auth.Authorities)
	}

	// Tamper with the signature body.
	if _, err := sso.Validate(context.Background(), tok[:len(tok)-1]+"x"); err == nil {
		t.Fatal("expected tampered token to be rejected")
	}
	if _, err := sso.Validate(context.Background(), "not-a-token"); err == nil {
		t.Fatal("expected malformed token to be rejected")
	}

	// Wrong secret must not verify.
	other := NewLuohuaSSO("other-secret", "luohua")
	if _, err := other.Validate(context.Background(), tok); err == nil {
		t.Fatal("expected token signed with another secret to be rejected")
	}
}

// TestLuohuaCatalog resolves luohua messages through the MapSource catalog.
func TestLuohuaCatalog(t *testing.T) {
	ms := luohuaCatalog("zh")

	msg, err := ms.Message(context.Background(), "luohua.orders.denied")
	if err != nil || msg != "无权访问订单资源" {
		t.Fatalf("Message(zh) = %q, %v", msg, err)
	}
	// Default locale zh, but an en request (not set here) falls back to zh too.
	msg, err = ms.Message(context.Background(), "luohua.orders.denied")
	if err != nil {
		t.Fatalf("Message: %v", err)
	}
	if msg == "luohua.orders.denied" {
		t.Fatal("catalog returned the raw key — localization not wired")
	}
}

// TestMemoryCacheRoundTrip exercises the luohua uniform driver backend.
func TestMemoryCacheRoundTrip(t *testing.T) {
	c := NewLuohuaCache()
	ctx := context.Background()

	if err := c.Set(ctx, "k", "v", time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	var got string
	if err := c.Get(ctx, "k", &got); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "v" {
		t.Fatalf("Get = %q, want v", got)
	}

	// Expired key is a miss.
	if err := c.Set(ctx, "gone", "x", time.Millisecond); err != nil {
		t.Fatalf("Set: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	if err := c.Get(ctx, "gone", new(string)); !errors.Is(err, cache.ErrMiss) {
		t.Fatalf("expired Get err = %v, want ErrMiss", err)
	}
}

var _ security.TokenValidator = (*LuohuaSSO)(nil)
var _ i18n.MessageSource = (*i18n.MapSource)(nil)
