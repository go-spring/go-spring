# 待讨论:结构规则(临时)

以下规则从原 `layout-rules` 抽出,属于**项目结构规则**(与 domain 分层规则相对),不属于 `domain-rules`。
待确认去向:单列成 `structure-rules.md`?折回 AGENTS?或并入其他文档?

---

## 项目结构硬约束(zh)

- 顶层目录固定为 `idl/`、`internal/`。
- `main.go` 只做 side-effect import 与 IoC 启动,**禁止**承载业务装配代码;注册逻辑落到各层 `init.go`。
- IDL 生成产物统一落 `idl/<protocol>/gen/`,**禁止**手工修改生成文件,**禁止**把生成代码搬进 `internal/`。
- `internal/` 内部采用 domain 分层,遵循 `docs/domain-layout.md`。

## 命名与组织(zh)

- 包名全小写,包名不要与包内类型重名。
- 会被跨包引用的类型所在文件带领域前缀(`order_controller.go`、`order_service.go`);包内辅助文件(`converter.go`、`assembler.go`、`dto.go`)不带前缀。
- `pkg/` 只承载无业务语义的通用工具,按职能拆包(`stringutil/`、`timeutil/`、`safego/`);**禁止**新增 `common/` / `goutil/` / `helper/` 聚合包。
- Bean 就近声明,不集中到统一装配文件;初始化顺序交给 IoC 容器,**禁止**用包级单例或全局变量长期持有依赖。

---

## Project Structure Hard Constraints (en)

- The top-level directories are fixed as `idl/` and `internal/`.
- `main.go` only does side-effect imports and IoC startup. It **must not** carry business assembly code; registration logic lives in `init.go` files across layers.
- IDL generated artifacts land in `idl/<protocol>/gen/`. **Do not** hand-edit generated files. **Do not** move generated code into `internal/`.
- `internal/` is domain-layered; follow `docs/domain-layout.md`.

## Naming and Organization (en)

- Package names are lowercase; the package name must not repeat types inside it.
- Files that expose cross-package types carry a domain prefix (`order_controller.go`, `order_service.go`). In-package helper files (`converter.go`, `assembler.go`, `dto.go`) do not.
- `pkg/` only carries business-agnostic utilities, split by function (`stringutil/`, `timeutil/`, `safego/`). **Do not** create umbrella packages like `common/` / `goutil/` / `helper/`.
- Beans are declared next to their implementation, not gathered into a central assembly file. Initialization order is handled by the IoC container. **Do not** use package-level singletons or globals to hold dependencies.
