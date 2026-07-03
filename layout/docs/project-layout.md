<!-- 本文档为通用目录结构片段，用于嵌入 <form>-layout.md 中「## 通用目录结构」段落。可单独阅读，但标题与副标题由宿主文档提供。 -->

以下顶层骨架与形态无关，`<form>` 表示当前项目形态标识（如 `domain`、`mvc`、`modulith`）。文档中使用 `idl-<form>/` 与 `internal-<form>/` 只是为了标注"这两个目录的分层受形态影响"；**生成的实际目录不带后缀**，最终就是 `idl/` 与 `internal/`。形态由 `gs.json` 中的形态字段决定，不同形态下 `internal/` 的内部分层可能差异较大，顶层结构保持一致。

### 顶层目录

```
.
├── conf/               # 运行期配置（app.properties 等）
│   └── app.properties
├── docs/               # 项目文档
├── idl-<form>/         # IDL 定义，按协议分子目录
│   ├── http/           # HTTP 接口定义
│   ├── grpc/           # gRPC proto
│   └── thrift/         # Thrift IDL
├── internal-<form>/    # 服务内部代码，形态决定分层结构，详见后续小节
├── logs/               # 运行时日志目录（占位）
├── public/             # 静态资源（占位）
├── main.go             # 程序入口，只做装配与生命周期启动
├── gs.json             # Go-Spring 项目元信息（名称、版本、形态等）
├── go.mod
├── go.sum
└── README.md
```

### 顶层目录职责

| 目录 / 文件 | 职责 |
|---|---|
| `conf/` | 运行期配置文件，与代码解耦，按环境替换。 |
| `docs/` | 项目文档，含骨架说明与形态说明，不进入构建产物。 |
| `idl-<form>/` | 对外协议契约，按协议（`http` / `grpc` / `thrift`）分子目录，不放实现代码。 |
| `internal-<form>/` | 服务内部代码，`internal` 语义禁止外部 import，内部分层由形态决定。 |
| `logs/` | 运行期日志目录占位，由日志组件写入。 |
| `public/` | 静态资源（前端产物、模板等）占位，由 HTTP 层暴露。 |
| `main.go` | 程序入口，只做依赖注入与生命周期启动，不承载业务逻辑。 |
| `gs.json` | Go-Spring 项目元信息（名称、版本、形态等）。 |

### 注意事项

- **`main.go` 与 `internal/init.go` 分工**：`main.go` 只负责 import `internal/...` 触发注册链、启动 IoC 容器并驱动生命周期；实际的路由 / 任务 / 消费者注册由各层 `init.go` 通过 side-effect import 完成，`main.go` 不承载任何业务装配代码。
- **IDL 生成产物落位**：`idl/` 只放协议契约源文件（`.thrift` / `.proto` / HTTP IDL）；生成的 Go 代码（stub、client、model）回写到 `idl/<protocol>/gen/` 下，与源文件同目录管理，不落到 `internal/`。业务层通过 import `idl/<protocol>/gen/...` 使用生成物，禁止手工修改生成文件。

### 形态标识

同一项目只保留一种形态。`<form>` 仅用于文档标注，**生成的实际目录不带 `-<form>` 后缀**：

- 形态候选：`domain`、`mvc`、`modulith`（按同一规则扩展）
- 文档写法：`idl-<form>/`、`internal-<form>/`
- 生成结果：统一为 `idl/`、`internal/`
- 形态字段仅影响 `internal/` 的内部分层，不影响顶层目录名
