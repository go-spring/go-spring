# gs 设计说明
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`gs` 是 Go-Spring 四层栈（stdlib → spring → starter → gs）里工具层的
Toolkit Manager 二进制。它有三件事：从 `layout/` 超集脚手架化新项目、从
项目 `idl/` 生成代码、生成 Kubernetes 部署脚手架。任何未内建的子命令都
被转发到同目录下的 `gs-<tool>` 二进制（外部工具协议）。

## 1. 职责与边界

- **内建子命令**（`gs/gs/main.go` 的 `builtins` map）：
  - `gs init` — sparse-checkout 克隆最新 `layout/vX.Y.Z` tag、按语言选
    择裁剪、按 feature 裁剪、替换 `GS_PROJECT_*` 占位符，然后跑
    `gs gen`。
  - `gs gen` — 从 `gs.json` 标识的项目根走 `idl/`，把每个协议子目录派
    发到对应生成器（目前 `idl/http` → 通过 `cmd/proto` 走 `gs-http-gen`）。
  - `gs add` — 从 `gs.json` 里锁定的 layout tag 里拷贝额外的 feature 切
    片到现有项目。
  - `gs k8s` — 把 Kubernetes 部署脚手架渲染到当前项目。
  - `gs go` / `gs serve` — `go build`/`go run` 的开发循环包装。
- **外部工具** — 任何不在 `builtins` 里的 `gs <name>` 执行同目录下的
  `gs-<name>`（见 `gs/gs/tool/tool.go`）。`gs-mock`、`gs-http-gen`、
  `gs-gui` 都走这个协议。
- CLI **不**在应用运行期跑。它产出的任何文件（Dockerfile、manifest、
  `conf/app-k8s.properties`、layout 文件）都是可编辑的起点，绝不是被
  生成项目的运行时依赖。

## 2. 关键抽象与接缝

- **编译进二进制的 feature manifest**。`gs init` 的 feature flag 全来自
  `cmd/feature/features.json`，通过 `cmd/feature/embed.go` 里的
  `//go:embed features.json` 编译进来。这是唯一真源：cobra 必须在 argv
  解析前注册 flag，所以 manifest 必须被烘进二进制；增删 feature = 改
  JSON + 重新构建 `gs`。这份 manifest 必须与它裁剪的 layout 超集保持
  同步。
- **超集 + 裁剪模型**。远端 `layout/` 是完整"全家桶"：每种 IDL、每种
  server、每种 controller 变体、每种 starter blank import 都在。
  `gs init` 克隆它（sparse-checkout、`--filter=blob:none`、`--depth 1`、
  `--branch layout/vX.Y.Z`），按 `--lang` 剥离 `.en`/`.zh` 语言后缀，
  然后 `feature.Prune` 删掉用户没选的部分。占位符替换
  （`GS_PROJECT_MODULE`、`GS_PROJECT_NAME`、`GS_PROJECT_LANG`、
  `GS_LAYOUT_VERSION`）按 key 长度降序进行，防止短 key 覆盖含它作为前
  缀的长 key。
- **feature flag 规则**（见 `cmd/feature/feature.go` 和 memory
  `project_gs_init_feature_flags.md`）。裸 flag = 默认切片；同一 flag
  key 命名同一竖切片（idl + server + controller 变体 + converter +
  `init.go` import + starter）。只有多协议框架带协议后缀
  （`--kitex-thrift`、`--kitex-pb`）。运行期配置（addr、db、连接池大
  小）永远不会成为 flag——那些进生成的 `conf/`。feature 严格独立，无
  跨 feature 依赖机制。
- **K8s 模板编译进二进制**。`cmd/k8s/embed.go` 用
  `//go:embed all:templates`——`all:` 前缀是必要的，才能带上
  `.dockerignore` 这种以 `.` 开头的 dotfile（默认 `embed` 会跳过 `.`
  或 `_` 开头的名字）。`k8s.Write` 把每个模板渲染到项目里，按 key 长
  度降序做占位符替换，除非 `--force` 否则跳过已有文件。替换的
  key 包含 `GS_APP_NAME`（由 `toDNS1123(moduleLeaf)` 派生）、
  `GS_APP_PORT`（`--port`，默认 9090）、`GS_MGMT_PORT`（写死 9370）、
  `GS_IMAGE`。
- **离线生成**。feature manifest 和 k8s 模板都不在运行时下载；
  `gs init` 还需要网络（git clone），但 `gs k8s` 与 `gs add` 完全依赖
  编译进来的数据 / 本地项目文件。
- **verbosity 契约**（见 `gs/CLAUDE.md`）。每个入口都通过
  `internal/runcmd.BindFlag` 绑定共享的 `-v` 计数 flag。等级 0 只打
  `[INFO]` 步骤行；`-v` 还打 argv；`-vv` 直连转发子进程的 stdout/stderr。
  这不是可选项——它是调试接缝，不允许被"简化"掉。

## 3. 约束

- 编进二进制的 `features.json` 必须与远端 `layout/` 超集保持同步。漂
  移意味着用户手里的 flag 指向 layout 里已经不存在的路径。
- 占位符替换发生在 layout 原始字节上、在 `gs gen` 之前，所以 manifest
  里 `Owns` 路径与 init-import 行都要用 `GS_PROJECT_MODULE` 这个 token
  引用——不要在裁剪前提前重写它们。
- 生成的 `deploy/` 模板与 Dockerfile 不加 Apache header（与 layout 的
  Makefile / docker-compose 对齐）；只有生成的 `.go` 文件加 header。
- K8s 探针接线（9370 上的 `/startup`、`/health`、`/readiness`，
  `preStop sleep 5`，`terminationGracePeriodSeconds 30`）刻意与
  `stdlib/actuator` 及框架级 graceful drain 默认值对齐；单独改模板值
  会静默偏离 shutdown 时序。

## 4. 权衡与被否决的方案

- **拒绝拼装组件、选择超集裁剪**。曾考虑用独立块拼装项目——feature 间
  的相互作用（init.go import、blank import、配置键）会组合爆炸。裁剪
  一份超集，可以保证每一步中间产物都是可运行的项目。
- **拒绝运行时拉 manifest**。cobra 需要在 `RegisterFlags` 期拿到 flag
  集合，那时 argv 还没解析，网络更没起作用。编进二进制是唯一稳的做法。
- **拒绝 feature 自动组合**。feature 按约定就是独立的、没依赖边。这让
  `--list-features` 保持扁平菜单，也吻合"从全家桶里剪出来"的心智模型。
