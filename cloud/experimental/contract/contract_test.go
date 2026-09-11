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

package contract_test

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"go-spring.org/cloud/experimental/contract"
	"go-spring.org/stdlib/testing/assert"
)

// contractFS embeds the fixtures so LoadFS can be exercised without touching
// the real filesystem layout.
//
//go:embed testdata/greet.contract.json
var contractFS embed.FS

// greetProvider is the real service under contract: it answers /greet with a
// JSON greeting, or 400 when the name is missing. Both branches are pinned by
// testdata/greet.contract.json, so this handler and any consumer stub built from
// the same file cannot disagree.
func greetProvider() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/greet", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		w.Header().Set("Content-Type", "application/json")
		if name == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":"name is required"}`)
			return
		}
		_, _ = fmt.Fprintf(w, `{"message":"Hello, %s!"}`, name)
	})
	return mux
}

// TestContract_BothDirections is the acceptance example: one contract file drives
// both the provider check and the consumer stub.
func TestContract_BothDirections(t *testing.T) {
	contracts, err := contract.Load("testdata/greet.contract.json")
	assert.Error(t, err).Nil()
	assert.That(t, len(contracts)).Equal(2)

	// Provider side: replay every contract against the real handler. If the
	// provider drifted (wrong status, header or body) this would fail.
	contract.VerifyHandler(t, greetProvider(), contracts)

	// Consumer side: build a stub from the same contracts and drive it with an
	// http.Client — the stand-in for a Task 01 declarative HTTP client whose
	// generated call site only holds an *http.Client.
	stub := contract.StubServer(t, contracts)
	client := stub.Client()

	// Happy path: the stub replays the greeting the provider promised.
	resp, err := client.Get(stub.URL + "/greet?name=Ada")
	assert.Error(t, err).Nil()
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	assert.That(t, resp.StatusCode).Equal(http.StatusOK)
	var greet struct {
		Message string `json:"message"`
	}
	assert.Error(t, json.Unmarshal(body, &greet)).Nil()
	assert.String(t, greet.Message).Equal("Hello, Ada!")

	// Error path: the stub also honors the 400 contract for a missing name.
	resp, err = client.Get(stub.URL + "/greet")
	assert.Error(t, err).Nil()
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	assert.That(t, resp.StatusCode).Equal(http.StatusBadRequest)
	assert.String(t, string(body)).JSONEqual(`{"error":"name is required"}`)
}

// TestVerify_DetectsProviderDrift proves Verify actually fails when the provider
// violates the contract, so a green run in the example above is meaningful.
func TestVerify_DetectsProviderDrift(t *testing.T) {
	contracts, err := contract.Load("testdata/greet.contract.json")
	assert.Error(t, err).Nil()

	// A drifted provider that always returns 200 with the wrong body.
	drifted := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"message":"nope"}`)
	})

	rec := &recordingTB{}
	contract.VerifyHandler(rec, drifted, contracts)
	assert.Number(t, rec.errors).GreaterThan(0, "Verify must report the drift")
}

// TestStub_UnmatchedRequestFailsLoudly ensures an out-of-contract call returns
// 501 rather than silently succeeding.
func TestStub_UnmatchedRequestFailsLoudly(t *testing.T) {
	contracts, err := contract.Load("testdata/greet.contract.json")
	assert.Error(t, err).Nil()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/unknown", nil)
	contract.StubHandler(contracts).ServeHTTP(rec, req)
	assert.That(t, rec.Code).Equal(http.StatusNotImplemented)
}

// recordingTB is a minimal contract.TB that counts Errorf calls, letting a test
// assert that Verify reported a failure without failing itself.
type recordingTB struct {
	errors int
}

func (r *recordingTB) Helper()                   {}
func (r *recordingTB) Cleanup(func())            {}
func (r *recordingTB) Errorf(string, ...any)     { r.errors++ }
func (r *recordingTB) Fatalf(f string, a ...any) { r.errors++ }

// ExampleStubServer shows the consumer-side flow end to end.
func ExampleStubServer() {
	contracts, _ := contract.Load("testdata/greet.contract.json")

	// StubServer needs a testing.TB; in a real test you pass t. Here we use a
	// throwaway so the example stays self-contained and runnable.
	srv := httptest.NewServer(contract.StubHandler(contracts))
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/greet?name=Ada")
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	fmt.Println(string(body))
	// Output: { "message": "Hello, Ada!" }
}

// TestContract_LoadFS covers the fs.FS entry point: the same fixtures loaded
// through an embed.FS must match what Load reads from disk.
func TestContract_LoadFS(t *testing.T) {
	fromFS, err := contract.LoadFS(contractFS, "testdata/*.json")
	assert.Error(t, err).Nil()
	assert.That(t, len(fromFS)).Equal(2)

	fromDisk, err := contract.Load("testdata/greet.contract.json")
	assert.Error(t, err).Nil()
	assert.That(t, fromFS).Equal(fromDisk)

	// A glob that matches nothing yields zero contracts, not an error.
	empty, err := contract.LoadFS(contractFS, "testdata/*.missing")
	assert.Error(t, err).Nil()
	assert.That(t, len(empty)).Equal(0)
}

// TestVerify_StringTargetURL covers the base-URL-string executor: the same
// contracts that pass against the in-process handler must also pass against a
// live server addressed by URL.
func TestVerify_StringTargetURL(t *testing.T) {
	contracts, err := contract.Load("testdata/greet.contract.json")
	assert.Error(t, err).Nil()

	srv := httptest.NewServer(greetProvider())
	defer srv.Close()

	// A recording TB proves Verify reported nothing rather than silently
	// skipping the contracts.
	rec := &recordingTB{}
	contract.VerifyURL(rec, srv.URL, contracts)
	assert.That(t, rec.errors).Equal(0)
}

// TestVerify_RawByteBody covers bodyEqual's non-JSON fallback: when the body
// is not valid JSON, comparison falls back to trimmed raw-byte equality.
func TestVerify_RawByteBody(t *testing.T) {
	plain := []contract.Contract{{
		Name:     "plain-text body",
		Request:  contract.Request{Method: http.MethodGet, Path: "/plain"},
		Response: contract.Response{Status: 200, Body: json.RawMessage("plain ok")},
	}}

	// Matching body with surrounding whitespace: equal after trimming.
	same := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "plain ok\n")
	})
	rec := &recordingTB{}
	contract.VerifyHandler(rec, same, plain)
	assert.That(t, rec.errors).Equal(0)

	// Different raw body must be reported.
	drifted := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "plain drift")
	})
	rec = &recordingTB{}
	contract.VerifyHandler(rec, drifted, plain)
	assert.Number(t, rec.errors).GreaterThan(0, "raw-byte mismatch must be reported")
}

// TestVerify_FormBody covers the form-content branch of bodyEqual: an
// application/x-www-form-urlencoded body is compared by parsed parameter set,
// so pair order and percent-encoding differences between producers don't fail
// the match, while a real value difference does.
func TestVerify_FormBody(t *testing.T) {
	formCT := contract.Values{"Content-Type": {"application/x-www-form-urlencoded"}}
	forms := []contract.Contract{{
		Name: "form body",
		Request: contract.Request{Method: http.MethodGet, Path: "/form",
			Headers: formCT, Body: json.RawMessage(`city=杭州&lang=go`)},
		Response: contract.Response{Status: 200, Headers: formCT,
			Body: json.RawMessage(`status=ok&count=2`)},
	}}

	// Same parameters, different pair order and encoding on the wire: match.
	same := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-www-form-urlencoded")
		_, _ = io.WriteString(w, "count=2&status=ok")
	})
	rec := &recordingTB{}
	contract.VerifyHandler(rec, same, forms)
	assert.That(t, rec.errors).Equal(0)

	// A different parameter value must be reported.
	drifted := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-www-form-urlencoded")
		_, _ = io.WriteString(w, "count=3&status=ok")
	})
	rec = &recordingTB{}
	contract.VerifyHandler(rec, drifted, forms)
	assert.Number(t, rec.errors).GreaterThan(0, "form value mismatch must be reported")
}

// TestStub_FormRequestBody covers the request side: a stub matches an incoming
// form body regardless of pair order or encoding on the wire.
func TestStub_FormRequestBody(t *testing.T) {
	stub := contract.StubHandler([]contract.Contract{{
		Name: "form echo",
		Request: contract.Request{Method: http.MethodPost, Path: "/submit",
			Headers: contract.Values{"Content-Type": {"application/x-www-form-urlencoded"}},
			Body:    json.RawMessage(`name=alice&city=北京`)},
		Response: contract.Response{Status: 200, Body: json.RawMessage(`"accepted"`)},
	}})

	// Percent-encoded and reordered: still the same parameter set.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/submit",
		strings.NewReader("city=%E5%8C%97%E4%BA%AC&name=alice"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	stub.ServeHTTP(rec, req)
	assert.That(t, rec.Code).Equal(http.StatusOK)

	// A different parameter set must not match (501 = no contract matched).
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/submit",
		strings.NewReader("city=%E5%8C%97%E4%BA%AC&name=bob"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	stub.ServeHTTP(rec, req)
	assert.That(t, rec.Code).Equal(http.StatusNotImplemented)
}

// TestContract_MultiValueQuery verifies the multi-value query contract: the
// request must carry exactly the listed values for a name — same set, any
// order — and a different count or an extra value breaks the match.
func TestContract_MultiValueQuery(t *testing.T) {
	cs := []contract.Contract{{
		Name: "search",
		Request: contract.Request{Method: http.MethodGet, Path: "/search",
			Query: contract.Values{"tag": {"go", "spring"}}},
		Response: contract.Response{Status: 200, Body: json.RawMessage(`"ok"`)},
	}}
	srv := contract.StubServer(t, cs)

	do := func(rawQuery string) int {
		resp, err := http.Get(srv.URL + "/search?" + rawQuery)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		return resp.StatusCode
	}

	// Any order matches; a different count or an extra value does not.
	if code := do("tag=spring&tag=go"); code != http.StatusOK {
		t.Fatalf("reordered set: status = %d, want 200", code)
	}
	if code := do("tag=go"); code != http.StatusNotImplemented {
		t.Fatalf("missing value: status = %d, want 501", code)
	}
	if code := do("tag=go&tag=spring&tag=go"); code != http.StatusNotImplemented {
		t.Fatalf("extra value: status = %d, want 501", code)
	}
	// Unlisted names are unconstrained.
	if code := do("tag=go&tag=spring&page=3"); code != http.StatusOK {
		t.Fatalf("extra unlisted name: status = %d, want 200", code)
	}

	// The verify side replays the multi-value query in the built request.
	contract.VerifyHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !reflect.DeepEqual(r.URL.Query()["tag"], []string{"go", "spring"}) {
			t.Errorf("replayed query tag = %v, want [go spring]", r.URL.Query()["tag"])
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`"ok"`))
	}), cs)
}
