# starter-config-vault Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`) and the smoke-tested [example/](example/).
**Vault's own semantics (KV engines, tokens, namespaces, leases, sealing) are
[Vault's documentation](https://developer.hashicorp.com/vault/docs)** — everything below is
go-spring's increment.

**Activation**: blank-importing the package registers the `vault` config provider
(`conf.RegisterProvider("vault", ...)`). It does nothing until a `vault:` entry appears in
`spring.config.import`. There is no `enabled` key, no property-key surface, and no injectable
bean — the entire user surface is the import string.

**Scope**: config-center role only — reads a KV secret and exposes its fields as application
properties, with poll-based hot-reload. A Vault Agent / CSI-mounted secret *file* belongs to
starter-config-file.

---

## 1. Complete worked project

Isomorphic to [example/](example/) (smoke-tested by `example/check.sh` against a dev-mode
Vault). File tree:

```
demo/
├── go.mod
├── main.go
├── conf/
│   └── app.properties
└── docker-compose.yml     # dev Vault for local drills
```

**go.mod** (deps that matter):

```
require (
    github.com/hashicorp/vault/api latest
    go-spring.org/spring            latest
    go-spring.org/starter-config-vault latest
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-config-vault"
)

// Demo binds two fields from the Vault-sourced document. gs.Dync[T] is what
// actually hot-reloads when the secret changes; a plain string would be bound
// once at startup and stay frozen.
type Demo struct {
    Message  gs.Dync[string] `value:"${demo.message:=none}"`
    Password gs.Dync[string] `value:"${demo.password:=none}"`
}

func main() {
    gs.Provide(&Demo{}).Export(gs.As[gs.Rooter]())
    gs.Run()
}
```

**conf/app.properties** — the complete surface:

```properties
# Import one config document out of a Vault KV v2 secret.
#  - optional:  app starts even when the secret does not exist yet
#  - 127.0.0.1:8200  Vault address (scheme defaults to http; add &scheme=https for TLS)
#  - secret/gs-config-demo   <mount>/<path>
#  - key=application.properties  single-field mode: that field holds a full
#    properties document, parsed with reader format "properties"
#  - poll-ms=1000  poll the secret every second for changes
# The Vault token NEVER lives in this file: it comes from VAULT_TOKEN /
# VAULT_TOKEN_FILE / ?token-file= (see §3).
spring.config.import=optional:vault:127.0.0.1:8200/secret/gs-config-demo?kv-version=2&key=application.properties&format=properties&poll-ms=1000
```

**docker-compose.yml** (local drills only):

```yaml
services:
  vault:
    image: hashicorp/vault:1.16
    environment:
      VAULT_DEV_ROOT_TOKEN_ID: "root"
      VAULT_DEV_LISTEN_ADDRESS: "0.0.0.0:8200"
    ports: ["127.0.0.1:8200:8200"]
```

**Verify** (this is exactly what `example/check.sh` automates):

```bash
docker compose up -d
export VAULT_TOKEN=root

# Publish the config document the import points at. Single-field mode: the
# field name must equal the key= parameter.
vault kv put secret/gs-config-demo application.properties="demo.message=hello
demo.password=topsecret"   # or: docker exec ... vault kv put ...

go run .                      # watch for: loaded vault config from secret/gs-config-demo keys=2
# in another terminal:
vault kv put secret/gs-config-demo application.properties="demo.message=rotated"
# within ~poll-ms + refresh latency, Demo.Message.Value() == "rotated" — no restart
```

The example also demonstrates property-level decryption: a document value of
`ENC(aes:<base64>)` is decrypted by the conf binding pipeline before binding, with the AES
key supplied out of band via `GS_CONFIG_DECRYPT_AES_KEY` (or `..._KEY_FILE`). See
`spring/conf/decrypt/aes`. ⚠ The shipped `example/check.sh` exports `VAULT_TOKEN` but **not**
the AES key — run the encrypted-password drill with
`GS_CONFIG_DECRYPT_AES_KEY=<base64 key> go run .` (see §6).

---

## 2. Assembly & timing

### 2.1 When the import resolves — and why it must be pre-bean

`spring.config.import` is processed while gs loads application properties, **before any bean
is created** (`spring/gs/internal/gs_conf/conf.go`, `loadFileImports`): after each config
file is read, its `spring.config.import` value is resolved and loaded through the registered
provider chain, and the result is added as a `StorageAppFile` layer (or `StorageProfileFile`
when profiles are active). Later-loaded sources override earlier ones. Two grammar rules from
`spring/conf/provider/provider.go:74-104`:

- the import string is `[optional:]<provider>:<path>` — `optional:` first, then the provider
  name before the first `:`, the rest is the provider-specific path;
- imports nest **one level only**: a `spring.config.import` inside an imported source is
  silently ignored.

Pre-bean is not an implementation accident: every `value:"${...}"` tag — including the
`${demo.message:=none}` on your beans — binds from the merged property set, so the Vault
secret must already be a property layer before the container wires anything. The design
reason the starter can offer nothing else: it runs before the IoC container exists.

### 2.2 The full chain, in order

```
blank-import starter-config-vault
  └─ init(): conf.RegisterProvider("vault", vaultController.Load)   [starter.go:60]

gs.Run()
  ├─ config load: conf/app.properties read
  │    └─ loadFileImports sees spring.config.import=vault:...
  │         └─ conf.Load → vaultController.Load(optional, source)   [starter.go:228]
  │              ├─ parseSource: "vault://"+source → URL → host, mount/path, query  [starter.go:107]
  │              ├─ resolveToken: ?token → VAULT_TOKEN → ?token-file / VAULT_TOKEN_FILE  [starter.go:164]
  │              ├─ clientFor: cached api.Client per address|namespace|token         [starter.go:193]
  │              ├─ registerWatch: spawn watchLoop goroutine (poll every poll-ms)    [starter.go:347]
  │              ├─ readSecret: KVv2(mount).Get / KVv1(mount).Get, 5s timeout       [starter.go:280]
  │              │    └─ 404 → nil data → optional? warn+skip : error "secret not found"
  │              ├─ toProperties: flatten whole map, or key-mode parse one field     [starter.go:315]
  │              └─ return props → added as StorageAppFile layer
  ├─ bean wiring: controller stays outside the IoC container (no bean at all)
  ├─ field binding: ${demo.message} resolves from the Vault layer; gs.Dync fields register for refresh
  ├─ Run / readiness
  └─ steady state: each watchLoop ticks
       ├─ readSecret; on error → continue (silently)
       ├─ fingerprint(json.Marshal(data)) vs last loaded fingerprint
       └─ changed → TriggerRefresh → gs.RefreshProperties()
            └─ re-runs the whole property load: imports re-resolve, Load re-reads the
               secret, layers rebuild, and gs.Dync[T] fields swap their values
```

Key timings verified from source:

- **Not cold-load only.** A per-secret polling watcher runs forever (`watchLoop`,
  starter.go:366-381), default every 5000 ms (example uses 1000 ms). Secret rotation is
  picked up without restart — *but only into `gs.Dync[T]` fields*; plain `value` tags are
  bound once and never re-read (gs refresh is Dync-only).
- **Refresh is guarded by start state**: before the app has started,
  `gs.RefreshProperties()` returns an error and `TriggerRefresh` is a harmless
  no-op — the startup load already captured the config (starter.go:82-89).
- **Fingerprint is content-based** (`json.Marshal` of the KV data map): a KV v2 write that
  produces identical data does NOT trigger a refresh; KV v2 version numbers are ignored.
- **The watcher never re-reads the token**: an expired token keeps the client cached
  (clientKey = address|namespace|token), and `watchLoop` swallows read errors — see §4.4.

### 2.3 One rotation, layer by layer

`vault kv put secret/gs-config-demo application.properties="demo.message=rotated"` →

1. next `watchLoop` tick (≤ poll-ms later) re-reads the secret;
2. fingerprint differs from the one captured at load → `TriggerRefresh`;
3. `RefreshProperties` re-runs the config pipeline: `spring.config.import` re-resolves,
   `Load` re-reads the (new) secret, updates the stored fingerprint;
4. property layers rebuild top-down, `demo.message` now resolves to `rotated`;
5. every `gs.Dync` field bound to `${demo.message}` swaps its value; plain fields do not.

---

## 3. Per-key behavior reference

There are **no property keys** in this starter — `grep -rhoE 'value:"[^"]+"'` over the
package returns zero tags (the `demo.*` keys in the example are example data keys, not
starter surface). The entire surface is the import string:

```
[optional:]vault:<host>:<port>/<mount>/<path>?<query params>
```

Parsed by prefixing `vault://` and using `url.Parse` (starter.go:107-161), so `host:port`
must be a valid URL host, and the path must contain exactly `<mount>/<path>` split on the
first `/` (both parts non-empty).

### 3.1 Path parts

| Part | Type | Default | Behavior / interactions | Misconfiguration consequence |
|------|------|---------|--------------------------|------------------------------|
| `host:port` | string | — (**required**) | Combined with `scheme` into the client address. | Missing → `missing vault server address` error at startup. |
| `mount` | string | — (**required**) | KV engine mount point, passed to `cli.KVv2/KVv1(mount)`. | Wrong mount → 404 → required: startup error; optional: warn + skip. |
| `path` | string | — (**required**) | Secret path within the mount. | Same as above; ⚠ KV v2 paths do NOT include a `data/` prefix — the SDK adds it. |

### 3.2 Query params

| Param | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-------|------|---------|--------------------------|------------------------------|
| `kv-version` | int | `2` | `1` → `cli.KVv1(mount).Get`; `2` → `cli.KVv2(mount).Get`. Only 1 or 2 accepted. | Anything else → startup error `kv-version must be 1 or 2`. v1 secret read as v2 → 404-style failure. |
| `scheme` | string | `http` | Prepended to build `scheme://host:port`. ⚠ lives in the query, not the URL scheme. | Forgetting it against an HTTPS Vault → connection reset/cleartext-to-TLS-port errors. |
| `namespace` | string | empty | `cli.SetNamespace` — Vault Enterprise only. | Set against OSS Vault → request errors. |
| `key` | string | empty | **Single-field mode**: read only that field of the secret; it must exist and be a string; its content is parsed with `format` and the result flattened. | Field missing / non-string → startup error naming the field. |
| `format` | string | `properties` | Parser for single-field mode (`reader.Read`: properties/yaml/json/toml...). Ignored without `key`. | Mismatched format → parse error at startup. |
| `prefix` | string | empty | Prepends `prefix.` to every produced key. | Wrong prefix → downstream `${...}` tags silently fall back to defaults. |
| `poll-ms` | int | `5000` | Watch interval per secret; must be > 0. | `0`/negative/non-numeric → startup error `invalid poll-ms`. |
| `token` | string | — | Inline token; **discouraged** (lands in config files / logs). | — |
| `token-file` | string | — | Path to a file whose trimmed content is the token. | Unreadable file → startup error (even under `optional:`). |

### 3.3 Token resolution and `optional:` semantics

Resolution order (starter.go:164-185): `?token=` → `VAULT_TOKEN` env → `?token-file=` →
`VAULT_TOKEN_FILE` env (file content trimmed). ⚠ **A missing token fails at startup even for
`optional:` sources** — token resolution runs inside `parseSource`, before the optional
check. `optional:` only softens *secret read* failures (404 / transport): with it the app
starts with zero keys from that source; without it the first bad read aborts startup.

Env: `VAULT_TOKEN`, `VAULT_TOKEN_FILE`, plus decryption `GS_CONFIG_DECRYPT_AES_KEY` /
`GS_CONFIG_DECRYPT_AES_KEY_FILE` when the document contains `ENC(aes:...)` values.

---

## 4. Verification & fault drills

All drills use the §1 project. `vault` CLI needs `export VAULT_ADDR=http://127.0.0.1:8200
VAULT_TOKEN=root`.

### 4.1 Cold-load verification

```bash
vault kv put secret/gs-config-demo application.properties="demo.message=cold"
go run . 2>&1 | grep 'loaded vault config'   # "loaded vault config from secret/gs-config-demo keys=1"
```

### 4.2 Rotation drill (hot)

```bash
go run . -manual &        # example's manual mode keeps the server up
vault kv put secret/gs-config-demo application.properties="demo.message=rotated"
# within ~1s (poll-ms=1000): hot-reload observed: rotated  — no restart, only gs.Dync fields
```

Same-content rewrite does NOT refresh (fingerprint equality) — verify by re-putting the
same document and observing nothing happens.

### 4.3 Wrong path (required vs optional)

```bash
# required: spring.config.import=vault:.../secret/nope
go run .   # ERROR "vault secret secret/nope not found" → exit

# optional: optional:vault:.../secret/nope
go run .   # WARN "optional config secret secret/nope not found (skipped)" → starts, fields on defaults
```

### 4.4 Token expired / wrong mid-run

Start correctly, then revoke the token: `vault token revoke <id>`. The app keeps running —
startup is unaffected — but every subsequent poll fails and `watchLoop` `continue`s
silently (starter.go:370-373): **no log line, no counter, config silently goes stale**. New
writes to Vault never arrive; recovery requires restarting the process (the watcher never
re-reads the token). This is the drill to run before trusting rotation in production.

### 4.5 Vault sealed / down at startup

`vault operator seal` (or stop the container), then `go run .`:
- required source: `read vault secret ... failed` error, startup aborts (fail-fast);
- optional source: WARN `optional config read secret ... failed (skipped)`, app starts
  with defaults — verify the fields show their `:=` fallback values.

Note the 5-second read timeout (starter.go:281): a hung Vault delays startup by up to 5s
per import.

### 4.6 Missing token

```bash
env -u VAULT_TOKEN go run .   # ERROR "no vault token found (set VAULT_TOKEN, VAULT_TOKEN_FILE, ...)"
```

Fails even with `optional:` (§3.3).

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `no vault token found (set VAULT_TOKEN, ...)` at startup | No token in any of the 4 slots | Export `VAULT_TOKEN` or use `?token-file=`; error fires even for `optional:` sources. |
| `vault secret <mount>/<path> not found` | Wrong mount/path, or KV v1 secret read with default `kv-version=2` | Fix path; add `&kv-version=1`; KV v2 paths must NOT include `data/`. |
| `vault path must be <mount>/<path>` | Path with fewer than two segments (e.g. `vault:...:8200/secret`) | Ensure `<host>:<port>/<mount>/<path>`. |
| App starts, but Vault-sourced values stay at defaults | `optional:` swallowed a read failure, or wrong `prefix` | Grep startup logs for `optional config ... (skipped)`; check prefix vs your `${...}` keys. |
| Secret rotated in Vault, app never updates | Field is a plain value (not `gs.Dync`), or poll failing silently (revoked token) | Bind via `gs.Dync[T]`; run the §4.4 drill; restart to recover a dead token. |
| Connection reset / TLS errors | HTTPS Vault without `&scheme=https` | Add it — the default is `http`. |
| `parse vault field "x" as properties failed` or `has no field "x"` | Single-field mode mismatch | `key=` must name an existing string field; `format=` must match its content. |
| `kv-version must be 1 or 2` / `invalid poll-ms` | Non-numeric or out-of-range query params | Fix values; both are validated at parse time. |
| Fields flash correct then revert after refresh | A lower-priority layer (app file) overrides the Vault layer | Remember override order: later-loaded imports override earlier; profile layers beat app layers. |

---

## 6. Design health + suspects

| Metric | Value |
|--------|-------|
| Property keys | 0 (entire surface is the import string) |
| Import-string params | 10 (3 path parts + 7 query) |
| Required parts | 3 path parts + a token from any of 4 slots |
| Quickstart external deps | 1 (Vault) |
| Injectable beans / programmatic API | 0 / 0 |
| "Watch out" entries | 6 |

Design suspects (for the audit ledger; first three carried over from the previous edition):

1. Largest source-string surface of the config family (10 params); `token`/`prefix`/`poll-ms`
   would read better as property keys → candidate split of "connection" params into keys.
2. Poll-only change detection: no sys/leases notify, KV v2 version numbers ignored —
   same-content rewrite does not refresh; undocumented behavior confirmed by
   `fingerprint()` (content hash only).
3. Poll errors silently skipped (`continue`) with no failure counter or log — a dead token
   or sealed Vault degrades into silently-stale config (drill §4.4).
4. `resolveToken` runs inside `parseSource`, so a missing token fails even `optional:`
   sources — defensible fail-fast, but asymmetric with `optional:`'s stated meaning.
5. `scheme` rides in the query string (`?scheme=https`) instead of accepting a real
   `vault://host/...` / `https://host/...` URL form — easy to miss, no inline hint.
6. Example drift: `example/check.sh` exports `VAULT_TOKEN` but not
   `GS_CONFIG_DECRYPT_AES_KEY`, and `example.go` declares an unused `aesKey` const plus an
   init comment claiming it sets env vars it does not set — the ENC-decryption leg of the
   smoke test cannot pass as scripted; fix the script or the init.
7. No health indicator, no metrics, no dedicated log tag (uses `_app_def`): staleness is
   unobservable without external probing.
