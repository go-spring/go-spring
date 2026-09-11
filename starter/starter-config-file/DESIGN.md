# starter-config-file Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-config-file` is a config-provider starter (`starter/DESIGN.md` §2.5)
in the integration layer: it makes local filesystem configuration a
hot-reloadable configuration source for Go-Spring. It registers two providers
that share one watch + refresh bridge:

- **`file-watch`** — one configuration document per import (a ConfigMap key
  holding `application.yaml`). Layered overrides compose via `spring.config.import`
  order; the provider never merges a directory.
- **`configtree`** — a directory of scalar key files (a Secret / env-style
  ConfigMap mount). Each leaf file maps to one property keyed by its dotted
  relative path, valued by its unparsed content.

## 1. Responsibilities & Boundaries

- Registers the `file-watch` and `configtree` provider names via
  `conf.RegisterProvider` in `init()` and nothing else at the package top level
  — no exported bean for users, no server (the internal controller bean is
  plumbing, registered by the shared init in starter.go).
- `file-watch`: takes `file-watch:<file>`, rejects a directory, reads the single
  file (format detected by extension via the shared conf reader registry), and
  returns the flattened result.
- `configtree`: parses `configtree:<dir>`, rejects a file, walks the tree, and
  returns one entry per non-dot leaf file (path → key, content → value).
- Both install fsnotify watchers (via the shared controller) on the relevant
  directories; every observed event fires the application-wide property refresh.
- Explicitly does **not** talk to a remote configuration center. Those are
  separate starters (`starter-config-{nacos,etcd,consul,vault,k8s}`).

## 2. Key Abstractions & Seams

- **Provider seam.** `conf.RegisterProvider` is called twice in `init()` —
  once for `file-watch` (→ `Load`) and once for `configtree`
  (→ `LoadConfigTree`) — both bound to methods of the same controller singleton.
  The providers run during `AppConfig.Refresh`, before any bean exists.
- **Refresh hook.** The controller is not a bean: on a change the fsnotify
  callback calls the process-level `gs.RefreshProperties()` facade directly
  (mounted by the app from its most recent `Start`, meant for out-of-container
  watch goroutines that cannot use dependency injection). Before the app has
  started the facade returns an error, so `TriggerRefresh` is a safe no-op.
  This eliminates the earlier bridge-bean + autowired `*gs.PropertiesRefresher`
  indirection.
- **Watch seam.** One fsnotify watcher per directory, deduped via a
  `watched` set so repeat `Load` calls do not create duplicate watches.

## 3. Constraints

- **Single file only; a directory is an error.** Priority belongs to the
  `spring.config.import` line order (the framework's layered storage), not to a
  directory's contents — merging arbitrary files into one key set has no
  well-defined precedence, so file-watch refuses to do it. Layered overrides
  are expressed as one file-watch import per file in priority order.
- **Watch the parent directory, never the file.** The kubelet updates a
  ConfigMap/Secret mount by writing a fresh timestamped directory and
  atomically renaming the `..data` symlink; common editors save by atomic
  rename too. In both cases the file's inode changes, so a per-file inotify
  watch would be left on a stale inode after the first update. The watcher
  therefore registers on `filepath.Dir(path)` (file-watch), or on every
  directory in the tree (configtree), all of which stay stable across the swap.
- **configtree: path is the key, content is the value; no merge, no priority.**
  Each non-dot leaf contributes exactly one property keyed by its dotted
  relative path. Because paths are unique, two leaves can never collide, so
  there is no intra-source precedence to define — the priority problem that
  killed the old directory-merge mode is eliminated by construction. Values are
  raw trimmed strings (no format parsing); `?format=` is not accepted.
- **`optional:` only tolerates a missing file.** Once the file exists,
  parsing and reading errors are always fatal so a mistyped file surfaces
  immediately.
- **The controller has no bean identity.** An earlier version registered a
  bridge bean exported as `gs.Rooter` (an `any` alias, where the stable name
  `configFileController` avoided a `__default__` collision with the
  application's own root beans). It is now a plain singleton registered as a
  provider in `init()` — no bean, no autowired fields.

## 4. Trade-offs / Alternatives Rejected

- **Polling — rejected.** fsnotify observes the ConfigMap symlink swap
  immediately; the extra CPU cost of a poll loop is not needed.
- **A per-provider format map / `?format=` override — rejected.** The shared
  `conf/reader` registry already maps extensions to readers; file-watch delegates
  to `reader.ReadFile` instead of re-collecting that mapping. A `?format=` query
  to force a reader by name was dropped: file-watch targets one concrete file, so
  requiring a recognizable extension is reasonable and keeps the provider
  query-free and dependency-light (no per-provider reader imports).
- **Directory merge — rejected.** A directory of scalar key files (path →
  key, content → value, no parse) is a distinct model with no overlap and no
  precedence problem; it belongs to a separate configtree-style provider, not
  to file-watch. Mixing the two would force an ill-defined intra-directory
  priority onto the otherwise priority-clean layered model.
