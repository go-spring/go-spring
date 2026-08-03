# starter-config-file Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-config-file` is a config-provider starter (`starter/DESIGN.md` §2.5)
in the integration layer: it makes local filesystem configuration a
hot-reloadable configuration source for Go-Spring. It registers two providers
that share one watch + refresh bridge:

- **`file-watch`** — one configuration document per import (a ConfigMap key
  holding `application.yaml`). Layered overrides compose via `spring.app.imports`
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
- **Refresh hook.** Container-scope controller bean `configFileController` (named
  `configFileController`, exported as `gs.Rooter`) injects
  `*gs.PropertiesRefresher` and stores it directly on the package-level
  `fileWatchController` singleton. Before wiring, `TriggerRefresh` is a safe
  no-op; after wiring, it calls `RefreshProperties`. This eliminates the
  previous `atomic.Pointer[func() error]` indirection — the IoC container's
  wiring order guarantees the controller is populated before any watcher events
  need it.
- **Watch seam.** One fsnotify watcher per directory, deduped via a
  `watched` set so repeat `Load` calls do not create duplicate watches.

## 3. Constraints

- **Single file only; a directory is an error.** Priority belongs to the
  `spring.app.imports` line order (the framework's layered storage), not to a
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
- **The bridge bean must be named.** `gs.Rooter` is `any`; the stable name
  `configFileController` avoids the `__default__` collision that would
  otherwise ambiguate with the application's own root beans.

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
