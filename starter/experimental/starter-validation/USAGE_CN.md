# starter-validation 使用说明 — 参考手册

详细使用参考。概览见 [README_CN.md](README_CN.md)。所有行为声明均已对照 starter 源码
（`starter.go`、`starter_test.go`）与可运行的 [example/](example/)（`example/check.sh`
—— 无容器、无外部依赖）。**校验规则语义（`required`、`email`、`min=...`、自定义规则）
见 [go-playground/validator](https://github.com/go-playground/validator)** —— 下文只写
驱动注册模型与中立错误形状。这是一个刻意很小的面；本文参考刻意不注水。

**激活方式**：仅 blank import。`init` 注册 `default` 校验驱动。无 bean、无端口、
**无配置 key**（`schema.json` 声明为空属性集）。

---

## 1. 完整工程示例

演示两条消费路径的命令工程 —— 配置绑定校验 + 带本地化 400 的 Web 请求 handler。
文件树（即 example 本身）：

```
demo/
├── go.mod
├── main.go
├── messages_en.yaml
├── messages_zh.yaml
└── conf/app.properties   // 空：无容器，不需要 spring.* key
```

**go.mod**（关键依赖）：

```
require (
    go-spring.org/cloud    v0.0.0   // web/validation + web/i18n
    go-spring.org/spring   v1.3.x   // conf + conf/reader
    go-spring.org/starter-validation latest
)
```

**main.go**（取自 example）：

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

    _ "go-spring.org/starter-validation" // 注册 "default" 驱动
)

// ServerConfig 先绑定再校验 —— 配错的环境 fail fast。
// validate tag 与 Web 路径用的是同一套规则词汇。
type ServerConfig struct {
    Admin string `value:"${admin}" validate:"required,email"`
    Port  int    `value:"${port}" validate:"min=1024"`
}

// SignupRequest 是 Web 请求体。
type SignupRequest struct {
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age" validate:"min=18"`
}

func main() {
    src := loadMessages()                       // en + zh 两个 bundle
    demoConfig(src)                             // 配置绑定路径
    demoWeb(src, "en", `{"email":"bad","age":10}`)
    demoWeb(src, "zh", `{"email":"bad","age":10}`)
    demoWeb(src, "en", `{"email":"a@b.com","age":20}`) // ok
}

// demoConfig：绑定一份故意配错的配置，再校验。
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

// demoWeb：经 validation.Handle 解码 + 校验 + 本地化拒绝。
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

**messages_en.yaml**（zh 同构，见 `example/messages_zh.yaml`）：

```yaml
# 键遵循 FieldError.MessageKey() 的 "validation.<rule>" 约定；
# {0} 是字段路径，{1} 是规则参数。
validation:
  required: "{0} is required"
  email: "{0} must be a valid email address"
  min: "{0} must be at least {1}"
```

**验证**：

```bash
cd starter/experimental/starter-validation/example && ./check.sh
# 期望：每个 locale 两条本地化失败信息、合法请求体 status=200；退出码 0
```

---

## 2. 装配与时序

### 2.1 注册模型

`starter.go` 的 `init`：

```
import starter-validation
  └─ init(): validation.RegisterDriver("default", playgroundDriver{})
        └─ 经 log.TagAppDef 打 debug 日志（无自有 tag）
```

- **名字**：驱动占用全局名 `default`。用 `validation.GetDriver("default")` /
  `MustGetDriver` 获取，用 `Driver.NewValidator()` 构造 validator。仅此而已 ——
  没有容器 bean、没有生命周期。
- **选择**：按名隐式选择。调用 `validation.Validate(ctx, "default", v)` 的消费方无需
  改代码即得到 go-playground/validator —— 这正是抽象/驱动拆分的意义。竞争实现用
  别的名字注册，消费方传那个名字即可。
- **构造**：`playgroundDriver.NewValidator` 每次新建
  `validator.New(validator.WithRequiredStructEnabled())` —— 按该选项自身语义，
  缺 tag 的 struct 字段默认必填。

### 2.2 一次 Validate 的逐层走读

`validation.Validate(ctx, "default", &cfg)`：

1. 注册表查找：`validation.GetDriver("default")` → `playgroundDriver`。
2. `NewValidator()` → `playgroundValidator{v: validator.New(...)}`。
3. `Validate`（starter.go）执行 `v.Struct(value)`：
   - nil error → 合法；
   - `*validator.InvalidValidationError`（如传了 nil 指针）→ **原样返回**：
     这是编程错误而非字段失败；
   - `validator.ValidationErrors` → 映射到中立的 `validation.ValidationErrors`，
     每条失败规则一个 `validation.FieldError`：`Field` = `fe.Namespace()`（完整结构
     路径，如 `user.Email`）、`Rule` = `fe.Tag()`（如 `email`）、`Param` =
     `fe.Param()`（如 `18`）、`Value` = 违规值。

rule 兼作 i18n key：`FieldError.MessageKey()` = `validation.<tag>` —— 上面的 yaml
bundle 就是按它取键。`starter_test.go` 固化了该契约
（`TestValidateMapsErrorsToNeutralShape`：`Rule=="email"`、`Field=="user.Email"`、
`Param=="18"`、`MessageKey()=="validation.email"`）。

---

## 3. 逐 key 行为参考

**无。** starter 是纯注册；`grep -rhoE 'value:"[^"]+"' starter-validation` 只命中
example 自己的 `${admin}`/`${port}` 演示绑定。规则语法来自 validator 的
`validate:"..."` tag；错误文案来自你提供的 i18n bundle。

---

## 4. 验证与故障演练

### 4.1 驱动注册

```bash
cd starter/experimental/starter-validation && go test ./...
# TestDriverRegisteredAsDefault 只有在 blank-import 接线生效时才通过
```

或程序化验证：`validation.GetDriver("default")` 必须无错返回非 nil 驱动。

### 4.2 中立错误形状演练

校验 `&user{Email: "not-an-email", Age: 5}`（tag `required,email` / `min=18`）：期望
恰好 2 条 `FieldError` —— 一条 `Rule: "email"`、一条 `Rule: "min"` 且 `Param: "18"`。
这是 Web 绑定路径依赖的验收契约。

### 4.3 i18n 演练

运行 example：同一失败请求体在 `WithLocale(ctx, "en")` 下渲染英文、在 `"zh"` 下渲染
中文 —— 证明 `MessageKey()` 与 locale 无关、差异只在 bundle。bundle 缺某规则键时按
i18n.MapSource 自身语义回退 —— 补上 `validation.<tag>` 键即可。

### 4.4 编程错误演练

`v.Validate(ctx, (*user)(nil))` 返回未包装的 `*validator.InvalidValidationError` ——
不要无条件断言 `validation.ValidationErrors`；先做类型断言检查。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| `validation.GetDriver("default")` 报 "not registered" | 该二进制没 blank-import starter | 加 `import _ "go-spring.org/starter-validation"`。 |
| 启动 panic：driver 名冲突 | 同时导入两个 "default driver" starter | 只能有一个占用 `default` —— 去掉一个。 |
| 错误渲染成裸键（`validation.email`） | i18n bundle 缺该规则键 | 给每个 bundle 补 `validation.<tag>: "..."`。 |
| 消息里字段名不对 | 期望的是短字段名 | `Field` 按设计是 `fe.Namespace()`（完整路径）—— 见 §2.2。 |
| 断言 `ValidationErrors` 时 panic | 入参是 nil 指针 → 原样返回 `InvalidValidationError` | 先检查断言；修传 nil 的调用方。 |
| 首次 Validate 报自定义规则 "not a valid validation tag" | 规则只注册在某 validator 实例上 | 自定义函数无注入点（嫌疑 #2）—— 用 struct-level tag 或换驱动。 |
| struct 字段意外过不了 `required` | `WithRequiredStructEnabled()` 语义 | 预期默认；见 validator 关于该选项的文档。 |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 0 |
| 其中必填 | 0 |
| quickstart 前置外部依赖 | 0 |
| "注意/坑" 条数 | 3 |

设计嫌疑清单（沿用上版，交设计裁决）：

1. 驱动在 import 时占用全局名 `default` —— 导入两个 "default driver" starter 会在
   注册时冲突，且无编译期信号。
2. `NewValidator` 每次新建 `validator.Validate`（不缓存自定义 tag 注册）—— 自定义
   校验函数没有受支持的注入点。
