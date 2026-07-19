# starter-config-vault

[English](README.md) | [中文](README_CN.md)

`starter-config-vault` integrates [HashiCorp Vault](https://www.vaultproject.io/)
as a **remote configuration center** and **secrets store** for Go-Spring, built
on github.com/hashicorp/vault/api. Blank-importing it registers a `vault` config
provider that reads a KV secret at startup, exposes its fields as application
properties, and hot-reloads them at runtime — no restart required.

This starter covers the config-center role only. It pairs naturally with the
[property-level decryption](#property-level-decryption) seam in
`spring/conf/decrypt`: a config value can be an `ENC(...)` payload that is
decrypted before binding. A Vault Agent / CSI-mounted secret *file* is read with
`starter-config-file` instead; this starter talks to the Vault API directly.

## Installation

```bash
go get go-spring.org/starter-config-vault
```

## Quick Start

### 1. Import the package

```go
import _ "go-spring.org/starter-config-vault"
```

### 2. Import config from Vault

Declare the import in your configuration file using the provider syntax
`[optional:]vault:<host>:<port>/<mount>/<path>?<query>`:

```properties
spring.app.imports=optional:vault:127.0.0.1:8200/secret/gs-config-demo?kv-version=2
```

Query parameters:

| Key          | Default        | Description                                                        |
|--------------|----------------|--------------------------------------------------------------------|
| `kv-version` | `2`            | KV secrets engine version: `1` or `2`                              |
| `scheme`     | `http`         | `http` or `https`                                                  |
| `namespace`  | (empty)        | Vault Enterprise namespace                                         |
| `key`        | (empty)        | Read one secret field as a document instead of mapping all fields  |
| `format`     | `properties`   | Format of that field: `properties`/`yaml`/`toml`/`json`            |
| `prefix`     | (empty)        | Prefix prepended to every produced property key                    |
| `poll-ms`    | `5000`         | Polling interval for change detection, in milliseconds             |
| `token`      | (empty)        | Vault token — **discouraged**, prefer `VAULT_TOKEN` (see below)    |

**Two content modes:**

- **Whole-secret (default):** every field of the secret becomes a property.
  A secret `{ "demo.message": "hi", "db.password": "x" }` yields
  `demo.message=hi` and `db.password=x`.
- **Single-field (`?key=...`):** the named field holds a full document parsed
  with `format`. Useful for storing a `properties`/`yaml` blob in one field.

Prefix with `optional:` so the application still starts when the secret does not
exist yet; the value is filled in once it is written.

### 3. Provide the token out of band

The Vault token is a credential and must not live in a configuration file. The
provider resolves it in this order:

1. the `token` query parameter (allowed for local demos, discouraged otherwise);
2. the `VAULT_TOKEN` environment variable;
3. a token file named by the `token-file` query parameter or the
   `VAULT_TOKEN_FILE` environment variable (e.g. a Kubernetes-injected token).

If none is found, startup fails fast with a clear error.

### 4. Bind a dynamic field

Bind imported keys to a `gs.Dync[T]` field so they update live:

```go
type Demo struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
}
```

When the secret changes, the provider's polling watcher triggers an application
property refresh, and all bound `gs.Dync` fields are updated atomically. See
[example-config](example-config/example.go) for the full write → hot-reload
flow.

## Property-level decryption

Independently of Vault, Go-Spring supports Jasypt-style property decryption: any
resolved value wrapped in `ENC(...)` or prefixed with `{cipher}` is decrypted
before it is bound, so application code only sees the plaintext.

```properties
db.password=ENC(<base64-ciphertext>)
# or, Spring Cloud Config style:
db.password={cipher}<base64-ciphertext>
```

The built-in `aes` driver uses AES-GCM. The key is supplied out of band — never
in a config file:

| Variable                     | Description                                        |
|------------------------------|----------------------------------------------------|
| `GS_CONFIG_DECRYPT_KEY`      | base64-encoded AES key (16/24/32 bytes)            |
| `GS_CONFIG_DECRYPT_KEY_FILE` | path to a file holding the base64-encoded key      |
| `GS_CONFIG_DECRYPT_DRIVER`   | driver name, default `aes`                         |

A value that carries a marker but cannot be decrypted fails startup rather than
degrading to a broken default. To plug in an asymmetric scheme or a cloud KMS,
register a driver in an `init` function and select it via the env var:

```go
conf.RegisterDecryptDriver("kms", func() (decrypt.Decryptor, error) { ... })
```

This works with any config source (local files, Vault, Nacos, ...); the Vault
example ships an `ENC(...)` value end-to-end.

## How It Works

- On startup, `spring.app.imports` invokes the `vault` provider, which builds a
  client from the source string, resolves the token, reads the KV secret, and
  starts a polling watcher.
- A change to the secret is detected on the next poll, which calls the
  framework's `PropertiesRefresher`. That reloads all configuration sources
  (re-running this provider) and re-binds every `gs.Dync` field via a two-phase,
  atomic commit.
- Any bound value wrapped in `ENC(...)` / `{cipher}` is decrypted by the
  `spring/conf/decrypt` seam during binding.
