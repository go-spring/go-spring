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

package security

import (
	"context"
	"errors"
	"testing"

	"go-spring.org/stdlib/aspect"
	"go-spring.org/stdlib/testing/assert"
)

func TestAuthentication_Authorities(t *testing.T) {
	auth := &Authentication{Authenticated: true, Authorities: []string{"orders:read", "ROLE_ADMIN"}}

	assert.That(t, auth.HasAuthority("orders:read")).True()
	assert.That(t, auth.HasAuthority("orders:write")).False()
	assert.That(t, auth.HasAnyAuthority("orders:write", "ROLE_ADMIN")).True()
	assert.That(t, auth.HasAnyAuthority("orders:write")).False()
	assert.That(t, auth.HasAnyAuthority()).True() // authenticated, no requirement
	assert.That(t, auth.HasAllAuthorities("orders:read", "ROLE_ADMIN")).True()
	assert.That(t, auth.HasAllAuthorities("orders:read", "orders:write")).False()
}

func TestAuthentication_NilAndAnonymous(t *testing.T) {
	var nilAuth *Authentication
	assert.That(t, nilAuth.HasAuthority("x")).False()
	assert.That(t, nilAuth.HasAnyAuthority()).False()
	assert.That(t, nilAuth.HasAllAuthorities()).False()

	anon := &Authentication{Authenticated: false, Authorities: []string{"x"}}
	assert.That(t, anon.HasAuthority("x")).False()
	assert.That(t, anon.HasAnyAuthority()).False()
}

func TestContext_RoundTrip(t *testing.T) {
	ctx := context.Background()
	_, ok := FromContext(ctx)
	assert.That(t, ok).False()

	auth := &Authentication{Principal: Principal{Subject: "u1"}, Authenticated: true}
	ctx = WithAuthentication(ctx, auth)
	got, ok := FromContext(ctx)
	assert.That(t, ok).True()
	assert.That(t, got).Same(auth)
	assert.String(t, got.Principal.Subject).Equal("u1")
}

// stubValidator is a trivial TokenValidator used to exercise the registry.
type stubValidator struct{ subject string }

func (s stubValidator) Validate(_ context.Context, token string) (*Authentication, error) {
	if token == "" {
		return nil, ErrUnauthenticated
	}
	return &Authentication{Principal: Principal{Subject: s.subject}, Token: token, Authenticated: true}, nil
}

func TestRegistry(t *testing.T) {
	RegisterValidator("stub-a", stubValidator{subject: "a"})

	v, ok := GetValidator("stub-a")
	assert.That(t, ok).True()
	got, err := v.Validate(context.Background(), "tok")
	assert.Error(t, err).Nil()
	assert.String(t, got.Principal.Subject).Equal("a")

	_, err = MustGetValidator("missing")
	assert.Error(t, err).Matches("no validator registered as \"missing\"")

	assert.Panic(t, func() { RegisterValidator("stub-a", stubValidator{}) }, "already registered")
	assert.Panic(t, func() { RegisterValidator("", stubValidator{}) }, "empty name")
	assert.Panic(t, func() { RegisterValidator("nil-v", nil) }, "nil validator")
}

func TestRequire_Interceptor(t *testing.T) {
	target := func(context.Context) (any, error) { return "ok", nil }

	tests := []struct {
		name    string
		ctx     context.Context
		require []string
		want    any
		wantErr error
	}{
		{
			name:    "unauthenticated",
			ctx:     context.Background(),
			require: []string{"orders:read"},
			wantErr: ErrUnauthenticated,
		},
		{
			name:    "forbidden",
			ctx:     WithAuthentication(context.Background(), &Authentication{Authenticated: true, Authorities: []string{"orders:read"}}),
			require: []string{"orders:write"},
			wantErr: ErrForbidden,
		},
		{
			name:    "allowed",
			ctx:     WithAuthentication(context.Background(), &Authentication{Authenticated: true, Authorities: []string{"orders:write"}}),
			require: []string{"orders:write"},
			want:    "ok",
		},
		{
			name:    "authenticated-no-requirement",
			ctx:     WithAuthentication(context.Background(), &Authentication{Authenticated: true}),
			require: nil,
			want:    "ok",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := aspect.NewChain(Require(tt.require...))
			got, err := chain.Run(tt.ctx, "Target", target)
			if tt.wantErr != nil {
				assert.That(t, errors.Is(err, tt.wantErr)).True()
				return
			}
			assert.Error(t, err).Nil()
			assert.That(t, got).Equal(tt.want)
		})
	}
}
