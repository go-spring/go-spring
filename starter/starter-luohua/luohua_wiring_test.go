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
	"testing"
	"time"

	"go-spring.org/cloud/i18n"
	"go-spring.org/cloud/security"
	"go-spring.org/spring/gs"
)

// TestAssemblesDefaultBeans proves the aggregator thesis end to end: with
// spring.luohua.* armed, the container actually wires the luohua defaults — a
// security.TokenValidator, an i18n.MessageSource — through their gs.Module /
// Export seams, and injects them by interface. It is the integration-level
// counterpart to the per-capability unit tests.
func TestAssemblesDefaultBeans(t *testing.T) {
	gs.Web(false).Configure(func(app gs.App) {
		app.Property("spring.luohua.identity.secret", "s3cret")
		app.Property("spring.luohua.identity.issuer", "luohua")
		app.Property("spring.luohua.i18n.default-locale", "zh")
	}).RunTest(t, func(ts *struct {
		Validator security.TokenValidator `autowire:""`
		Messages  i18n.MessageSource      `autowire:""`
	}) {
		if ts.Validator == nil {
			t.Fatal("expected a security.TokenValidator bean from luohua.identity")
		}
		if ts.Messages == nil {
			t.Fatal("expected an i18n.MessageSource bean from luohua.i18n")
		}

		// The wired validator actually vouches for a token luohua issued.
		sso, ok := ts.Validator.(*LuohuaSSO)
		if !ok {
			t.Fatalf("validator is %T, want *LuohuaSSO", ts.Validator)
		}
		tok, err := sso.Issue("user-7", "acme", []string{AuthorityOrdersRead}, time.Minute)
		if err != nil {
			t.Fatalf("Issue: %v", err)
		}
		auth, err := ts.Validator.Validate(context.Background(), tok)
		if err != nil || auth == nil || !auth.Authenticated {
			t.Fatalf("Validate via wired bean failed: %v %+v", err, auth)
		}
	})
}

// TestNoBeansWhenDisabled confirms luohua is inert unless configured: without
// spring.luohua.* the container wires nothing of ours.
func TestNoBeansWhenDisabled(t *testing.T) {
	gs.Web(false).RunTest(t, func(ts *struct {
		Validator security.TokenValidator `autowire:"?"`
		Messages  i18n.MessageSource      `autowire:"?"`
	}) {
		if ts.Validator != nil {
			t.Fatal("no TokenValidator expected without spring.luohua.identity")
		}
		if ts.Messages != nil {
			t.Fatal("no MessageSource expected without spring.luohua.i18n")
		}
	})
}
