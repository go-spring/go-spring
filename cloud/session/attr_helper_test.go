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
	"time"
)

// fakeByteStore is an in-memory ByteStore used to exercise typed attribute
// access across a FromByteStore round trip.
type fakeByteStore struct{ m map[string][]byte }

func (s *fakeByteStore) Get(_ context.Context, id string) ([]byte, bool, error) {
	data, ok := s.m[id]
	return data, ok, nil
}

func (s *fakeByteStore) Set(_ context.Context, id string, data []byte, _ /* ttl */ time.Duration) error {
	s.m[id] = data
	return nil
}

func (s *fakeByteStore) Delete(_ context.Context, id string) error {
	delete(s.m, id)
	return nil
}
