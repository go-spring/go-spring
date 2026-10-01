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

package StarterSecurityJWT

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go-spring.org/cloud/security"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/testing/assert"
)

// TestExportsValidatorSeam proves the wiring: with spring.security.jwt.instances
// armed, the authenticator is exported as security.TokenValidator under its
// config sub-key name, so an application injects the seam — and composes it with
// a server family's Authenticate middleware — without naming a concrete type.
// The concrete *Authenticator stays injectable for callers that need Wrap.
func TestExportsValidatorSeam(t *testing.T) {
	const secret = "topsecret"
	gs.Web(false).Configure(func(app gs.App) {
		app.Property("spring.security.jwt.instances.api.secret", secret)
		app.Property("spring.security.jwt.instances.api.issuer", "iss-a")
	}).RunTest(t, func(ts *struct {
		Validator security.TokenValidator `autowire:"api"`
		Concrete  *Authenticator          `autowire:"api"`
	}) {
		if ts.Validator == nil {
			t.Fatal(`expected a security.TokenValidator bean named "api"`)
		}
		if _, ok := ts.Validator.(*Authenticator); !ok {
			t.Fatalf("validator is %T, want *Authenticator", ts.Validator)
		}
		if ts.Concrete != ts.Validator {
			t.Fatal("the seam and the concrete type must resolve to the same bean")
		}

		token := signHS(t, secret, jwt.MapClaims{
			"sub": "user-1",
			"iss": "iss-a",
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		auth, err := ts.Validator.Validate(t.Context(), token)
		assert.Error(t, err).Nil()
		assert.String(t, auth.Principal.Subject).Equal("user-1")
	})
}

// TestInertWithoutConfig locks the enable switch: importing the starter with no
// "${spring.security.jwt.instances}" entry registers no validator at all.
func TestInertWithoutConfig(t *testing.T) {
	gs.Web(false).RunTest(t, func(ts *struct {
		Validator security.TokenValidator `autowire:"?"`
	}) {
		if ts.Validator != nil {
			t.Fatalf("no configuration must register nothing, got %T", ts.Validator)
		}
	})
}
