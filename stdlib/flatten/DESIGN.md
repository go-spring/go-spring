# flatten Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`flatten` sits in the zero-dependency `stdlib` foundation. Its job is to
bridge hierarchical configuration data and the flat `key -> string` model the
Go-Spring binder wants to see.

## 1. Responsibilities & Boundaries

- Own the shape of a "flat property key" (dot-separated segments plus bracket
  indices) and every conversion between hierarchical and flat forms.
- Provide the `Storage` interface the binder programs against, plus the
  concrete implementations needed by the framework: single flat source
  (`PropertiesStorage`), prefixed view (`PrefixedStorage`), and layered
  precedence chain (`LayeredStorage`).
- Not a config loader. It never reads files, env, or CLI flags — the caller
  is responsible for building `Properties` and slotting it into a layer.

## 2. Key Abstractions

- `Flatten` — display-oriented, one-way conversion of JSON-shaped
  `map[string]any` to `map[string]string`. Explicitly documented as
  **not reversible**; intended for logging, diffing and, one step up, feeding
  a `Storage`.
- `Path` + `Split/JoinPath` — round-trippable representation of a key path,
  used by binding code that needs to walk keys segment by segment.
- `Storage` interface — three capabilities the binder actually needs
  (`Value`, `MapKeys`, `SliceEntries`) plus `Exists` for property-condition
  checks. Kept minimal so alternative implementations (e.g. remote config)
  can plug in.

## 3. Constraints & Trade-offs

- Only JSON-native types are supported by `Flatten` (map/slice/primitive/nil).
  Structs, non-string map keys, and custom types are out of scope by design.
- `LayeredStorage` mixes two override rules:
  - **Leaf values and slices**: highest-priority layer wins; lookup stops at
    the first hit. This means partial slices from lower layers are hidden
    once a higher layer defines the slice.
  - **Maps**: keys are merged across every layer, but per-leaf value
    resolution still follows override semantics.
  The asymmetry is intentional — merging arrays would be ambiguous, merging
  map keys is the shape callers expect.
- `PrefixedStorage.SliceEntries` re-strips its own prefix from returned keys
  so callers get keys in the caller's namespace, not the underlying store's.
- `LayeredStorage.Data()` is a snapshot for introspection (e.g. an actuator
  "env" endpoint), not a binding path.
