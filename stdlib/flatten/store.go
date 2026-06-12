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

package flatten

import (
	"strings"
)

// Storage defines the minimal abstraction required by the bind system.
//
// Data is stored in flattened form, for example:
//
//	server.port=8080
//	server.host=localhost
//	users[0].name=tom
//
// The implementation assumes the input data is already valid.
// Storage itself does not validate structural correctness.
//
// The interface provides three capabilities used during binding:
//
//   - leaf value lookup
//   - map key discovery
//   - slice entry discovery
//
// Exists is mainly intended for property condition checks rather than binding.
type Storage interface {

	// Exists reports whether a key exists.
	//
	// A key is considered existing if:
	//   - it exists as an exact leaf key
	//   - it is a prefix of other keys
	//
	// Example:
	//
	//	server.port=8080
	//
	//	Exists("server")      -> true
	//	Exists("server.port") -> true
	//	Exists("server.host") -> false
	//
	// This method is typically used by property condition logic.
	Exists(key string) bool

	// Value returns the value of a leaf node.
	//
	// Only exact key matches are returned.
	Value(key string) (string, bool)

	// MapKeys collects the direct child keys of a map node.
	//
	// Example:
	//
	//	key = "server"
	//	data:
	//	    server.host
	//	    server.port
	//
	// result:
	//
	//	    {"host", "port"}
	//
	// Only the first map level is returned.
	MapKeys(key string, result map[string]struct{}) bool

	// SliceEntries collects all flattened entries belonging to a slice node.
	//
	// Example:
	//
	//	key = "users"
	//	data:
	//	    users[0].name
	//	    users[1].name
	//
	// result will contain all matching entries.
	SliceEntries(key string, result map[string]string) bool
}

// Properties represents a flattened key-value storage.
type Properties struct {
	data map[string]string
}

// NewProperties creates a new Properties instance.
func NewProperties(data map[string]string) *Properties {
	if data == nil {
		data = make(map[string]string)
	}
	return &Properties{data: data}
}

// MapProperties creates a new Properties instance from a
// hierarchical map by flattening it into key-value pairs.
func MapProperties(data map[string]any) *Properties {
	return NewProperties(Flatten(data))
}

// Data returns the underlying flattened data.
func (s *Properties) Data() map[string]string {
	return s.data
}

// Get retrieves the value of a leaf node.
func (s *Properties) Get(key string) (string, bool) {
	v, ok := s.data[key]
	return v, ok
}

// Set sets the value of a leaf node.
func (s *Properties) Set(key, val string) {
	s.data[key] = val
}

// PropertiesStorage adapts Properties to the Storage interface.
type PropertiesStorage struct {
	*Properties
}

// NewPropertiesStorage creates a new PropertiesStorage instance.
func NewPropertiesStorage(s *Properties) *PropertiesStorage {
	return &PropertiesStorage{Properties: s}
}

// Exists reports whether the key exists.
//
// A key is considered existing if:
//   - it exists as an exact leaf key
//   - it is a prefix of other keys (intermediate node)
func (s *PropertiesStorage) Exists(key string) bool {
	if _, ok := s.data[key]; ok {
		return true
	}
	for k := range s.data {
		str, ok := strings.CutPrefix(k, key)
		if !ok || str == "" {
			continue
		}
		if str[0] == '.' || str[0] == '[' {
			return true
		}
	}
	return false
}

// Value retrieves the value of a leaf node.
func (s *PropertiesStorage) Value(key string) (string, bool) {
	val, ok := s.data[key]
	return val, ok
}

// MapKeys collects child keys of a map node.
func (s *PropertiesStorage) MapKeys(key string, result map[string]struct{}) bool {
	var found bool
	for k := range s.data {
		var str string
		if key == "" {
			str = k
		} else {
			var ok bool
			str, ok = strings.CutPrefix(k, key)
			if !ok || str == "" || str[0] != '.' {
				continue
			}
			if str = str[1:]; str == "" {
				continue
			}
		}
		if i := strings.IndexAny(str, ".["); i > 0 {
			result[str[:i]] = struct{}{}
			found = true
		} else if i < 0 {
			result[str] = struct{}{}
			found = true
		}
	}
	return found
}

// SliceEntries collects all entries belonging to a slice node.
//
// The implementation only checks for the presence of key[index].
// It does not enforce index continuity.
func (s *PropertiesStorage) SliceEntries(key string, result map[string]string) bool {
	var found bool
	for k, v := range s.data {
		str, ok := strings.CutPrefix(k, key)
		if !ok || str == "" || str[0] != '[' {
			continue
		}
		result[k] = v
		found = true
	}
	return found
}

// PrefixedStorage wraps another Storage and automatically
// prepends a fixed prefix to all keys.
type PrefixedStorage struct {
	Storage
	Prefix string
}

// NewPrefixedStorage creates a new PrefixedStorage instance.
func NewPrefixedStorage(s Storage, prefix string) *PrefixedStorage {
	return &PrefixedStorage{
		Storage: s,
		Prefix:  prefix,
	}
}

// Exists checks existence with the configured prefix.
func (s *PrefixedStorage) Exists(key string) bool {
	return s.Storage.Exists(s.Prefix + key)
}

// Value retrieves a value with the configured prefix.
func (s *PrefixedStorage) Value(key string) (string, bool) {
	return s.Storage.Value(s.Prefix + key)
}

// MapKeys retrieves map keys with the configured prefix.
func (s *PrefixedStorage) MapKeys(key string, result map[string]struct{}) bool {
	return s.Storage.MapKeys(s.Prefix+key, result)
}

// SliceEntries retrieves slice entries with the configured prefix.
func (s *PrefixedStorage) SliceEntries(key string, result map[string]string) bool {
	m := make(map[string]string)
	if !s.Storage.SliceEntries(s.Prefix+key, m) {
		return false
	}
	for k, v := range m {
		if str, ok := strings.CutPrefix(k, s.Prefix); ok {
			result[str] = v
		}
	}
	return true
}

const (
	// StorageCommandLine represents configuration provided via command line.
	// This usually has the highest priority.
	StorageCommandLine = iota

	// StorageEnvironment represents configuration from environment variables.
	StorageEnvironment

	// StorageProfileFile represents configuration loaded from profile-specific files.
	// Example: application-dev.properties.
	StorageProfileFile

	// StorageAppFile represents configuration from the main application file.
	// Example: application.properties or application.yml.
	StorageAppFile

	// StorageDefault represents built-in default configuration values.
	// This layer typically has the lowest priority.
	StorageDefault

	// StorageMax is the number of supported layers.
	StorageMax
)

type ConfigSource struct {
	*PropertiesStorage
	Name string
}

// LayeredStorage aggregates multiple configuration sources with
// deterministic precedence rules.
//
// The design follows the layered configuration model used by
// Spring-style environments: configuration values may come from
// multiple sources (command line, environment variables, files, etc.),
// and a predictable priority order determines which value wins.
//
// Precedence rules:
//
//  1. Layers with a lower index have higher priority.
//  2. Within the same layer, sources added later override earlier ones.
//
// For example:
//
//	CommandLine
//	Environment
//	ProfileFile
//	AppFile
//	Default
//
// Lookup always scans layers from highest priority to lowest.
//
// Different data structures behave differently across layers:
//
//   - Leaf values follow **override semantics**.
//     The first value found wins.
//
//   - Map properties follow **merge semantics**.
//     Keys from all layers are combined.
//
//   - Slice properties follow **override semantics**.
//     The first layer defining the slice replaces all lower layers.
type LayeredStorage struct {
	// layers groups configuration sources by priority level.
	//
	// Each index represents a logical configuration layer.
	// The slice allows multiple sources inside the same layer.
	layers [StorageMax][]ConfigSource
}

// AddStorage registers a configuration source into the specified layer.
//
// Sources within the same layer follow an override rule:
// the most recently added source has higher priority.
//
// This is implemented by inserting the new source at the
// beginning of the slice so that iteration always sees
// newer sources first.
func (s *LayeredStorage) AddStorage(index int, source *PropertiesStorage, name string) {
	s.layers[index] = append([]ConfigSource{{
		PropertiesStorage: source,
		Name:              name,
	}}, s.layers[index]...)
}

// Exists reports whether the given key exists in any layer.
//
// Lookup follows layer priority: once a higher-priority source
// reports the key exists, lower layers are not checked.
//
// Exists considers both leaf nodes and intermediate prefixes,
// depending on the underlying Storage implementation.
func (s *LayeredStorage) Exists(key string) bool {
	for _, arr := range s.layers {
		for _, source := range arr {
			if source.Exists(key) {
				return true
			}
		}
	}
	return false
}

// Value retrieves the leaf value for a key.
//
// The lookup follows override semantics across layers:
// the first matching value found in the highest-priority
// source is returned.
func (s *LayeredStorage) Value(key string) (string, bool) {
	for _, arr := range s.layers {
		for _, source := range arr {
			if v, ok := source.Value(key); ok {
				return v, true
			}
		}
	}
	return "", false
}

// MapKeys collects the child keys of a map node across all layers.
//
// Unlike leaf values, map structures are merged across sources.
// This means keys defined in different layers are combined
// into a single logical map.
//
// Example:
//
//	source1:
//	    server.port=8080
//
//	source2:
//	    server.host=localhost
//
// Final map:
//
//	server.port
//	server.host
//
// If the same key appears in multiple layers, the actual
// value resolution still follows the normal override rule
// in Value().
func (s *LayeredStorage) MapKeys(key string, result map[string]struct{}) bool {
	var found bool
	for _, arr := range s.layers {
		for _, source := range arr {
			if source.MapKeys(key, result) {
				found = true
			}
		}
	}
	return found
}

// SliceEntries collects slice entries for the specified key.
//
// Lists follow an override rule across configuration layers.
// Once a higher-priority source defines a slice, lower layers
// are ignored entirely.
//
// Example:
//
//	source1:
//	    my.list[0]=a
//	    my.list[1]=b
//
//	source2:
//	    my.list[0]=c
//
// Result:
//
//	[c]
//
// Not:
//
//	[c,b]
//
// Therefore the search stops as soon as a source containing
// slice entries is found.
func (s *LayeredStorage) SliceEntries(key string, result map[string]string) bool {
	for _, arr := range s.layers {
		for _, source := range arr {
			if _, ok := source.Value(key); ok {
				return false
			}
			if source.SliceEntries(key, result) {
				return true
			}
		}
	}
	return false
}
