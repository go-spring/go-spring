# starter-config-file Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-config-file` is a config-provider starter (`starter/DESIGN.md` §2.5)
in the integration layer: it makes a local file or a mounted directory a
hot-reloadable configuration source for Go-Spring. Its primary purpose is to
turn a Kubernetes ConfigMap or Secret mount into a live configuration source
with no custom code.

## 1. Responsibilities & Boundaries

- Registers a `file-watch` provider name via `conf.RegisterProvider` in
  `init()` and nothing else at the package top level — no injectable bean,
  no server.
- Parses the provider source `file-watch:<path>[?format=..]`, reads the file
  or every eligible file in the directory, and merges the flattened result.
- Installs an fsnotify directory watcher; every observed event fires the
  application-wide property refresh.
- Explicitly does **not** talk to a remote configuration center. Those are
  separate starters (`starter-config-{nacos,etcd,consul,vault,k8s}`).

## 2. Key Abstractions & Seams

- **Provider seam.** `conf.RegisterProvider("file-watch", loadWatchedConfig)`.
  The provider runs during `AppConfig.Refresh`, before any bean exists.
- **Refresh hook.** Container-scope bridge bean `configRefreshBridge` (named
  `configFileRefreshBridge`, exported as `gs.Rooter`) injects
  `*gs.PropertiesRefresher` and stores its `RefreshProperties` into an
  `atomic.Pointer[func() error]`.
- **Watch seam.** One fsnotify watcher per directory, deduped via a
  `watched` set so repeat `Load` calls do not create duplicate watches.

## 3. Constraints

- **Watch the directory, never the file.** The kubelet updates a ConfigMap
  or Secret mount by writing a fresh timestamped directory and atomically
  renaming the `..data` symlink. A per-file watcher would be left pointing at
  a stale inode after the first update. When the source path is a file the
  watcher registers on `filepath.Dir(path)`.
- **Skip dotted entries when reading a directory.** `..data` and the
  timestamped temp directories used by the projected-volume mechanism start
  with `.` and must not be treated as config files. Real config keys are
  symlinks whose targets live inside `..data`; they read transparently
  through the directory listing.
- **Unknown extensions are silently skipped in directory mode.** A ConfigMap
  routinely contains keys the app does not intend to bind (e.g.
  `README.md`), so a `NOT_A_KNOWN_FORMAT` file is skipped rather than
  failing the load. A forced `format=` overrides this and applies to every
  read entry — used when ConfigMap keys have no extension.
- **`optional:` only tolerates a missing path.** Once the path exists,
  parsing and reading errors are always fatal so a mistyped format surfaces
  immediately.
- **The bridge bean must be named.** `gs.Rooter` is `any`; the stable name
  `configFileRefreshBridge` avoids the `__default__` collision that would
  otherwise ambiguate with the application's own root beans.

## 4. Trade-offs / Alternatives Rejected

- **Polling — rejected.** fsnotify observes the ConfigMap symlink swap
  immediately; the extra CPU cost of a poll loop is not needed.
- **Native `net/url` for the source — rejected.** File paths can legally
  contain `?` on some filesystems; the provider parses only a trailing
  `?...` as query string (via a single `strings.Cut`) to keep the parser
  small and dependency-free.
- **Recursive directory watch — deliberately omitted.** ConfigMap mounts
  are flat by design; a recursive walk would only complicate exclusion of
  the projected-volume bookkeeping directories.
