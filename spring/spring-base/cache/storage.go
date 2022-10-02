/*
 * Copyright 2012-2019 the original author or authors.
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

package cache

import "sync"

// HashFunc returns the hash value of the key.
type HashFunc func(key string) int

var (
	SimpleHash HashFunc = func(key string) int {
		const max = 20
		n := len(key)
		if n > max {
			n = max
		}
		c := 0
		for i := 0; i < n; i++ {
			c += int(key[i])
		}
		return c
	}
)

// Storage is a Shardable cache implementation.
type Storage struct {
	n int
	h HashFunc
	m []*sync.Map
}

// NewStorage returns a new *Storage.
func NewStorage(size int, hash HashFunc) *Storage {
	s := &Storage{n: size, h: hash}
	s.Reset()
	return s
}

// Reset resets the Storage.
func (s *Storage) Reset() {
	s.m = make([]*sync.Map, s.n, s.n)
	for i := 0; i < s.n; i++ {
		s.m[i] = &sync.Map{}
	}
}

// Sharding returns the shard of the key.
func (s *Storage) Sharding(key string) *sync.Map {
	return s.m[s.h(key)%s.n]
}
