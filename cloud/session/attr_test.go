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

package session

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

type testUser struct {
	ID    int64    `json:"id"`
	Name  string   `json:"name"`
	Roles []string `json:"roles"`
}

func TestGet(t *testing.T) {
	s := newSession()

	if _, ok, err := Get[string](s, "missing"); ok || err != nil {
		t.Fatalf("missing key: ok=%v err=%v", ok, err)
	}

	Set(s, "user", testUser{ID: 7, Name: "bob", Roles: []string{"admin"}})

	// Direct assertion path (Memory store keeps the original type).
	u, ok, err := Get[testUser](s, "user")
	if !ok || err != nil || u.Name != "bob" || u.ID != 7 {
		t.Fatalf("direct: ok=%v err=%v u=%+v", ok, err, u)
	}

	// JSON round-trip path: simulate what a byte backend hands back.
	var raw any
	data, _ := json.Marshal(testUser{ID: 7, Name: "bob", Roles: []string{"admin"}})
	_ = json.Unmarshal(data, &raw)
	Set[any](s, "raw", raw)
	u2, ok, err := Get[testUser](s, "raw")
	if !ok || err != nil || u2.Name != "bob" || len(u2.Roles) != 1 {
		t.Fatalf("roundtrip: ok=%v err=%v u=%+v", ok, err, u2)
	}

	// Numbers come back as float64 from JSON; Get recovers the typed value.
	f, _ := json.Marshal(42)
	_ = json.Unmarshal(f, &raw)
	Set[any](s, "count", raw)
	n, ok, err := Get[int](s, "count")
	if !ok || err != nil || n != 42 {
		t.Fatalf("int: ok=%v err=%v n=%v", ok, err, n)
	}

	// A value that cannot decode into T is an error, not a silent zero.
	Set[any](s, "user", "not-a-struct")
	if _, ok, err := Get[testUser](s, "user"); !ok || err == nil {
		t.Fatalf("decode failure should error: ok=%v err=%v", ok, err)
	}

	// Scalar values work too.
	Set(s, "deadline", time.Second)
	d, ok, err := Get[time.Duration](s, "deadline")
	if !ok || err != nil || d != time.Second {
		t.Fatalf("duration: ok=%v err=%v d=%v", ok, err, d)
	}
}

// TestGetAcrossByteStore verifies typed access survives a full
// byte-backend round trip (Save → Load), where attributes are JSON maps.
func TestGetAcrossByteStore(t *testing.T) {
	bs := &fakeByteStore{m: map[string][]byte{}}
	store := FromByteStore(bs)

	s := newSession()
	s.id = "sid-1"
	Set(s, "user", testUser{ID: 1, Name: "alice", Roles: []string{"ops"}})
	if err := store.Save(context.TODO(), s, time.Minute); err != nil {
		t.Fatal(err)
	}
	got, found, err := store.Load(context.TODO(), "sid-1")
	if !found || err != nil {
		t.Fatalf("load: found=%v err=%v", found, err)
	}
	u, ok, err := Get[testUser](got, "user")
	if !ok || err != nil || u.Name != "alice" || u.ID != 1 {
		t.Fatalf("loaded: ok=%v err=%v u=%+v", ok, err, u)
	}
}
