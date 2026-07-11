<!-- 本文档为通用目录结构片段，用于嵌入 domain-layout.md 中「## 通用目录结构」段落。可单独阅读，但标题与副标题由宿主文档提供。 -->

以下顶层骨架适用于所有 Go-Spring 单服务项目。`idl/` 与 `internal/` 是固定的顶层目录，`internal/` 内部采用 domain 分层，详见后续小节。

### 顶层目录

```
.
├── conf/               # 运行期配置（app.properties 等）
│   └── app.properties
├── docs/               # 项目文档
├── idl/                # IDL 定义，按协议分子目录
│   ├── http/           # HTTP 接口定义
│   ├── grpc/           # gRPC proto
│   └── thrift/         # Thrift IDL
├── internal/           # 服务内部代码，采用 domain 分层，详见后续小节
├── logs/               # 运行时日志目录（占位）
├── public/             # 静态资源（占位）
├── main.go             # 程序入口，只做装配与生命周期启动
├── gs.json             # Go-Spring 项目元信息（名称、版本等）
├── go.mod
├── go.sum
└── README.md
```

### 顶层目录职责

| 目录 / 文件 | 职责 |
|---|---|
| `conf/` | 运行期配置文件，与代码解耦，按环境替换。 |
| `docs/` | 项目文档，含骨架与分层说明，不进入构建产物。 |
| `idl/` | 对外协议契约，按协议（`http` / `grpc` / `thrift`）分子目录，不放实现代码。 |
| `internal/` | 服务内部代码，`internal` 语义禁止外部 import，采用 domain 分层。 |
| `logs/` | 运行期日志目录占位，由日志组件写入。 |
| `public/` | 静态资源（前端产物、模板等）占位，由 HTTP 层暴露。 |
| `main.go` | 程序入口，只做依赖注入与生命周期启动，不承载业务逻辑。 |
| `gs.json` | Go-Spring 项目元信息（名称、版本等）。 |

### 注意事项

- **`main.go` 与 `internal/init.go` 分工**：`main.go` 只负责 import `internal/...` 触发注册链、启动 IoC 容器并驱动生命周期；实际的路由 / 任务 / 消费者注册由各层 `init.go` 通过 side-effect import 完成，`main.go` 不承载任何业务装配代码。
- **IDL 生成产物落位**：`idl/` 只放协议契约源文件（`.thrift` / `.proto` / HTTP IDL）；生成的 Go 代码（stub、client、model）回写到 `idl/<protocol>/gen/` 下，与源文件同目录管理，不落到 `internal/`。业务层通过 import `idl/<protocol>/gen/...` 使用生成物，禁止手工修改生成文件。
