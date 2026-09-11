# starter-config-file Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `filewatch.go`, `configtree.go`), the gs core
(`spring/conf/provider/provider.go`, `spring/gs/internal/gs_conf/conf.go`,
`spring/gs/internal/gs_app/app.go`) and the smoke-tested examples
[example/](example/) and [example-configtree/](example-configtree/) (both `check.sh` green).

**Activation**: one blank import registers **two** configuration providers on a shared
watch+refresh bridge. Each is activated individually by its entry in `spring.config.import`:

- `file-watch:<file>` — one configuration document per import (a ConfigMap key holding
  `application.yaml`), parsed by extension (filewatch.go:50).
- `configtree:<dir>` — a directory tree of scalar key files (a Secret / env-style ConfigMap
  mount); each leaf file is one property keyed by its dotted relative path (configtree.go:43).

The centerpiece is **hot reload without restart**: both providers watch the parent directory,
so the kubelet's atomic `..data` symlink swap on a ConfigMap/Secret update becomes a live
`gs.Dync` refresh. Remote config centers (Nacos/etcd/Consul) are separate starters.

---

## 1. Complete worked project

A service that reads its database credentials from a Secret-style mount (`configtree`) and its
application settings from a ConfigMap document (`file-watch`), with both bound to `gs.Dync`
fields for hot reload. File tree:

```
demo/
├── go.mod
├── main.go
├── config.go
├── conf/
│   └── app.properties
└── mount/                  # laid out like a K8s projected volume
    ├── application.yaml -> ..data/application.yaml
    ├── db.user           -> ..data/db.user
    ├── db.password       -> ..data/db.password
    └── ..data            -> ..2025_...   (symlink, swapped atomically on update)
```

**go.mod** (module deps that matter):

```
require (
    go-spring.org/spring            v1.3.x
    go-spring.org/starter-config-file latest
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-config-file"
)

func main() { gs.Run() }
```

**config.go** — bind the mounted values to dynamic fields:

```go
package main

import "go-spring.org/spring/gs"

// DbConfig reads a Secret-style configtree mount. Each key file is one
// property; values are RAW strings (never parsed as yaml/properties).
// Dync fields hot-reload on watch events; plain fields would be startup-only.
type DbConfig struct {
    User     gs.Dync[string] `value:"${db.user:=none}"`
    Password gs.Dync[string] `value:"${db.password:=}"`
}

// AppSettings reads one ConfigMap key document imported via file-watch,
// parsed by extension (.yaml here -> nested keys flatten to dotted keys).
type AppSettings struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
    Port    gs.Dync[string] `value:"${server.port:=none}"`
}

func init() {
    // Export as gs.Rooter so the container creates these beans eagerly
    // (unexported Provide beans without injection sites are pruned in prod).
    gs.Provide(&DbConfig{}).Export(gs.As[gs.Rooter]())
    gs.Provide(&AppSettings{}).Export(gs.As[gs.Rooter]())
}
```

**conf/app.properties** — the complete, commented surface:

```properties
# Two imports, comma-separated; later imports override earlier ones
# (same layering rule as every spring.config.import source).
spring.config.import=file-watch:./mount/application.yaml,optional:configtree:./mount

# Optional prefix: start even if the Secret mount is absent on a laptop.
# This example only exercises the config providers, so no HTTP server:
spring.http.server.enabled=false
```

Lay down the initial mount before first start (or let your deployment's kubelet do it):

```bash
mkdir -p mount
printf 'demo:\n  message: initial\nserver:\n  port: "8080"\n' > mount/application.yaml
printf 'alice'   > mount/db.user
printf 's3cr3t'  > mount/db.password
```

**Verify** (cold load, then the hot-reload drill — isomorphic to the examples' `check.sh`,
which performs exactly this flow and exits non-zero on failure):

```bash
cd example && ./check.sh                    # file-watch provider, automated
cd ../example-configtree && ./check.sh      # configtree provider, automated

# or drive it manually:
go run ./example -manual                    # stays up
echo 'demo:\n  message: flipped' > example/mount/application.yaml
# within ~1s the gs.Dync field reports "flipped" — no restart
```

---

## 2. Assembly & timing

### 2.1 Where imports resolve — pre-bean, and why that matters

```
import starter-config-file
  ├─ init: conf.RegisterProvider("file-watch", controller.Load)      (filewatch.go:50)
  └─ init: conf.RegisterProvider("configtree", controller.LoadConfigTree)  (configtree.go:43)
        │
gs.Run() → App.Start()                                                (app.go:285)
  1. mount the gs.RefreshProperties / gs.AppStarted facade targets
  2. app.p.Refresh(): load conf/app.* files, resolve spring.config.import
     → provider.Load runs HERE: reads the file/tree, ensureWatch() installs
       the fsnotify watcher on the parent directory / every tree directory
  3. initLog, then IoC container wiring
  4. app.started = true → RefreshProperties becomes legal               (app.go:309)
  5. Runners, Servers, readiness
  └─ on any watched-directory event: watchLoop → TriggerRefresh
       → gs.RefreshProperties(): reload ALL sources, merge by
         priority, update every gs.Dync field atomically               (app.go:247)
```

Why the controller needs no bean although config loads pre-bean: the watch callback
reaches the refresh through the process-level `gs.RefreshProperties()` facade, which
returns an error before the app has started, so `TriggerRefresh` is deliberately a
no-op pre-start — early watch events are dropped harmlessly because the startup load
just captured the state. This is why both providers hang off ONE controller singleton.

Other timing facts:

- Imports are resolved inside the config-load step (gs_conf/conf.go `loadFileImports`), i.e.
  **before** beans bind — a required import that fails aborts startup before any bean code runs.
- Only **one level** of imports is processed: an `spring.config.import` declared inside an
  imported file is silently ignored (gs_conf/conf.go:210-215).
- The import list is deduplicated and comma-separated (`Imports []string value:"${spring.config.import:=}"`,
  gs_conf/conf.go:218).
- Watchers are deduplicated per directory (`watched` map, starter.go:86-106): the startup load
  and every refresh re-call `Load`/`LoadConfigTree`, which would otherwise stack one fsnotify
  watcher per refresh.

### 2.2 One file change, layer by layer

1. Editor or kubelet writes a new timestamped dir and renames `..data` onto it (atomic).
2. fsnotify delivers a CREATE/RENAME event **on the directory** (the file's inode changed, which
   is exactly why the watch is on the parent dir, never the file — filewatch.go:80-86).
3. `watchLoop` reacts to every event without name filtering (correct: a K8s update surfaces as
   an event on `..data`, not on the key files — starter.go:112-116) → `TriggerRefresh`.
4. `RefreshProperties` refuses to run if the app hasn't finished wiring (app.go:248); otherwise
   it re-runs the whole config load — both providers, env, cmd args — merges by priority and
   swaps `gs.Dync` values atomically. One swap typically produces two events (temp-symlink
   create + rename), hence the doubled `loaded ... config` log lines you see per update: no
   debounce, by design.

---

## 3. Per-key behavior reference

The starter exposes **no property keys of its own** — the only `value:` tags in this repo are
the example beans' (`${demo.message:=none}` etc.), which are ordinary absolute property
references, not starter keys. The whole surface is the import-string grammar
`[optional:]<provider>:<path>` parsed by gs core (provider.go:74-104).

### 3.1 Import string grammar

| Segment | Type | Default | Behavior / interactions | Misconfiguration consequence |
|---------|------|---------|-------------------------|------------------------------|
| `optional:` prefix | flag | absent | Parsed before the provider split; provider `Load` receives `optional=true` and returns `(nil, nil)` with a Warn log when the path is missing (filewatch.go:67-70, configtree.go:61-64). | Absent + missing path → stat error, startup aborts. |
| `file-watch` provider | enum | `file` | One document per import, format by extension. Must be a file. | Typo → `unsupported provider type` at startup. Note the core default for a bare path is `file`, NOT `file-watch` — bare paths get no watching. |
| `configtree` provider | enum | `file` | A directory tree; each non-dot leaf file = one property. Must be a directory. | Pointing it at a file → explicit error steering you to `file-watch` (configtree.go:68-71). |
| `<path>` (file-watch) | file path | — | Resolved for property placeholders before loading (`conf.Resolve`, gs_conf/conf.go:229). Extension routed through the shared reader registry: `.properties/.yaml/.yml/.toml/.tml/.json`. Parent dir is watched. | Directory path → error steering you to `configtree` (filewatch.go:75-78). Unsupported extension (`.md`) → read error (unit-tested). |
| `<path>` (configtree) | dir path | — | Key = dotted relative path (`db/user` file → `db.user`; a flat file literally named `db.user` also → `db.user`, since K8s keys may contain dots); value = file content `TrimSpace`d, NOT parsed (configtree.go:116-125). Every directory in the tree is watched. ⚠ numeric/boolean-looking values stay raw strings — cast in your code. | Expecting yaml parsing inside a key file → literal text ends up as the value. |
| dot-prefixed entries | filter | skipped | `..data`, `..timestamp` dirs and dotfiles are skipped at every level; the root itself is never skipped (configtree.go:103-108). Symlink leaves are followed (`os.ReadFile`). | None — this is what makes K8s mounts clean. |
| import list order | list | — | `spring.config.import` is comma-separated, deduplicated; **later imports override earlier ones**; imports override the importing file's own keys (layered storage, gs_conf/conf.go:244-251). ⚠ no query parameters exist — the source string is a bare path. | Wrong order → layered overrides silently inverted. |
| `spring.http.server.enabled` | bool | true | Core gs key, not this starter's; examples disable it because the demo has no HTTP surface. | Left true → default server starts on :8080. |

### 3.2 What does NOT exist

No refresh-interval, debounce, include/exclude or format query params; no `enabled` key; no
log tag of its own (uses the shared `_app_def` infra tag); no health indicator; no metrics.

---

## 4. Verification & fault drills

Each drill is isomorphic to the smoke-tested examples (`example/check.sh`,
`example-configtree/check.sh`, both verified green).

### 4.1 Cold load

```bash
go run ./example            # log: "loaded file-watch config from file=./mount/application.properties keys=1"
                            #      "initial value: initial"  <- bound gs.Dync sees the value
go run ./example-configtree # log: "loaded configtree from dir=./mount keys=3"
```

### 4.2 Hot reload via edit (the centerpiece)

```bash
go run ./example -manual &     # server stays up
echo 'demo.message=flipped' > example/mount/application.properties
sleep 1
# log shows "loaded file-watch config ... keys=1" again (twice: no debounce),
# and any handler reading the Dync field now sees "flipped".
```

The atomic variant (what the kubelet does, and what `check.sh` automates): write a fresh
`..2025_...` dir, symlink `..data_tmp` → it, `rename(..data_tmp, ..data)`. The directory watch
sees the swap even though the key file's inode changed. Hot reload is observed within ~1s
locally; on Kubernetes add the kubelet's projection sync latency (~1 min by default).

### 4.3 Malformed file handling

```bash
echo 'demo: [broken' > example/mount/application.yaml   # invalid yaml
```

The next refresh's `Load` fails; `RefreshProperties` aborts **before** any partial update
(app.go:246-255: validation failure ⇒ no partial updates), so the last-good values stay live.
The refresh error is now logged by `TriggerRefresh` as a WARN
(`property refresh after file change failed, previous snapshot retained: ...`); watcher
creation failures and fsnotify channel errors are logged as WARNs too. Startup with a
malformed required file, by contrast, fails loudly with `file-watch: read ... failed`.

### 4.4 Optional vs required

```bash
spring.config.import=optional:file-watch:/etc/config/application.yaml
```

- present → loaded normally; missing → Warn `optional config path ... not found (skipped)`,
  startup continues.
- without `optional:` → missing path aborts startup with the stat error.
- A path that exists but has the wrong TYPE (file vs dir) is an error regardless of
  `optional:` — optionality only covers non-existence (os.IsNotExist check, filewatch.go:67).

### 4.5 Observables

Log tag `_app_def` (infra default): per load — `loading ... from <path>` (Debug),
`loaded ... keys=<n>` (Info), `optional ... not found (skipped)` (Warn). No metrics/spans;
the refresh count is inferable from repeated `loaded` lines.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Startup fails: `file-watch: stat ... failed` | required path missing | Add the file, or prefix `optional:`. |
| Startup fails: `expects a single file, got directory` / `expects a directory, got file` | wrong provider for the path shape | Switch `file-watch:` ⇄ `configtree:` — the error text names the right one. |
| Startup fails: `unsupported provider type` | typo in provider name, or assumed a bare path means file-watch | Bare paths route to gs's built-in `file` provider with **no watching**; write `file-watch:` explicitly. |
| Values never hot-reload, no errors | watcher creation failed silently (best-effort, starter.go:86-105 returns without logging) or fsnotify/inotify limits exhausted | Check `ulimit`/inotify watches; look for the missing `loaded` line after an edit; restart recovers. |
| Edit applied, then config "sticks" stale | a later edit broke parsing; refresh errors are swallowed (§4.3) | Fix the file; watch for the next `loaded ... config` line to confirm recovery. |
| Changed a value but the field didn't move | bound as a plain field, not `gs.Dync` | Only `gs.Dync[T]` hot-reloads; plain `value:` tags are startup-only. |
| Nested import not loading | imports inside an imported file are ignored (one level only, gs_conf/conf.go:210-215) | Declare all imports in `conf/app.*`. |
| Override order wrong | later imports win; imports beat the importing file | Reorder the comma list — there is no priority parameter. |
| Extra `..data`-ish keys appear | none expected — dot entries are skipped | If seen, the path isn't being served by `configtree` (e.g. bare `file:` import). |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys (own) | 0 (grammar only: `[optional:]<provider>:<path>`) |
| Required | 0 (path required once a provider is named) |
| Quickstart external deps | 0 |
| "Watch out" entries | 6 |

Design suspects (kept from the previous audit, plus new ones — for the audit ledger):

- Watcher-creation failure keeps the static-snapshot behavior but now logs a WARN naming the
  directory (fixed in this pass).
- Every directory event triggers a full refresh (no file-name filtering, no debounce) —
  correct but chatty under edit storms; each K8s swap fires ~2 full reloads.
- Refresh errors from `RefreshProperties` are now logged as WARNs in `TriggerRefresh`
  (fixed in this pass), so a malformed watched file no longer degrades silently.
- `optional:` semantics live in each provider (os.IsNotExist checks duplicated in both Load
  functions) rather than in the provider framework — minor duplication.
- The provider framework's bare-path default (`file`) shadows this starter's purpose;
  users writing `./config.yaml` get no watching and no hint.
- K8s projection latency (~1 min) is outside the starter's control; the API-watch
  alternative lives in starter-config-k8s — cross-reference only, not actionable here.
