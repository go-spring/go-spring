# wire-starter 子流程

在已有 Go-Spring 项目里接入一个外部组件(Redis / MySQL / Kafka / gRPC 等),优先复用现成 Starter,补齐配置绑定 + Bean 注册 + 生命周期。

## 何时使用

- 用户说「接一下 Redis」「加个 MySQL」「用 gorm」「加 pprof」;
- 用户已在写自定义初始化代码,但项目里其实有对应 Starter。

## 前置检查

- 当前 cwd 位于目标子 module 内(有 `go.mod`)。
- 已知仓库 `starter/` 下可用组件(截至当前):

  | 组件 | Starter | 说明 |
  | --- | --- | --- |
  | Gin HTTP | `starter-gin` | HTTP server |
  | Redis(go-redis) | `starter-go-redis` | 官方 go-redis 客户端 |
  | Redis(redigo) | `starter-redigo` | redigo 客户端 |
  | MySQL(gorm) | `starter-gorm-mysql` | gorm + mysql driver |
  | gRPC | `starter-grpc` | gRPC server/client |
  | Thrift | `starter-thrift` | Thrift server/client |
  | pprof | `starter-pprof` | pprof 端点 |

  用户点的组件不在表内 → 先查 `starter/` 目录确认;仍无 → 提示「无现成 Starter,建议参考已有 Starter 结构自研,或复用同类替代品」,不直接从零写初始化。

## 收集信息

- **组件用途**:业务读写 / 缓存 / 消息 / 追踪等;
- **实例数**:单实例默认;多实例需要 Bean name 区分;
- **配置来源**:走项目已有配置文件(`conf/`)还是环境变量。

## 工作流程

### 1. 加依赖

在目标 module 目录:

```bash
go get go-spring.org/starter/<starter-name>@latest
```

流式输出。

### 2. 引入 Starter

在 module 入口(通常是 `main.go` 或 `init.go`)引入 blank import:

```go
import _ "go-spring.org/starter/<starter-name>"
```

Starter 会在启动期自动注册 Bean + 绑定配置。

### 3. 配置绑定

在 `conf/app.yaml`(或项目使用的配置文件)按 Starter 约定加配置段。配置 key 与结构对齐 Starter 内部 struct tag,不要发明字段。

多实例场景:按 Starter 约定加 name 后缀区分。

### 4. 注入使用

在需要该组件的 controller / service 里,用启动期注入(struct 字段 + 依赖注入 tag),**不**在运行期动态取。

同名同类型 Bean 需要区分时,用条件互斥显式选一个,不靠隐式覆盖。

### 5. 生命周期

Starter 已托管 open/close;业务代码不要重复 Close / 手动管理连接池,除非需求明确。

### 6. 验证

进入 build-test 子流程:gofmt + `go test` +(必要时)`go build`。

## 完成后输出

- 接入的 Starter 名 + 版本;
- 新增依赖 / import / 配置段清单;
- 使用点(哪个 controller/service 注入了它);
- 验证结果;
- 遗留风险(如未配置连接池上限、未覆盖失败重试等)。

## 关键约束

- **禁止**在有 Starter 的情况下手写初始化。
- **禁止**在运行期动态注入 Bean。
- **禁止**改 Starter 内部代码来适配业务;有需求走扩展点或提 issue。
- 配置字段以 Starter 内 struct tag 为准,不要凭记忆写 key。
