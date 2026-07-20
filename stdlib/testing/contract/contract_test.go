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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-spring.org/stdlib/testing/assert"
	"go-spring.org/stdlib/testing/contract"
)

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
	contract.Verify(t, greetProvider(), contracts)

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
	contract.Verify(rec, drifted, contracts)
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
func (r *recordingTB) Skipf(string, ...any)      {}

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
