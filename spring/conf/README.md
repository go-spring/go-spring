# conf

[English](README.md) | [中文](README_CN.md)

`conf` provides a flexible configuration binding system for Go applications.
It enables declarative configuration binding from various file formats and
providers into Go structs with automatic type conversion, placeholder
resolution, and expression-based validation.

## Core Concepts

`conf` works by:

1. Loading configuration data from a source via providers
2. Flattening nested data into a key-value store
3. Binding properties to struct fields using tagged annotations
4. Resolving variable placeholders recursively
5. Validating values using expressions

## Tag Syntax

Struct fields use the `value` tag to specify the configuration key:

```go
value:"${key:=default}"
```

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

```go
type ServerConfig struct {
    Host string `value:"${host:=localhost}"`
    Port int    `value:"${port:=8080}"`
}
```

## Supported Types

The following types are supported out of the box:

1. **Primitives**: string, all integer/float types, bool (automatic string conversion)
2. **Time types**: `time.Time`, `time.Duration` (built-in converters); `conf.ByteSize`
   for human-readable byte counts (`"1.5GB"`), see below
3. **Structs**: recursive binding of nested struct fields
4. **Slices**: two input formats:
   - Indexed properties: `endpoints[0]=a`, `endpoints[1]=b`
   - Comma-separated: `endpoints=a,b,c`
5. **Maps**: key-value binding from dot-notation subkeys: `users.alice=Alice`, `users.bob=Bob`
6. **Custom types**: register custom converters with `RegisterConverter`

### ByteSize

`conf.ByteSize` is a byte count parsed from a human-readable config string such
as `"1KB"`, `"1.5MB"`, or `"1024"` - the byte-domain counterpart of
`time.Duration`. It binds straight from properties:

```go
var cfg struct {
    MaxMemory conf.ByteSize `value:"${max-memory:=1MB}"`
}
// cfg.MaxMemory.Bytes() == 1 << 20
```

Suffixes are binary (powers of 1024), matching JVM (`-Xmx`), nginx and Kafka
conventions. Accepted suffixes (case-insensitive, optional whitespace between
number and suffix): `B`; `K`/`KB`/`KiB`; `M`/`MB`/`MiB`; `G`/`GB`/`GiB`;
`T`/`TB`/`TiB`. An empty suffix means bytes. A fractional value like `"1.5GB"`
is allowed and rounded to the nearest byte. Decimal (power-of-1000) semantics
are deliberately not supported to avoid the KB-vs-KiB ambiguity in config
files.

## Value Resolution

All property values undergo variable resolution before binding. Any `${key}`
occurrences in the string are replaced with their resolved values from the
configuration store. Resolution is recursive and handles nested placeholders.

```properties
host=localhost
port=8080
url=http://${host}:${port}/api
```

After resolution, `url` becomes `http://localhost:8080/api`.

## Property-Level Decryption

A property value wrapped in a decryption marker is decrypted right before it is
bound, so application code only sees the plaintext:

```properties
db.password=ENC(aes:<base64-ciphertext>)
# or, Spring Cloud Config style:
db.password={cipher}aes:<base64-ciphertext>
```

The marker must name its driver - so several decryption schemes can coexist in
one configuration file. The built-in `aes` driver (AES-GCM) reads its key out
of band, never from a config file:

| Variable                       | Description                                    |
|--------------------------------|------------------------------------------------|
| `GS_CONFIG_DECRYPT_AES_KEY`     | base64-encoded AES key (16/24/32 bytes)        |
| `GS_CONFIG_DECRYPT_AES_KEY_FILE`| path to a file holding the base64-encoded key  |

To plug in an asymmetric scheme or a cloud KMS, register a driver in an `init`
function and name it in the marker. A value that carries a marker but cannot
be decrypted fails startup rather than degrading to a broken default. See the
`decrypt` and `decrypt/aes` packages for details.

## Validation

Add validation to any field using the `expr` tag. The expression has access to:

- `$` - the current field's value
- All registered custom validation functions

Examples:

```go
type Config struct {
    Port  int    `value:"${port}" expr:"$ > 0 && $ < 65536"`
    Email string `value:"${email}" expr:"contains($, '@')"`
    Data  string `value:"${data}" expr:"len($) > 3"`
}
```

Register custom validation functions:

```go
// Register a validator that checks a time is in the future
conf.RegisterValidateFunc("future", func(t time.Time) (bool, error) {
    return t.After(time.Now()), nil
})

// Use it in validation:
type Event struct {
    StartTime time.Time `value:"${start-time}" expr:"future($)"`
}
```

For more expression syntax see [expr-lang/expr](https://github.com/expr-lang/expr).

### Cross-Field Validation

Per-field expr tags cannot express relationships between fields. For
constraints that span multiple fields, implement the `Validator` interface on
your configuration struct. `Validate` is called automatically after all struct
fields are bound, including for nested structs inside slices and maps.

```go
type ServerConfig struct {
    Host string `value:"${host}"`
    Port int    `value:"${port:=0}"`
}

func (c *ServerConfig) Validate() error {
    if c.Port > 0 && c.Host == "" {
        return fmt.Errorf("host required when port is set")
    }
    if c.Port < 0 || c.Port > 65535 {
        return fmt.Errorf("port %d out of range [0, 65535]", c.Port)
    }
    return nil
}
```

Validation is hierarchical: each nested struct can carry its own cross-field
rules, and they are validated independently from the bottom up. This composes
naturally with expr tags - a struct can use both per-field and cross-field
validation at the same time.

Comparison with constructor validation:

|                          | conf.Validator     | constructor body   |
|--------------------------|--------------------|--------------------|
| conf.Bind path           | ✓ (automatic)      | ✗ (not called)     |
| gs.TagArg path           | ✓ (automatic)      | ✓ (manual)         |
| external deps            | ✗ (no injection)   | ✓ (can inject beans) |
| consistent across binding entrypoints | ✓       | depends on caller  |

Use `Validator` when the constraint is intrinsic to the type itself; use
constructor validation when the check needs access to other beans or external
state.

## Loading Configuration

`conf.Load` accepts a source URI in the format:

```
[optional:]<provider>:<location>
```

Examples:

```go
// Load from YAML file (auto-detects format by extension)
props, err := conf.Load("config.yaml")

// Explicit file provider
props, err = conf.Load("file:config.yaml")

// Optional file - does not error if file not found
props, err = conf.Load("optional:file:config.yaml")
```

Register custom providers (e.g., etcd, Consul, environment variables) with
`RegisterProvider` to support additional configuration sources. A provider is a
lifecycle object: besides `Load` it implements `Close(ctx)`, which the runtime
calls once at shutdown — with the shutdown context, which carries trace values
but is not cancelled — to stop the watchers and listeners `Load` installed.

### Supported File Formats

Built-in readers are registered for these extensions:

- JSON (`.json`)
- Java Properties (`.properties`, `.props`)
- YAML (`.yaml`, `.yml`)
- TOML (`.toml`, `.tml`)

Register custom readers with `RegisterReader` for additional formats.

## Quick Start

```go
package main

import (
    "fmt"
    "log"

    "go-spring.org/spring/conf"
    "go-spring.org/stdlib/flatten"
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
    if err := conf.Bind(flatten.NewPropertiesStorage(props), &cfg); err != nil {
        log.Fatal(err)
    }

    // Use cfg.Host and cfg.Port
    fmt.Printf("Server starting on %s:%d\n", cfg.Host, cfg.Port)
}
```

`Bind` can also bind only under a specific key prefix:

```go
var cfg AppConfig
conf.Bind(flatten.NewPropertiesStorage(props), &cfg, "${app}") // all fields look for keys under app.*
```

## Extension Points

All extensions must be registered during `init()`:

- `RegisterProvider` - add new configuration source providers
- `RegisterReader` - add support for additional file formats
- `RegisterConverter` - add type conversion for custom types
- `RegisterValidateFunc` - add custom validation functions
- `RegisterDecryptor` - add property-level decryption schemes

## Runtime Refresh and Lifecycle

- **The only hot-update primitive is `gs.Dync[T]`.** It re-binds a field wholesale
  (coarse, per-field), with no diff and no per-key callback. `gs.OnProperty` is a
  startup-time condition, **not** a runtime listener — to react to a key change,
  poll a `Dync` yourself or build a watcher.
- **The refresh entry is the process-level facade** `gs.RefreshProperties(ctx)` /
  `gs.AppStarted()`; the old `gs.PropertiesRefresher` bean is gone — do not
  reintroduce a Refresher injection. A new config provider calls the facade
  directly; a pre-start call returns an error the caller warns and drops.
- **A config provider is a lifecycle object.** Besides `Load` it implements
  `Close` (stop watchers/listeners/clients and clear caches). `Close` is
  idempotent, and the next `Load` must be able to **re-arm** (generate a fresh
  generation context, empty the `clients`/`listened` maps, ...). Register the
  controller object itself, **never** the `ctrl.Load` method value (that drops
  the receiver); wrap a pure-function source in `provider.ProviderFunc` (whose
  `Close` is a no-op).
- **`CloseAll` is called once by the application runtime at shutdown** (the
  graceful path, before the container closes; `gs.Run` / `gs.RunTest` carry a
  `defer` fallback covering a failed startup). The registry does not own the
  lifecycle and is **not** emptied after `Close` — the same process may run
  several instances in sequence (e.g. back-to-back `gs.RunTest`); a directory is
  not a lifecycle owner.
- **Family config is two buckets; a family that owns its child namespace keeps
  its own shape.** A client/resource family holds exactly
  `spring.<family>.default.*` (family-wide, each overridable per instance) and
  `spring.<family>.instances.<name>.*` (one instance; `<name>` is the bean name)
  — no other direct child. The invariant: never let a user-chosen name share a
  level with a framework key. `default` holds only overridable defaults;
  non-overridable policy gets its own namespace (`spring.governance.*`). A family
  that adds a framework level inside `instances`
  (`spring.lock.instances.<backend>.<name>`) keeps one `default` at the family
  prefix. A single-instance family (`spring.http.server`) keeps its keys directly
  under the prefix.
