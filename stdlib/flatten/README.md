# flatten

[English](README.md) | [中文](README_CN.md)

`flatten` turns hierarchical JSON-shaped data into flat `key -> string` maps and
provides the `Storage` abstraction the Go-Spring configuration binder reads
against. It is not a configuration loader: it never reads files, environment
variables, or command-line flags — callers build a `Properties` themselves and
slot it into a layer.

## Usage

```go
import "go-spring.org/stdlib/flatten"

flat := flatten.Flatten(map[string]any{
    "server": map[string]any{"port": 8080, "host": "localhost"},
    "users":  []any{map[string]any{"name": "tom"}},
})
// flat == {"server.port":"8080","server.host":"localhost","users[0].name":"tom"}

path, err := flatten.SplitPath("server.port")
_ = path // [{key server} {key port}]
_ = flatten.JoinPath(path)

s := flatten.NewPropertiesStorage(flatten.NewProperties(flat))
v, _ := s.Value("server.port")
```

### API

| API | Description |
|---|---|
| `Flatten(map[string]any) map[string]string` | Flatten a nested map into `key -> string` |
| `Path` / `PathType` | One parsed segment of a hierarchical key (key or index) |
| `SplitPath(key) ([]Path, error)` | Parse `"a.b[0]"` into `[]Path`; errors on malformed syntax |
| `JoinPath([]Path) string` | Inverse of `SplitPath`, round-trippable |
| `Properties` | Flat `key -> string` store with `Get` / `Set` / `Data` |
| `MapProperties(map[string]any) *Properties` | Shorthand for `NewProperties(Flatten(m))` |
| `PropertiesStorage` | Adapts `Properties` to the `Storage` interface |
| `PrefixedStorage` | Wraps any `Storage`, transparently prefixing all keys |
| `LayeredStorage` | Combines config sources with fixed precedence; implements `Storage` |
| `LayeredStorage.Sources() []Source` | Read-only snapshot of every source (priority order, unmerged) |

## Flattening Rules

`Flatten` targets the data shape produced by `encoding/json.Unmarshal` and
expands recursively:

| Input shape | Output |
|---|---|
| `{"a":{"b":1}}` | `"a.b" = "1"` (maps expand with `.`) |
| `{"a":[1,2]}` | `"a[0]" = "1"`, `"a[1]" = "2"` (slices expand with `[i]`) |
| `{"a":nil}` / typed nil (nil map / nil slice / nil pointer) | `"a" = "<nil>"` |
| `{"a":{}}` (non-nil empty map) | `"a" = "{}"` |
| `{"a":[]}` (non-nil empty slice) | `"a" = "[]"` |
| Primitives | Deterministic `strconv` formatting (`true`, `8080`, `3.14`) |

## Key Path Syntax (Path / SplitPath / JoinPath)

`SplitPath` is the binder's entry point for parsing hierarchical keys; its
grammar maps one-to-one onto `Flatten` output:

```
"foo.bar[0]" -> [{key foo} {key bar} {index 0}]
"a[1][2]"    -> [{key a} {index 1} {index 2}]
```

`JoinPath` is the inverse of `SplitPath`: `JoinPath(SplitPath(k)) == k` for
every valid key, so the `Path` slice is the canonical intermediate
representation for manipulating keys in code (iterate, rewrite, truncate).

## The Storage Interface

The binder depends on exactly these four methods — the single contract for
plugging in a custom config source:

```go
type Storage interface {
    Exists(key string) bool                          // property condition checks (OnProperty etc.)
    Value(key string) (string, bool)                 // leaf value lookup (exact match)
    MapKeys(key string, result map[string]struct{}) bool // enumerate direct child keys of a map node
    SliceEntries(key string, result map[string]string) bool // enumerate all flat entries of a slice node
}
```

Semantic notes (using `PropertiesStorage`, with `server.port=8080`):

- **`Exists` is prefix-aware**: both `Exists("server")` and
  `Exists("server.port")` return `true` (intermediate nodes count as existing);
  `Exists("server.host")` returns `false`. It serves "enabled once a prefix is
  configured" style condition checks, not binding.
- **`Value` matches exactly** — no prefix guessing, no path assembly.
- **`MapKeys` returns one level only**: given `server.host` and `server.port`,
  `key="server"` yields `{"host","port"}` — no recursion.
- **`SliceEntries` does not validate index continuity**: it collects every
  `key[i]...` entry; `[0]` and `[2]` both present is fine, correctness is the
  binder's call.

Implementations only need to guarantee "the input data is already valid" —
`Storage` itself performs no structural validation, which keeps adapters for
remote config centers (nacos/etcd etc.) extremely thin.

## LayeredStorage: Layered Precedence

`LayeredStorage` combines config sources in a Spring-style layered model, with
five fixed layers (smaller value = higher priority):

| Layer | Meaning |
|---|---|
| `StorageCommandLine` | Command-line arguments (highest) |
| `StorageEnvironment` | Environment variables |
| `StorageProfileFile` | Profile files (e.g. `application-dev.properties`) |
| `StorageAppFile` | Main config file (`application.properties` / `.yml`) |
| `StorageDefault` | Built-in defaults (lowest) |

```go
ls := &flatten.LayeredStorage{}
ls.AddStorage(flatten.StorageAppFile, appStorage, "application.properties")
ls.AddStorage(flatten.StorageEnvironment, envStorage, "env")
```

Precedence rules:

1. The smaller layer index wins;
2. **Within a layer, the most recently added source wins** (new sources are
   inserted at the head of the layer's slice).

### Three Aggregation Semantics Across Layers

This is the most easily misunderstood — and most important — set of rules in
the package: **different structures deliberately aggregate differently across
layers**:

| Structure | Semantics | Behavior |
|---|---|---|
| Leaf value | Override | First match scanning from the highest-priority layer wins |
| Map | Merge | Union of child keys across all layers; each key's value still resolves by leaf override |
| Slice | Replace wholesale | The first layer defining the slice wins; lower layers are shadowed entirely |

Map merge example — two sources each define half, the binding side sees the
full map:

```
source1: server.port=8080
source2: server.host=localhost
MapKeys("server") -> {port, host}
```

Slice override example — **not** per-element override:

```
source1: my.list[0]=a, my.list[1]=b     (lower-priority layer)
source2: my.list[0]=c                    (higher-priority layer)
SliceEntries("my.list") -> [c]           ✅ not [c, b]
```

The asymmetry is deliberate: "merging arrays" across config sources has no
clear semantics (concatenate? override by index? longer wins?), while merging
map keys is what callers expect — a profile file overrides individual fields
of the main file and the rest keep working. One more `SliceEntries` detail:
if a layer holds a **leaf value** at that key (`my.list=x`), a slice defined
by a lower layer is shadowed by the leaf and lookup reports not-found.

### Sources(): Introspection Snapshot

`Sources()` returns each source's `Name` + `Data` (a deep copy, mutate freely)
in priority order, **with no merging at all**. Because leaf, map, and slice
properties aggregate differently, any single merged view would necessarily be
distorted — keeping the raw layers is the faithful representation (the
actuator env endpoint uses it to show "which source each value came from").
It is a diagnostics path, not a binding path.

## Design

- `Flatten` is a display-oriented one-way conversion, **not reversible**; only
  JSON-native types are supported — structs, non-string map keys, and custom
  types are explicitly out of scope.
- `Path` + `Split/JoinPath` provide a round-trippable key representation; the
  `Storage` interface stays minimal — the three binding capabilities `Value` /
  `MapKeys` / `SliceEntries` plus `Exists` for property condition checks — so
  alternative implementations such as remote config are easy to plug in.
- `LayeredStorage` deliberately mixes two override regimes (see above): leaves
  and slices resolve to the highest-priority layer, while maps merge keys
  across layers with leaf values still resolving by override.
- `PrefixedStorage.SliceEntries` strips its own prefix so the namespace stays
  transparent; `LayeredStorage.Sources()` is an introspection snapshot, not a
  binding path.
- Every lookup is an O(n) full scan (`Exists` / `MapKeys` / `SliceEntries` all
  walk the `map[string]string`): fast enough at config scale, and the price
  for `Properties`' minimal representation; if large key sets ever matter, the
  optimization belongs in an index structure, not the interface.

## License

Apache License 2.0. See [LICENSE](../../LICENSE).
