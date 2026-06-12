/*
 * Copyright 2024 The Go-Spring Authors.
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

/*
Package conf provides a flexible configuration binding system for Go applications.

It enables declarative configuration binding from various file formats and providers
into Go structs with automatic type conversion, placeholder resolution, and
expression-based validation.

# Core Concepts

conf works by:
 1. Loading configuration data from a source via providers
 2. Flattening nested data into a key-value store
 3. Binding properties to struct fields using tagged annotations
 4. Resolving variable placeholders recursively
 5. Validating values using expressions

# Tag Syntax

Struct fields use the `value` tag to specify the configuration key:

	value:"${key:=default}"

Where:
  - `${key}` - references the configuration key
  - `:=default` - optional default value if the key is not found

Features:
  - Nested keys: `${db.host}`, `${service.endpoint}`
  - Default values: `${DB_HOST:=localhost}`
  - Chained defaults: `${A:=${B:=default}}` - if A is missing, try B, then fall back to default
  - Nested references: `${prefix${suffix}}` - nested variable expansion
  - Root binding: `${ROOT}` - binds from the root level (used at the top-level struct)

Example:

	type ServerConfig struct {
	    Host string `value:"${host:=localhost}"`
	    Port int    `value:"${port:=8080}"`
	}

# Supported Types

The following types are supported out of the box:

 1. **Primitives**: string, all integer/float types, bool (automatic string conversion)
 2. **Time types**: time.Time, time.Duration (built-in converters)
 3. **Structs**: recursive binding of nested struct fields
 4. **Slices**: two input formats:
    - Indexed properties: `endpoints[0]=a`, `endpoints[1]=b`
    - Comma-separated: `endpoints=a,b,c`
 5. **Maps**: key-value binding from dot-notation subkeys: `users.alice=Alice`, `users.bob=Bob`
 6. **Custom types**: register custom converters with RegisterConverter

# Value Resolution

All property values undergo variable resolution before binding. Any `${key}`
occurrences in the string are replaced with their resolved values from the
configuration store. Resolution is recursive and handles nested placeholders.

Example:

	host=localhost
	port=8080
	url=http://${host}:${port}/api

After resolution, `url` becomes `http://localhost:8080/api`

# Validation

Add validation to any field using the `expr` tag. The expression has access to:
  - `$` - the current field's value
  - All registered custom validation functions

Examples:

	type Config struct {
	    Port int    `value:"${port}" expr:"$ > 0 && $ < 65536"`
	    Email string `value:"${email}" expr:"contains($, '@')"`
	    MinLen string `value:"${data}" expr:"len($) > 3"`
	}

Register custom validation functions:

	// Register a validator that checks a time is in the future
	conf.RegisterValidateFunc("future", func(t time.Time) bool {
	    return t.After(time.Now())
	})

	// Use it in validation:
	type Event struct {
	    StartTime time.Time `value:"${start-time}" expr:"future($)"`
	}

For more expression syntax see: https://github.com/expr-lang/expr

# Loading Configuration

conf.Load accepts a source URI in the format:

	[optional:]<provider>:<location>

Examples:

	// Load from YAML file (auto-detects format by extension)
	props, err := conf.Load("config.yaml")

	// Explicit file provider
	props, err = conf.Load("file:config.yaml")

	// Optional file - does not error if file not found
	props, err = conf.Load("optional:file:config.yaml")

Register custom providers (e.g., etcd, Consul, environment variables) with
RegisterProvider to support additional configuration sources.

# Supported File Formats

Built-in readers are registered for these extensions:

  - JSON (.json)
  - Java Properties (.properties)
  - YAML (.yaml, .yml)
  - TOML (.toml, .tml)

Register custom readers with RegisterReader for additional formats.

# Quick Start

	package main

	import (
	    "fmt"
	    "log"

	    "github.com/go-spring/spring-core/conf"
	)

	type Config struct {
	    Host string `value:"${host:=localhost}"`
	    Port int    `value:"${port:=8080}"`
	}

	func main() {
	    // Load configuration from file
	    props, err := conf.Load("config.yaml")
	    if err != nil {
	        log.Fatal(err)
	    }

	    // Bind to struct (uses ${ROOT} by default - binds all keys from root)
	    var cfg Config
	    if err := conf.Bind(props, &cfg); err != nil {
	        log.Fatal(err)
	    }

	    // Use cfg.Host and cfg.Port
	    fmt.Printf("Server starting on %s:%d\n", cfg.Host, cfg.Port)
	}

Bind can also bind only under a specific key prefix:

	var cfg AppConfig
	conf.Bind(props, &cfg, "${app}") // all fields look for keys under app.*

# Extension Points

All extensions must be registered during init():

  - RegisterProvider - add new configuration source providers
  - RegisterReader - add support for additional file formats
  - RegisterConverter - add type conversion for custom types
  - RegisterValidateFunc - add custom validation functions
*/
package conf
