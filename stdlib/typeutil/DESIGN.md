# typeutil Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

Part of the zero-dependency `stdlib` layer. `typeutil` centralises the
"what kind of type is this?" predicates the Go-Spring container uses when
scanning candidates for injection or provider registration.

## 1. Responsibilities & Boundaries

- Own the definition of "primitive value type", "constructor", "bean type",
  and "injection / binding target". These names show up in error messages
  from the container, so the definitions live in one file for grep-ability.
- Not a general reflection utility library. Anything that would require
  poking at runtime values (as opposed to `reflect.Type`) belongs elsewhere
  — usually alongside the code that consumes it.

## 2. Key Decisions

- **`IsBeanType` shape**: `chan`, `func`, `interface`, and `*struct`. Value
  structs are deliberately excluded — the container works with references so
  it can install proxies / advice. Callers that want value semantics need to
  wrap them in a pointer.
- **`IsConstructor` shape**: either `func() T` (T not error) or
  `func() (T, error)`. Anything else (multiple return values, `func() error`
  alone) is rejected upstream by the container.
- **`IsPropBindingTarget` vs. `IsBeanInjectionTarget`**: two separate
  predicates because the container treats "give me a config value" and
  "give me a dependency" as different injection paths with different valid
  target shapes.
- Nil `reflect.Type` is handled in `IsErrorType` and
  `IsBeanInjectionTarget` (return `false`), but not everywhere; callers with
  possibly-nil types should guard first.

## 3. Constraints

- Zero dependencies. Everything is `reflect` plus generic constraints. This
  package is imported by almost every container / aspect internal, so the
  bar for adding an import is very high.
