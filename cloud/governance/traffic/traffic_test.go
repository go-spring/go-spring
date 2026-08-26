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

package traffic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarkerRoundTrip(t *testing.T) {
	ctx := context.Background()
	assert.False(t, IsLoadTest(ctx))
	assert.Equal(t, "", Source(ctx))

	tagged := WithLoadTest(ctx, "loadtest.Run")
	assert.True(t, IsLoadTest(tagged))
	assert.Equal(t, "loadtest.Run", Source(tagged))

	// tagging again keeps the original source
	again := WithLoadTest(tagged, "other")
	assert.Equal(t, "loadtest.Run", Source(again))
}

func TestMarkerWithoutSource(t *testing.T) {
	ctx := WithLoadTest(context.Background())
	assert.True(t, IsLoadTest(ctx))
	assert.Equal(t, "", Source(ctx))
}

func TestWithLoadTestNilContext(t *testing.T) {
	var nilCtx context.Context
	ctx := WithLoadTest(nilCtx, "x")
	require.NotNil(t, ctx)
	assert.True(t, IsLoadTest(ctx))
}

func TestPropagate(t *testing.T) {
	parent := WithLoadTest(context.Background(), "http-header")

	// child inherits marker + source
	child := Propagate(parent, context.Background())
	assert.True(t, IsLoadTest(child))
	assert.Equal(t, "http-header", Source(child))

	// nil parent => child unchanged (still not a load-test ctx)
	var nilCtx context.Context
	plain := context.Background()
	out2 := Propagate(nilCtx, plain)
	assert.False(t, IsLoadTest(out2))

	// non-load-test parent => child loses the marker it never had
	assert.False(t, IsLoadTest(Propagate(context.Background(), context.Background())))
}

func TestPropagateNilChild(t *testing.T) {
	parent := WithLoadTest(context.Background(), "grpc-metadata")
	out := Propagate(parent, nil)
	require.NotNil(t, out)
	assert.True(t, IsLoadTest(out))
}

func TestCarrierHasAcceptsTruthyValues(t *testing.T) {
	c := Carrier{}
	assert.False(t, c.Has("k"))

	for _, v := range []string{"1", "true", "on", "TRUE", "On"} {
		c["k"] = []string{v}
		assert.True(t, c.Has("k"), "value %q should be truthy", v)
	}
	for _, v := range []string{"0", "false", "", "no"} {
		c["k"] = []string{v}
		assert.False(t, c.Has("k"), "value %q should not be truthy", v)
	}
}

func TestCarrierSetIsIdempotent(t *testing.T) {
	c := Carrier{}
	c.Set("k")
	c.Set("k")
	assert.Len(t, c["k"], 1)
}

func TestCarrierNilSafe(t *testing.T) {
	var c Carrier
	assert.NotPanics(t, func() {
		c.Set("k")
		_ = c.Has("k")
	})
	assert.False(t, c.Has("k"))
}

func TestExtractInjectHTTP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderLoadTest, "1")
	ctx := ExtractHTTP(context.Background(), req)
	assert.True(t, IsLoadTest(ctx))
	assert.Equal(t, "http-header", Source(ctx))

	plain := httptest.NewRequest(http.MethodGet, "/", nil)
	assert.False(t, IsLoadTest(ExtractHTTP(context.Background(), plain)))

	outReq := httptest.NewRequest(http.MethodGet, "/", nil)
	InjectHTTP(ctx, outReq)
	assert.Equal(t, "1", outReq.Header.Get(HeaderLoadTest))

	outPlain := httptest.NewRequest(http.MethodGet, "/", nil)
	InjectHTTP(context.Background(), outPlain)
	assert.Equal(t, "", outPlain.Header.Get(HeaderLoadTest))
}

func TestExtractInjectCarrier(t *testing.T) {
	md := Carrier{MetaKeyLoadTest: []string{"1"}}
	ctx := ExtractCarrier(context.Background(), md, MetaKeyLoadTest, "grpc-metadata")
	assert.True(t, IsLoadTest(ctx))
	assert.Equal(t, "grpc-metadata", Source(ctx))

	assert.False(t, IsLoadTest(ExtractCarrier(context.Background(), Carrier{}, MetaKeyLoadTest, "grpc-metadata")))

	out := Carrier{}
	InjectCarrier(ctx, out, MetaKeyLoadTest)
	assert.Equal(t, []string{"1"}, out[MetaKeyLoadTest])

	out2 := Carrier{}
	InjectCarrier(context.Background(), out2, MetaKeyLoadTest)
	assert.Nil(t, out2[MetaKeyLoadTest])
}

func TestExtractHTTPHeaderCaseInsensitive(t *testing.T) {
	// http.Header.Get canonicalises, so any case spelling matches.
	for _, spelling := range []string{"x-loadtest", "X-LOADTEST", "X-LoadTest"} {
		req := &http.Request{Header: http.Header{}}
		req.Header.Set(spelling, "1")
		ctx := ExtractHTTP(context.Background(), req)
		assert.True(t, IsLoadTest(ctx), "spelling %q should match", spelling)
	}
}
