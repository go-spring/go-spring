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
	"testing"

	"github.com/go-spring/stdlib/testing/assert"
)

func TestProperties(t *testing.T) {
	t.Run("NewProperties with nil data", func(t *testing.T) {
		p := NewProperties(nil)
		assert.That(t, p).NotNil()
		assert.That(t, p.data).NotNil()
		assert.That(t, len(p.data)).Equal(0)
	})

	t.Run("NewProperties with existing data", func(t *testing.T) {
		data := map[string]string{"key": "value"}
		p := NewProperties(data)
		assert.That(t, p).NotNil()
		assert.That(t, p.data).Equal(data)
	})

	t.Run("MapProperties", func(t *testing.T) {
		data := map[string]any{
			"server.host": "localhost",
			"server.port": "8080",
		}
		p := MapProperties(data)
		assert.That(t, p).NotNil()

		val, ok := p.Get("server.host")
		assert.That(t, ok).True()
		assert.That(t, val).Equal("localhost")

		val, ok = p.Get("server.port")
		assert.That(t, ok).True()
		assert.That(t, val).Equal("8080")
	})

	t.Run("Data returns underlying data", func(t *testing.T) {
		data := map[string]string{"key": "value"}
		p := NewProperties(data)
		returned := p.Data()
		assert.That(t, returned).Equal(data)
	})

	t.Run("Get existing key", func(t *testing.T) {
		p := NewProperties(map[string]string{"key": "value"})
		val, ok := p.Get("key")
		assert.That(t, ok).True()
		assert.That(t, val).Equal("value")
	})

	t.Run("Get non-existing key", func(t *testing.T) {
		p := NewProperties(map[string]string{})
		_, ok := p.Get("nonexistent")
		assert.That(t, ok).False()
	})

	t.Run("Set new key", func(t *testing.T) {
		p := NewProperties(map[string]string{})
		p.Set("newkey", "newvalue")
		val, ok := p.Get("newkey")
		assert.That(t, ok).True()
		assert.That(t, val).Equal("newvalue")
	})

	t.Run("Set overwrite existing key", func(t *testing.T) {
		p := NewProperties(map[string]string{"key": "oldvalue"})
		p.Set("key", "newvalue")
		val, ok := p.Get("key")
		assert.That(t, ok).True()
		assert.That(t, val).Equal("newvalue")
	})
}

func TestPropertiesStorage(t *testing.T) {
	t.Run("Exists with exact leaf key", func(t *testing.T) {
		p := NewProperties(map[string]string{
			"server.host": "localhost",
			"server.port": "8080",
		})
		s := NewPropertiesStorage(p)

		assert.That(t, s.Exists("server.host")).True()
		assert.That(t, s.Exists("server.port")).True()
	})

	t.Run("Exists with intermediate node", func(t *testing.T) {
		p := NewProperties(map[string]string{
			"server.host": "localhost",
			"server.port": "8080",
		})
		s := NewPropertiesStorage(p)

		assert.That(t, s.Exists("server")).True()
	})

	t.Run("Exists with non-existing key", func(t *testing.T) {
		p := NewProperties(map[string]string{
			"server.host": "localhost",
		})
		s := NewPropertiesStorage(p)

		assert.That(t, s.Exists("database")).False()
		assert.That(t, s.Exists("serverx")).False()
	})

	t.Run("Exists with partial match but wrong separator", func(t *testing.T) {
		p := NewProperties(map[string]string{
			"serverhost": "localhost",
		})
		s := NewPropertiesStorage(p)

		// "server" is a prefix of "serverhost" but not followed by '.' or '['
		assert.That(t, s.Exists("server")).False()
	})

	t.Run("Value with existing key", func(t *testing.T) {
		p := NewProperties(map[string]string{
			"server.host": "localhost",
		})
		s := NewPropertiesStorage(p)

		val, ok := s.Value("server.host")
		assert.That(t, ok).True()
		assert.That(t, val).Equal("localhost")
	})

	t.Run("Value with non-existing key", func(t *testing.T) {
		p := NewProperties(map[string]string{})
		s := NewPropertiesStorage(p)

		val, ok := s.Value("nonexistent")
		assert.That(t, ok).False()
		assert.That(t, val).Equal("")
	})

	t.Run("MapKeys collects child keys", func(t *testing.T) {
		p := NewProperties(map[string]string{
			"server.host": "localhost",
			"server.port": "8080",
			"server.name": "myserver",
		})
		s := NewPropertiesStorage(p)

		result := make(map[string]struct{})
		found := s.MapKeys("server", result)

		assert.That(t, found).True()
		assert.Map(t, result).ContainsKeys([]string{"host", "port", "name"})
	})

	t.Run("MapKeys with nested maps", func(t *testing.T) {
		p := NewProperties(map[string]string{
			"server.http.host": "localhost",
			"server.http.port": "8080",
			"server.grpc.port": "9090",
		})
		s := NewPropertiesStorage(p)

		result := make(map[string]struct{})
		found := s.MapKeys("server", result)

		assert.That(t, found).True()
		assert.Map(t, result).ContainsKeys([]string{"http", "grpc"})
	})

	t.Run("MapKeys with nested slice", func(t *testing.T) {
		p := NewProperties(map[string]string{
			"server.users[0].name": "Alice",
			"server.users[1].name": "Bob",
		})
		s := NewPropertiesStorage(p)

		result := make(map[string]struct{})
		found := s.MapKeys("server", result)

		assert.That(t, found).True()
		assert.Map(t, result).ContainsKeys([]string{"users"})
		assert.That(t, len(result)).Equal(1)
	})

	t.Run("MapKeys with empty key collects root keys", func(t *testing.T) {
		p := NewProperties(map[string]string{
			"server.host":   "localhost",
			"users[0].name": "Alice",
			"debug":         "true",
		})
		s := NewPropertiesStorage(p)

		result := make(map[string]struct{})
		found := s.MapKeys("", result)

		assert.That(t, found).True()
		assert.Map(t, result).ContainsKeys([]string{"server", "users", "debug"})
		assert.That(t, len(result)).Equal(3)
	})

	t.Run("MapKeys with non-existing prefix", func(t *testing.T) {
		p := NewProperties(map[string]string{
			"server.host": "localhost",
		})
		s := NewPropertiesStorage(p)

		result := make(map[string]struct{})
		found := s.MapKeys("database", result)

		assert.That(t, found).False()
		assert.That(t, len(result)).Equal(0)
	})

	t.Run("MapKeys with empty result map", func(t *testing.T) {
		p := NewProperties(map[string]string{
			"server.host": "localhost",
		})
		s := NewPropertiesStorage(p)

		result := make(map[string]struct{})
		s.MapKeys("server", result)

		// Verify result map is populated
		assert.Map(t, result).ContainsKeys([]string{"host"})
	})

	t.Run("SliceEntries collects all entries", func(t *testing.T) {
		p := NewProperties(map[string]string{
			"users[0].name": "Alice",
			"users[0].age":  "30",
			"users[1].name": "Bob",
			"users[1].age":  "25",
		})
		s := NewPropertiesStorage(p)

		result := make(map[string]string)
		found := s.SliceEntries("users", result)

		assert.That(t, found).True()
		assert.Map(t, result).ContainsKeys([]string{"users[0].name", "users[0].age", "users[1].name", "users[1].age"})
		assert.That(t, result["users[0].name"]).Equal("Alice")
		assert.That(t, result["users[1].name"]).Equal("Bob")
	})

	t.Run("SliceEntries with non-existing prefix", func(t *testing.T) {
		p := NewProperties(map[string]string{
			"users[0].name": "Alice",
		})
		s := NewPropertiesStorage(p)

		result := make(map[string]string)
		found := s.SliceEntries("items", result)

		assert.That(t, found).False()
		assert.That(t, len(result)).Equal(0)
	})

	t.Run("SliceEntries with partial match", func(t *testing.T) {
		p := NewProperties(map[string]string{
			"users[0].name":      "Alice",
			"superusers[0].name": "Admin",
		})
		s := NewPropertiesStorage(p)

		result := make(map[string]string)
		found := s.SliceEntries("users", result)

		assert.That(t, found).True()
		assert.Map(t, result).ContainsKeys([]string{"users[0].name"})
		assert.Map(t, result).NotContainsKeys([]string{"superusers[0].name"})
	})
}

func TestPrefixedStorage(t *testing.T) {
	t.Run("Exists with prefix", func(t *testing.T) {
		base := NewProperties(map[string]string{
			"prod.server.host": "localhost",
			"prod.server.port": "8080",
		})
		s := NewPrefixedStorage(NewPropertiesStorage(base), "prod.")

		// Should find keys with prefix added
		assert.That(t, s.Exists("server.host")).True()
		assert.That(t, s.Exists("server.port")).True()

		// Should not find keys without prefix
		// Note: "server" is a prefix of "prod.server.host", so it exists
		assert.That(t, s.Exists("server")).True()
	})

	t.Run("Value with prefix", func(t *testing.T) {
		base := NewProperties(map[string]string{
			"prod.server.host": "localhost",
		})
		s := NewPrefixedStorage(NewPropertiesStorage(base), "prod.")

		val, ok := s.Value("server.host")
		assert.That(t, ok).True()
		assert.That(t, val).Equal("localhost")

		// Without prefix should not be found
		_, ok = s.Value("host")
		assert.That(t, ok).False()
	})

	t.Run("MapKeys with prefix", func(t *testing.T) {
		base := NewProperties(map[string]string{
			"prod.server.host": "localhost",
			"prod.server.port": "8080",
		})
		s := NewPrefixedStorage(NewPropertiesStorage(base), "prod.")

		result := make(map[string]struct{})
		found := s.MapKeys("server", result)

		assert.That(t, found).True()
		assert.Map(t, result).ContainsKeys([]string{"host", "port"})
	})

	t.Run("SliceEntries with prefix", func(t *testing.T) {
		base := NewProperties(map[string]string{
			"prod.users[0].name": "Alice",
			"prod.users[0].age":  "30",
		})
		s := NewPrefixedStorage(NewPropertiesStorage(base), "prod.")

		result := make(map[string]string)
		found := s.SliceEntries("users", result)

		assert.That(t, found).True()
		assert.Map(t, result).ContainsKeys([]string{"users[0].name", "users[0].age"})
		assert.Map(t, result).NotContainsKeys([]string{"prod.users[0].name", "prod.users[0].age"})
		assert.That(t, result["users[0].name"]).Equal("Alice")
		assert.That(t, result["users[0].age"]).Equal("30")
	})

	t.Run("Empty prefix", func(t *testing.T) {
		base := NewProperties(map[string]string{
			"server.host": "localhost",
		})
		s := NewPrefixedStorage(NewPropertiesStorage(base), "")

		assert.That(t, s.Exists("server.host")).True()
		val, ok := s.Value("server.host")
		assert.That(t, ok).True()
		assert.That(t, val).Equal("localhost")
	})
}

func TestLayeredStorage(t *testing.T) {
	t.Run("AddStorage registers sources correctly", func(t *testing.T) {
		s := &LayeredStorage{}
		layer1 := NewProperties(map[string]string{
			"server.host": "localhost",
		})
		layer2 := NewProperties(map[string]string{
			"server.port": "8080",
		})

		s.AddStorage(StorageAppFile, NewPropertiesStorage(layer1), "app.properties")
		s.AddStorage(StorageProfileFile, NewPropertiesStorage(layer2), "profile.properties")

		// Verify sources are registered
		assert.That(t, s.Exists("server.host")).True()
		assert.That(t, s.Exists("server.port")).True()
	})

	t.Run("Exists with layer precedence", func(t *testing.T) {
		s := &LayeredStorage{}

		// Add lower priority layer first
		defaultLayer := NewProperties(map[string]string{
			"server.host": "default.example.com",
			"server.port": "8080",
		})
		s.AddStorage(StorageDefault, NewPropertiesStorage(defaultLayer), "default")

		// Add higher priority layer
		overrideLayer := NewProperties(map[string]string{
			"server.port": "9090",
		})
		s.AddStorage(StorageCommandLine, NewPropertiesStorage(overrideLayer), "cmdline")

		// Should exist in both layers
		assert.That(t, s.Exists("server.host")).True()
		assert.That(t, s.Exists("server.port")).True()

		// Non-existent key
		assert.That(t, s.Exists("database.host")).False()
	})

	t.Run("Value follows override semantics", func(t *testing.T) {
		s := &LayeredStorage{}

		// Lower priority
		defaultLayer := NewProperties(map[string]string{
			"server.host": "default.example.com",
			"server.port": "8080",
		})
		s.AddStorage(StorageDefault, NewPropertiesStorage(defaultLayer), "default")

		// Higher priority
		cmdlineLayer := NewProperties(map[string]string{
			"server.port": "9090",
		})
		s.AddStorage(StorageCommandLine, NewPropertiesStorage(cmdlineLayer), "cmdline")

		// Should get value from higher priority layer
		val, ok := s.Value("server.port")
		assert.That(t, ok).True()
		assert.That(t, val).Equal("9090")

		// Should get value from default layer (not overridden)
		val, ok = s.Value("server.host")
		assert.That(t, ok).True()
		assert.That(t, val).Equal("default.example.com")

		// Non-existent key
		_, ok = s.Value("database.host")
		assert.That(t, ok).False()
	})

	t.Run("MapKeys merges across all layers", func(t *testing.T) {
		s := &LayeredStorage{}

		// Layer 1
		layer1 := NewProperties(map[string]string{
			"server.host": "localhost",
			"server.port": "8080",
		})
		s.AddStorage(StorageAppFile, NewPropertiesStorage(layer1), "app.properties")

		// Layer 2
		layer2 := NewProperties(map[string]string{
			"server.name": "myserver",
			"server.ssl":  "true",
		})
		s.AddStorage(StorageEnvironment, NewPropertiesStorage(layer2), "env")

		result := make(map[string]struct{})
		found := s.MapKeys("server", result)

		assert.That(t, found).True()
		// Should merge keys from all layers
		assert.Map(t, result).ContainsKeys([]string{"host", "port", "name", "ssl"})
	})

	t.Run("MapKeys with duplicate keys across layers", func(t *testing.T) {
		s := &LayeredStorage{}

		layer1 := NewProperties(map[string]string{
			"server.host": "localhost",
		})
		s.AddStorage(StorageAppFile, NewPropertiesStorage(layer1), "app.properties")

		layer2 := NewProperties(map[string]string{
			"server.host": "example.com",
		})
		s.AddStorage(StorageEnvironment, NewPropertiesStorage(layer2), "env")

		result := make(map[string]struct{})
		found := s.MapKeys("server", result)

		assert.That(t, found).True()
		// Should only contain unique keys
		assert.Map(t, result).ContainsKeys([]string{"host"})
		assert.That(t, len(result)).Equal(1)
	})

	t.Run("SliceEntries follows override semantics", func(t *testing.T) {
		s := &LayeredStorage{}

		// Lower priority layer
		defaultLayer := NewProperties(map[string]string{
			"users[0].name": "Default User",
			"users[0].age":  "0",
			"users[1].name": "Another User",
		})
		s.AddStorage(StorageDefault, NewPropertiesStorage(defaultLayer), "default")

		// Higher priority layer
		cmdlineLayer := NewProperties(map[string]string{
			"users[0].name": "Cmdline User",
		})
		s.AddStorage(StorageCommandLine, NewPropertiesStorage(cmdlineLayer), "cmdline")

		result := make(map[string]string)
		found := s.SliceEntries("users", result)

		assert.That(t, found).True()
		// Higher priority layer overrides completely
		assert.That(t, result["users[0].name"]).Equal("Cmdline User")
		// users[1] from default layer should NOT be included
		_, hasOldKey := result["users[1].name"]
		assert.That(t, hasOldKey).False()
	})

	t.Run("SliceEntries stops at first matching layer", func(t *testing.T) {
		s := &LayeredStorage{}

		// Lower priority layer with more entries
		defaultLayer := NewProperties(map[string]string{
			"users[0].name": "Default",
			"users[1].name": "User1",
			"users[2].name": "User2",
		})
		s.AddStorage(StorageDefault, NewPropertiesStorage(defaultLayer), "default")

		// Higher priority layer with fewer entries
		cmdlineLayer := NewProperties(map[string]string{
			"users[0].name": "Override",
		})
		s.AddStorage(StorageCommandLine, NewPropertiesStorage(cmdlineLayer), "cmdline")

		result := make(map[string]string)
		found := s.SliceEntries("users", result)

		assert.That(t, found).True()
		// Should only have entries from cmdline layer
		assert.That(t, len(result)).Equal(1)
		assert.That(t, result["users[0].name"]).Equal("Override")
	})

	t.Run("SliceEntries stops at higher priority scalar value", func(t *testing.T) {
		s := &LayeredStorage{}

		defaultLayer := NewProperties(map[string]string{
			"users[0]": "Default",
			"users[1]": "User1",
		})
		s.AddStorage(StorageDefault, NewPropertiesStorage(defaultLayer), "default")

		cmdlineLayer := NewProperties(map[string]string{
			"users": "Override1,Override2",
		})
		s.AddStorage(StorageCommandLine, NewPropertiesStorage(cmdlineLayer), "cmdline")

		result := make(map[string]string)
		found := s.SliceEntries("users", result)

		assert.That(t, found).False()
		assert.That(t, len(result)).Equal(0)
		val, ok := s.Value("users")
		assert.That(t, ok).True()
		assert.That(t, val).Equal("Override1,Override2")
	})

	t.Run("multiple sources in same layer", func(t *testing.T) {
		s := &LayeredStorage{}

		// First source in layer
		source1 := NewProperties(map[string]string{
			"server.host": "first.example.com",
			"server.port": "8080",
		})
		s.AddStorage(StorageAppFile, NewPropertiesStorage(source1), "first.properties")

		// Second source in same layer (should have higher priority for same keys)
		source2 := NewProperties(map[string]string{
			"server.host": "second.example.com",
			"server.name": "myserver",
		})
		s.AddStorage(StorageAppFile, NewPropertiesStorage(source2), "second.properties")

		// Later source wins within same layer for duplicate keys
		val, ok := s.Value("server.host")
		assert.That(t, ok).True()
		assert.That(t, val).Equal("second.example.com")

		// Key only in first source should still be accessible
		val, ok = s.Value("server.port")
		assert.That(t, ok).True()
		assert.That(t, val).Equal("8080")

		// Key only in second source
		val, ok = s.Value("server.name")
		assert.That(t, ok).True()
		assert.That(t, val).Equal("myserver")
	})

	t.Run("empty layer", func(t *testing.T) {
		s := &LayeredStorage{}

		emptyLayer := NewProperties(map[string]string{})
		s.AddStorage(StorageDefault, NewPropertiesStorage(emptyLayer), "empty")

		assert.That(t, s.Exists("nonexistent")).False()
		_, ok := s.Value("nonexistent")
		assert.That(t, ok).False()
	})
}

func TestStorageConstants(t *testing.T) {
	t.Run("storage layer constants are defined", func(t *testing.T) {
		// Verify constants are defined and in correct order
		assert.That(t, StorageCommandLine).Equal(0)
		assert.That(t, StorageEnvironment).Equal(1)
		assert.That(t, StorageProfileFile).Equal(2)
		assert.That(t, StorageAppFile).Equal(3)
		assert.That(t, StorageDefault).Equal(4)
		assert.That(t, StorageMax).Equal(5)
	})

	t.Run("layer precedence example", func(t *testing.T) {
		s := &LayeredStorage{}

		// Add sources in different layers
		defaultLayer := NewProperties(map[string]string{
			"app.name": "DefaultApp",
			"app.port": "8080",
		})
		s.AddStorage(StorageDefault, NewPropertiesStorage(defaultLayer), "defaults")

		appFileLayer := NewProperties(map[string]string{
			"app.name": "MyApp",
		})
		s.AddStorage(StorageAppFile, NewPropertiesStorage(appFileLayer), "application.properties")

		envLayer := NewProperties(map[string]string{
			"app.port": "9090",
		})
		s.AddStorage(StorageEnvironment, NewPropertiesStorage(envLayer), "ENV")

		cmdlineLayer := NewProperties(map[string]string{
			"app.debug": "true",
		})
		s.AddStorage(StorageCommandLine, NewPropertiesStorage(cmdlineLayer), "cmdline")

		// Verify precedence
		val, ok := s.Value("app.name")
		assert.That(t, ok).True()
		assert.That(t, val).Equal("MyApp") // From AppFile (higher than Default)

		val, ok = s.Value("app.port")
		assert.That(t, ok).True()
		assert.That(t, val).Equal("9090") // From Environment (higher than Default)

		val, ok = s.Value("app.debug")
		assert.That(t, ok).True()
		assert.That(t, val).Equal("true") // From CommandLine
	})
}

func TestConfigSource(t *testing.T) {
	t.Run("ConfigSource wraps PropertiesStorage with name", func(t *testing.T) {
		props := NewProperties(map[string]string{
			"server.host": "localhost",
		})
		storage := NewPropertiesStorage(props)
		source := ConfigSource{
			PropertiesStorage: storage,
			Name:              "test.properties",
		}

		assert.That(t, source.Name).Equal("test.properties")
		assert.That(t, source.Exists("server.host")).True()
		val, ok := source.Value("server.host")
		assert.That(t, ok).True()
		assert.That(t, val).Equal("localhost")
	})
}

func TestStorageIntegration(t *testing.T) {
	t.Run("chained PrefixedStorage and LayeredStorage", func(t *testing.T) {
		// Create layered storage
		s := &LayeredStorage{}

		// Add production config (without prefix in this test)
		prodData := NewProperties(map[string]string{
			"server.host": "prod.example.com",
			"server.port": "443",
		})
		s.AddStorage(StorageProfileFile, NewPropertiesStorage(prodData), "prod.properties")

		// Add dev layer
		devData := NewProperties(map[string]string{
			"server.host": "localhost",
			"server.port": "8080",
		})
		s.AddStorage(StorageDefault, NewPropertiesStorage(devData), "dev.properties")

		// Should get prod values (higher priority due to layer order)
		val, ok := s.Value("server.host")
		assert.That(t, ok).True()
		assert.That(t, val).Equal("prod.example.com")

		val, ok = s.Value("server.port")
		assert.That(t, ok).True()
		assert.That(t, val).Equal("443")
	})

	t.Run("MapProperties integration", func(t *testing.T) {
		data := map[string]any{
			"server": map[string]any{
				"host": "localhost",
				"port": 8080,
			},
			"database": map[string]any{
				"host": "db.example.com",
				"port": 5432,
			},
		}

		p := MapProperties(data)
		s := NewPropertiesStorage(p)

		// Test flattened access
		assert.That(t, s.Exists("server")).True()
		assert.That(t, s.Exists("database")).True()

		val, ok := s.Value("server.host")
		assert.That(t, ok).True()
		assert.That(t, val).Equal("localhost")

		val, ok = s.Value("server.port")
		assert.That(t, ok).True()
		assert.That(t, val).Equal("8080")

		// Test map key collection
		result := make(map[string]struct{})
		found := s.MapKeys("server", result)
		assert.That(t, found).True()
		assert.Map(t, result).ContainsKeys([]string{"host", "port"})
	})
}
