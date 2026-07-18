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

package i18n

import (
	"context"
	"errors"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

func TestLocaleContext(t *testing.T) {
	ctx := context.Background()
	assert.String(t, LocaleFrom(ctx)).Equal("")
	ctx = WithLocale(ctx, "zh")
	assert.String(t, LocaleFrom(ctx)).Equal("zh")
}

func newSource() *MapSource {
	return NewMapSource("en").
		AddMap("en", map[string]string{
			"validation.email": "{0} is not a valid email",
			"greeting":         "hello, {0}",
		}).
		AddMap("zh", map[string]string{
			"validation.email": "{0} 不是合法邮箱",
		})
}

func TestMessageLocaleHit(t *testing.T) {
	src := newSource()
	ctx := WithLocale(context.Background(), "zh")
	got, err := src.Message(ctx, "validation.email", "Email")
	assert.Error(t, err).Nil()
	assert.String(t, got).Equal("Email 不是合法邮箱")
}

func TestMessageFallsBackToDefaultLocale(t *testing.T) {
	src := newSource()
	// "greeting" exists only in en; a zh request falls back to the default locale.
	ctx := WithLocale(context.Background(), "zh")
	got, err := src.Message(ctx, "greeting", "bob")
	assert.Error(t, err).Nil()
	assert.String(t, got).Equal("hello, bob")
}

func TestMessageMissingKeyReturnsKeyAndError(t *testing.T) {
	src := newSource()
	ctx := WithLocale(context.Background(), "en")
	got, err := src.Message(ctx, "nope.key")
	assert.String(t, got).Equal("nope.key")
	assert.Error(t, err).NotNil()
	assert.That(t, errors.Is(err, ErrMessageNotFound)).True()
}

func TestMessageNoLocaleUsesDefault(t *testing.T) {
	src := newSource()
	got, err := src.Message(context.Background(), "greeting", "x")
	assert.Error(t, err).Nil()
	assert.String(t, got).Equal("hello, x")
}

func TestInterpolateLeavesUnmatchedPlaceholders(t *testing.T) {
	src := NewMapSource("en").Add("en", "k", "{0} and {5}")
	got, err := src.Message(WithLocale(context.Background(), "en"), "k", "first")
	assert.Error(t, err).Nil()
	assert.String(t, got).Equal("first and {5}")
}

func TestAddParsedFlattensNestedMaps(t *testing.T) {
	src := NewMapSource("en")
	src.AddParsed("en", map[string]any{
		"validation": map[string]any{
			"email": "bad email",
			"min":   "too small",
		},
		"count": 3,
	})
	ctx := WithLocale(context.Background(), "en")
	got, err := src.Message(ctx, "validation.min")
	assert.Error(t, err).Nil()
	assert.String(t, got).Equal("too small")

	got, err = src.Message(ctx, "count")
	assert.Error(t, err).Nil()
	assert.String(t, got).Equal("3")
}

func TestLocalizerSwallowsMissingKey(t *testing.T) {
	src := newSource()
	loc := Localizer(src, WithLocale(context.Background(), "en"))
	assert.String(t, loc("greeting", "y")).Equal("hello, y")
	assert.String(t, loc("missing")).Equal("") // error swallowed -> empty
}
