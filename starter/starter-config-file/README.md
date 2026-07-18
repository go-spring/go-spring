# starter-config-file

[English](README.md) | [中文](README_CN.md)

`starter-config-file` integrates a mounted directory (or a single file) as a
**hot-reloadable configuration source** for Go-Spring, built on
github.com/fsnotify/fsnotify. Blank-importing it registers a `file-watch`
config provider that loads application configuration from the path at startup
and hot-reloads it whenever the files change — no restart required.

Its primary purpose is **Kubernetes**: a `ConfigMap` or `Secret` mounted as a
volume becomes hot-reloadable without any custom code. The kubelet updates such
a volume by atomically swapping the `..data` symlink, which this provider's
directory watcher detects and turns into a live property refresh.

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

### 2. Import config from a mounted path

Declare the import in your configuration file using the provider syntax
`[optional:]file-watch:<path>[?format=..]`:

```properties
# Watch a mounted ConfigMap/Secret directory (recommended for K8s):
spring.app.imports=file-watch:/etc/config
```

The path may be a **directory** (every recognized file in it is read and
merged) or a **single file**. In both cases the *directory* is watched, so the
K8s `..data` symlink swap on a ConfigMap update is picked up correctly.

Query parameters:

| Key      | Default            | Description                                                        |
|----------|--------------------|--------------------------------------------------------------------|
| `format` | by file extension  | Force a format for all files: `properties`/`yaml`/`toml`/`json`. Use this when ConfigMap keys have no extension. |

By default files are parsed by extension (`.properties`, `.yaml`/`.yml`,
`.toml`/`.tml`, `.json`); files with an unknown extension are skipped. Prefix
with `optional:` so the application still starts when the path does not exist
yet.

### 3. Bind a dynamic field

Bind imported keys to a `gs.Dync[T]` field so they update live:

```go
type Demo struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
}
```

When a watched file changes, the provider's watcher triggers an application
property refresh, and all bound `gs.Dync` fields are updated atomically. See
[example-config](example-config/example.go) for the full flow — it reproduces
the exact Kubernetes `..data` atomic symlink swap and asserts the bound field
hot-reloads.

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
spring.app.imports=file-watch:/etc/config
```

`kubectl edit configmap my-app-config` (or a new rollout) updates the volume;
bound `gs.Dync` fields refresh within seconds, without restarting the pod.

## How It Works

- On startup, `spring.app.imports` invokes the `file-watch` provider, which
  reads the mounted path, parses each file, and starts a **directory** watcher.
- Kubernetes updates a mounted ConfigMap/Secret by writing a fresh timestamped
  data directory and atomically renaming the `..data` symlink onto it. Watching
  the directory (not the individual files) is what makes this observable —
  entries whose name begins with `.` (`..data`, the timestamped dirs) are
  skipped, while the real key symlinks are read through `..data`.
- A change fires the watcher, which calls the framework's
  `PropertiesRefresher`. That reloads all configuration sources (re-running this
  provider) and re-binds every `gs.Dync` field via a two-phase, atomic commit.
