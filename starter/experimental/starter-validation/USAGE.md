# starter-validation Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `starter_test.go`) and the runnable
[example/](example/) (`example/check.sh` — no container, no external deps). **Validation-rule
semantics (`required`, `email`, `min=...`, custom rules) are
[go-playground/validator](https://github.com/go-playground/validator)** — only the driver
registration model and the neutral error shape are covered here. This is a deliberately
tiny surface; the reference is short on purpose.

**Activation**: blank import only. `init` registers the `default` validation driver.
No beans, no port, **no configuration keys** (`schema.json` declares an empty property set).

---

## 1. Complete worked project

A command demonstrating both consumption paths — config-binding validation and a Web request
handler with localized 400s. File tree (this IS the example):

```
demo/
├── go.mod
├── main.go
├── messages_en.yaml
├── messages_zh.yaml
└── conf/app.properties   // empty: no container, no spring.* keys needed
```

**go.mod** (module deps that matter):

```
require (
    go-spring.org/cloud    v0.0.0   // web/validation + web/i18n
    go-spring.org/spring   v1.3.x   // conf + conf/reader
    go-spring.org/starter-validation latest
)
```

**main.go** (from the example):

```go
package main

import (
    "context"
    "net/http"
    "net/http/httptest"
    "strings"

    "go-spring.org/cloud/experimental/web/i18n"
    "go-spring.org/cloud/experimental/web/validation"
    "go-spring.org/spring/conf"
    "go-spring.org/spring/conf/reader"
    "go-spring.org/stdlib/flatten"

    _ "go-spring.org/starter-validation" // registers the "default" driver
)

// ServerConfig is bound from configuration, then validated — a misconfigured
// environment fails fast. The validate tags are the SAME ones the Web path
// uses: one rule vocabulary for both.
type ServerConfig struct {
    Admin string `value:"${admin}" validate:"required,email"`
    Port  int    `value:"${port}" validate:"min=1024"`
}

// SignupRequest is the Web request body.
type SignupRequest struct {
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age" validate:"min=18"`
}

func main() {
    src := loadMessages()                       // en + zh bundles
    demoConfig(src)                             // config-binding path
    demoWeb(src, "en", `{"email":"bad","age":10}`)
    demoWeb(src, "zh", `{"email":"bad","age":10}`)
    demoWeb(src, "en", `{"email":"a@b.com","age":20}`) // ok
}

// demoConfig: bind a deliberately broken config, then validate it.
func demoConfig(src *i18n.MapSource) {
    store := flatten.NewPropertiesStorage(flatten.NewProperties(map[string]string{
        "admin": "not-an-email",
        "port":  "80",
    }))
    var cfg ServerConfig
    if err := conf.Bind(store, &cfg, "${ROOT}"); err != nil {
        panic(err)
    }
    if err := validation.Validate(context.Background(), "default", &cfg); err != nil {
        render := i18n.Localizer(src, i18n.WithLocale(context.Background(), "en"))
        for _, line := range err.(validation.ValidationErrors).Localize(render) {
            println("-", line)
        }
    }
}

// demoWeb: decode + validate + localized rejection via validation.Handle.
func demoWeb(src *i18n.MapSource, locale, body string) {
    v, _ := mustValidator()
    handler := validation.Handle[SignupRequest](v, nil,
        func(e validation.FieldError) string {
            ctx := i18n.WithLocale(context.Background(), locale)
            s, _ := src.Message(ctx, e.MessageKey(), e.Field, e.Param)
            return s
        },
        func(w http.ResponseWriter, _ *http.Request, req *SignupRequest) {
            _, _ = w.Write([]byte("ok: " + req.Email))
        },
    )
    rec := httptest.NewRecorder()
    handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(body)))
}

func mustValidator() (validation.Validator, error) {
    d, err := validation.GetDriver("default")
    if err != nil {
        return nil, err
    }
    return d.NewValidator()
}
```

**messages_en.yaml** (zh analogous — see `example/messages_zh.yaml`):

```yaml
# Keys follow the "validation.<rule>" convention produced by
# FieldError.MessageKey(); {0} is the field path, {1} the rule param.
validation:
  required: "{0} is required"
  email: "{0} must be a valid email address"
  min: "{0} must be at least {1}"
```

**Verify**:

```bash
cd starter/experimental/starter-validation/example && ./check.sh
# expects the two localized failure lines per locale and status=200 for the
# valid body; exit 0
```

---

## 2. Assembly & timing

### 2.1 The registration model

`starter.go` `init`:

```
import starter-validation
  └─ init(): validation.RegisterDriver("default", playgroundDriver{})
        └─ debug log via log.TagAppDef (no tag of its own)
```

- **Name**: the driver claims the global name `default`. Retrieve with
  `validation.GetDriver("default")` / `MustGetDriver`; construct validators with
  `Driver.NewValidator()`. There is nothing else — no container beans, no lifecycle.
- **Selection**: implicit by name. Consumers that call
  `validation.Validate(ctx, "default", v)` get go-playground/validator without any code
  change — that is the whole point of the abstraction/driver split. A competing
  implementation registers under a different name and consumers pass that name instead.
- **Construction**: `playgroundDriver.NewValidator` builds a fresh
  `validator.New(validator.WithRequiredStructEnabled())` per call — struct fields become
  required by default when the tag is absent-struct, per the option's own semantics.

### 2.2 One Validate call, layer by layer

`validation.Validate(ctx, "default", &cfg)`:

1. Registry lookup: `validation.GetDriver("default")` → `playgroundDriver`.
2. `NewValidator()` → `playgroundValidator{v: validator.New(...)}`.
3. `Validate` (starter.go) runs `v.Struct(value)`:
   - nil error → valid;
   - `*validator.InvalidValidationError` (e.g. nil pointer passed) → returned **verbatim**:
     a programming error, not a field failure;
   - `validator.ValidationErrors` → mapped onto the neutral
     `validation.ValidationErrors`, one `validation.FieldError` per failing rule:
     `Field` = `fe.Namespace()` (full struct path, e.g. `user.Email`), `Rule` = `fe.Tag()`
     (e.g. `email`), `Param` = `fe.Param()` (e.g. `18`), `Value` = offending value.

The rule doubles as the i18n key: `FieldError.MessageKey()` = `validation.<tag>` — that is
what the yaml bundles above key on. `starter_test.go` pins this contract
(`TestValidateMapsErrorsToNeutralShape`: `Rule=="email"`, `Field=="user.Email"`,
`Param=="18"`, `MessageKey()=="validation.email"`).

---

## 3. Per-key behavior reference

**None.** The starter is pure registration; `grep -rhoE 'value:"[^"]+"' starter-validation`
yields only the example's own `${admin}`/`${port}` demo bindings. Rule syntax comes from the
validator's `validate:"..."` tags; error wording comes from the i18n bundles you supply.

---

## 4. Verification & fault drills

### 4.1 Driver registration

```bash
cd starter/experimental/starter-validation && go test ./...
# TestDriverRegisteredAsDefault passes only when the blank-import wiring works
```

Or programmatically: `validation.GetDriver("default")` must return a non-nil driver with
no error.

### 4.2 Neutral error shape drill

Validate `&user{Email: "not-an-email", Age: 5}` (tags `required,email` / `min=18`): expect
exactly 2 `FieldError`s — one `Rule: "email"`, one `Rule: "min"` with `Param: "18"`. This
is the acceptance contract the Web binding path depends on.

### 4.3 i18n drill

Run the example: the same failing body renders English under `WithLocale(ctx, "en")` and
Chinese under `"zh"` — proving `MessageKey()` is locale-independent and only the bundle
differs. Miss a rule key in a bundle and the renderer falls back per i18n.MapSource's own
semantics — add the `validation.<tag>` key.

### 4.4 Programming-error drill

`v.Validate(ctx, (*user)(nil))` returns `*validator.InvalidValidationError` unwrapped — do
not type-assert `validation.ValidationErrors` unconditionally; check the assertion.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `validation.GetDriver("default")` errors "not registered" | starter not blank-imported in that binary | Add `import _ "go-spring.org/starter-validation"`. |
| Panic at startup: driver name collision | two "default driver" starters imported | Only one may claim `default`; drop one. |
| Errors render as raw keys (`validation.email`) | i18n bundle missing the rule key | Add `validation.<tag>: "..."` to each bundle. |
| Field names wrong in messages | expecting the short field name | `Field` is `fe.Namespace()` (full path) by design — see §2.2. |
| Type-assert panic on `ValidationErrors` | input was a nil pointer → `InvalidValidationError` returned verbatim | Check the assertion; fix the caller passing nil. |
| Custom rule "not a valid validation tag" at first Validate | tag registered on one validator instance, not the driver's | No injection point for custom funcs (suspect #2) — use struct-level tags or swap drivers. |
| Struct fields unexpectedly failing `required` | `WithRequiredStructEnabled()` semantics | Intended default; see validator docs on the option. |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 0 |
| Required | 0 |
| Quickstart external deps | 0 |
| "Watch out" entries | 3 |

Design suspects (kept from the previous edition; for the audit ledger):

1. The driver claims the global name `default` on import — importing two "default driver"
   starters would collide at registration with no compile-time signal.
2. `NewValidator` builds a fresh `validator.Validate` per call (no cached custom tag
   registrations), so custom validation functions have no supported injection point.
