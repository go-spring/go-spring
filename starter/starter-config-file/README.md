# starter-config-file

[English](README.md) | [中文](README_CN.md)

`starter-config-file` integrates **local filesystem configuration** as
**hot-reloadable configuration sources** for Go-Spring, built on
github.com/fsnotify/fsnotify. One blank-import registers **two** providers that
cover the two distinct shapes of a Kubernetes ConfigMap/Secret mount:

- **`file-watch`** — a single configuration document (a ConfigMap key holding
  `application.yaml`). For layered overrides, declare one `file-watch` import
  per file in priority order; later imports win, the same rule
  `spring.app.imports` uses for every other source.
- **`configtree`** — a directory of scalar key files (a Secret / env-style
  ConfigMap mount). Each file's path becomes a dotted property key and its
  unparsed content the value; paths are unique, so there is no priority problem.

Both watch the parent directory (never the file itself), so the kubelet's atomic
`..data` symlink swap on a ConfigMap/Secret update is detected and turned into a
live property refresh without a restart.

This starter covers local file/volume watching only. Remote configuration
centers (Nacos, etcd, Consul) are separate starters.

## Installation

```bash
go get go-spring.org/starter-config-file
```

## Quick Start

### 1. Import the package

```go
import _ "go-spring.org/starter-config-file"
```

### 2. Import config from a single file

Declare the import in your configuration file using the provider syntax
`[optional:]file-watch:<file>`:

```properties
# A single mounted ConfigMap/Secret key file (recommended for K8s):
spring.app.imports=file-watch:/etc/config/application.yaml

# Layered overrides: later imports win.
spring.app.imports=file-watch:/etc/app/application.yaml
spring.app.imports=file-watch:/etc/app/application-prod.yaml
```

The path must be a **single file** (a directory is rejected — use the
`configtree` provider for directories). It is parsed by extension
(`.properties`, `.yaml`/`.yml`, `.toml`/`.tml`, `.json`) through the shared conf
reader registry. The watcher always registers on the file's parent directory, so
the K8s `..data` symlink swap on a ConfigMap update is picked up correctly.
Prefix with `optional:` so the application still starts when the file does not
exist yet.

### 3. Bind a dynamic field

Bind imported keys to a `gs.Dync[T]` field so they update live:

```go
type Demo struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
}
```

When a watched file changes, the provider's watcher triggers an application
property refresh, and all bound `gs.Dync` fields are updated atomically. See
[example](example/example.go) for the full flow — it reproduces the exact
Kubernetes `..data` atomic symlink swap and asserts the bound field hot-reloads.

## Kubernetes example

```yaml
volumeMounts:
  - name: config
    mountPath: /etc/config
volumes:
  - name: config
    configMap:
      name: my-app-config
```

```properties
# Point at the specific key file that holds your config document.
spring.app.imports=file-watch:/etc/config/application.yaml
```

`kubectl edit configmap my-app-config` (or a new rollout) updates the volume;
bound `gs.Dync` fields refresh within seconds, without restarting the pod.

## configtree — directory of scalar keys

Use `configtree` when each file is **one scalar value** rather than a whole
config document — the shape of a Kubernetes Secret or env-style ConfigMap mount.
The provider walks the tree; each leaf file becomes one property whose key is
its dotted relative path and whose value is its trimmed raw content (not parsed).

```properties
# A Secret mount: db.user, db.password, server.port (one value per file)
spring.app.imports=configtree:/etc/secret
```

```
/etc/secret/
  db.user          -> "alice"        # property db.user=alice
  db.password      -> "s3cr3t"       # property db.password=s3cr3t
  server.port      -> "8080"         # property server.port=8080
```

K8s Secret/ConfigMap mounts are flat (one file per key; dots in key names are
allowed, so `db.user` is a valid key). `configtree` also supports genuinely
nested trees (`db/user` → property `db.user`); entries whose name starts with `.`
(`..data`, timestamped dirs) are skipped at every level. No `?format=` is
accepted — values are raw strings. See
[example-configtree](example-configtree/example.go) for the full hot-reload flow.

### file-watch vs configtree

| Shape | Provider | Priority |
|---|---|---|
| One whole config document | `file-watch:<file>` | multiple files compose via `spring.app.imports` order |
| Many scalar key files | `configtree:<dir>` | none needed — paths are unique, keys never collide |

## How It Works

- On startup, `spring.app.imports` invokes the `file-watch` / `configtree`
  provider, which reads the source and starts a watcher on its **parent
  directory** (every directory in the tree, for `configtree`).
- Kubernetes updates a mounted ConfigMap/Secret by writing a fresh timestamped
  data directory and atomically renaming the `..data` symlink onto it. The key
  file you import is a symlink resolved through `..data`, so its inode changes on
  every update — which is why the watch is on the parent directory (stable)
  rather than the file itself (whose inode is swapped on each update).
- A change fires the watcher, which calls the framework's
  `PropertiesRefresher`. That reloads all configuration sources (re-running this
  provider) and re-binds every `gs.Dync` field via a two-phase, atomic commit.