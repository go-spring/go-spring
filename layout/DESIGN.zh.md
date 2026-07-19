# GS_PROJECT_NAME 设计说明
[English](DESIGN.en.md) | [中文](DESIGN.zh.md)

这是 Go-Spring 单仓 `layout/` 项目脚手架的设计说明。`layout/` 不是可运
行的应用——它是被 `gs init` sparse-checkout 克隆、按语言剥离、按
feature 裁剪、按占位符替换后生成用户项目的源模板。

## 1. 职责与边界

- **是起点，不是框架**。`layout/` 下的每一样都被期望在 `gs init` 之后
  被用户改动。模板的职责是让首次提交合理：正确的 module 布局、真实的
  配置键、能跑通的 `gs.Run()` 接线、每种协议的 IDL + server + init 胶
  水。
- **只保留 domain 分层**。`internal/` 固定为 `api/`、`application/`、
  `domain/`、`infra/`、`pkg/`、`consts/` 加 `init.go`。没有 `--layout`
  flag、没有 MVC / modulith 变体目录、没有形态占位符——domain 是唯一
  支持的分层形态，这是有意的。
- **feature 超集**。模板出货每种框架/协议 server
  （`internal/api/server/*svr`）、每种 IDL 家族（`idl/*`）、
  `internal/init.go` 里每一个 starter 的 blank import。`gs init` 把
  用户没选的裁掉。下游贡献者是扩超集，不是新增"模式"。

## 2. 关键约定与接缝

- **语言变体**。以 `.en.md` / `.zh.md` 结尾的文件是源；
  `gs init --lang <lang>` 剥离后缀，用户项目里就变成 `AGENTS.md`、
  `README.md` 等。模板内部指向 `common-rules.md` / `domain-rules.md`
  这类**不带**语言后缀的链接是正确的——它们在剥离之后才被解析。不要
  把它们"修"成带 `.en` / `.zh`。唯一有意的例外是
  `coding-style.{en,zh}.md` 的互链，按用户决定保留后缀。本模板的
  README 组保持同样的后缀规则（`README.en.md` / `README.zh.md`），本
  DESIGN 组按同样约定成对（`DESIGN.en.md` / `DESIGN.zh.md`）。
- **每种协议维持独立 IDL 体系**。每个 RPC 框架各自独立的 IDL + 代码生
  成 + 原生类型。Controller 消费框架生成的原生类型（不是应用 DTO），
  只在 application service 层统一。这就是为什么 layout 会并排出货
  `pb/`、`kitex_gen/`、`goctl` 输出，而不是搞一棵"协议中立"的 DTO 树。
  任何"给其它协议套 HTTP 壳"的路数都被否掉了。
- **框架-协议命名与拆分**。
  - **`idl/` 目录用 dash 分隔框架与协议**：`idl/goframe-grpc`、
    `idl/kitex-thrift`、`idl/kratos-ws`。`idl/` 不是 Go 包目录（真正的
    Go 代码在 `pb/`、`kitex_gen/` 子目录），所以 dash 可用又更易读。
  - **server 包目录保持 Go 惯用连写 + `svr` 后缀**：`goframegrpcsvr`、
    `kitexthriftsvr`、`kratoswssvr`。Go 包名不能含 dash。
  - **单协议框架不带协议后缀**：trpc 只有一种协议，就叫 `idl/trpc` +
    `trpcsvr`，不写 `trpc-xxx`。
  - **多协议框架必须拆分**：kitex 拆成 `kitex-thrift` + `kitex-grpc`，
    配置前缀也带协议段（`spring.kitex.thrift.server` /
    `spring.kitex.grpc.server`）。
  - **每个 server 使用独立端口**，即使对应 starter 是桩，端口也在
    `conf/app.properties` 里写死。整个 layout 端口全局唯一。
- **占位符**。`GS_PROJECT_MODULE`、`GS_PROJECT_NAME`、`GS_PROJECT_LANG`、
  `GS_LAYOUT_VERSION`——`gs init` 按 key 长度降序替换。模板文件里
  用字面 token 引用它们；不要在裁剪前提前替换。

## 3. 约束

- layout 文件（Makefile、`docker-compose.yml`、`.yaml`、`.properties`、
  markdown）**不加** Apache header——这与仓库中所有非 `.go` 文件的约定
  一致。只有生成的 `.go` 文件加 header。
- 不引入 `--layout` / `-mvc` / `-modulith` 变体。框架 README 里说的
  "modulith" 指 Go-Spring 的模块化理念，不是本模板的分层选项。
- 不引入跨协议共享的 IDL 树。每种协议自带自己的 IDL、生成器、原生类
  型。
- `conf/app.properties` 里分配的端口不允许重叠——这个不变量的守护点在
  layout，不在 starter。

## 4. 权衡与被否决的方案

- **拒绝拼装式脚手架**。"自选组件搭 layout" 会让 init-import、feature
  flag、配置键的兼容矩阵爆炸。裁剪超集这条路让每个中间态都是可运行的
  项目。
- **拒绝多分层形态**（MVC / modulith / domain 选择）。domain 是唯一
  支持的分层（2026-07 用户决定）。移除选择项也一并拆掉了形态占位符与
  `stripLayoutSuffix` 机制。
- **拒绝给非 HTTP 协议套 HTTP 壳**。让所有框架共用 HTTP DTO 树，就丢
  掉了各协议独立生态的意义。
