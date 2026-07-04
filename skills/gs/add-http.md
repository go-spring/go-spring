# add-http 子流程

在已有 Go-Spring 项目里新增一个 HTTP 接口,覆盖 IDL → 生成 → handler/service → 注册 → 测试 → 验证 全链路。

## 何时使用

- 用户说「加一个 HTTP 接口」「新增 API」「按 xxx 查询 yyy」;
- 用户改了 IDL 但没跑生成、没接 handler。

## 前置检查

- 当前 cwd 位于目标子 module 内(有 `go.mod`);不在则先定位。
- `gs-http-gen` 在 PATH;缺失时提示 `go install go-spring.org/gs-http-gen@latest`。
- `idl/http/` 目录存在(否则该项目不是 HTTP 服务,终止并说明)。
- 分层符合 layout 约定:`api/controller` 承载协议无关业务,`api/server/http/handler` 承载协议适配。

## 收集信息

用 `AskUserQuestion` 补齐(用户已提供的跳过):

- **接口语义**:方法(GET/POST/...)、路径、入参出参字段。
- **归属模块**:属于哪个 domain / controller,若项目多 controller 需明确。
- **鉴权 / 中间件**:是否需要现有中间件链。
- **错误码**:复用 `consts/errno` 已有码或新增(新增需说明)。

## 工作流程

### 1. 改 IDL

- 在 `idl/http/` 下对应 `.idl` 文件里增补 method 定义;命名跟随现有风格。
- 入参出参 struct 复用现有类型优先,必要时新增。
- **禁止**手写生成物,只改 `.idl` 源文件。

### 2. 重新生成代码

进入 `idl/http/` 目录,流式执行:

```bash
rm -rf proto && mkdir -p proto
gs-http-gen --server --output proto
```

生成失败(`errutil.Explain(err, "run gs-http-gen")`)保留现场,不清理。

### 3. 实现 controller(协议无关)

在 `api/controller/<模块>.go`(或 domain 变体下对应位置)新增方法:

- 入参出参使用生成的 DTO 或领域类型。
- 业务逻辑走已有的 service / repository;缺失能力时先在下层补。
- 错误用 `errutil.Explain` / `errutil.Stack` 包装,业务错用 `consts/errno` 的错误码。

### 4. 组装 handler(协议适配)

`api/server/http/handler.go` 通过嵌入聚合 controller,通常无需改动;仅当引入新 controller 时才加嵌入字段。**不要**把方法体写进 handler(见项目约定)。

### 5. 注册路由

生成代码通常已在 `proto/` 里给出路由注册函数,确认已被 handler 初始化流程引用。若是全新 controller,补 Bean 注册(启动期注入)。

### 6. 补测试

- controller 层:单测走真实依赖或轻量 fake,不要 mock 数据库(见项目约定)。
- handler 层:必要时补 HTTP 集成测试,覆盖正常路径 + 至少一条失败路径(参数校验 / 业务错)。

### 7. 验证

在子 module 根目录流式执行:

```bash
gofmt -l -w .
go test ./...
go build ./...
```

任一失败直接终止,交出错误定位。

### 8. 文档同步

- 若项目有接口清单(如 `docs/api/*.md`),补该接口的语义、入参出参、错误码。
- 若示例目录 `examples/` 有对应场景,补一条最小调用示例。

## 完成后输出

- 新增/修改文件清单(按 IDL / 生成物 / controller / handler / test / docs 分组);
- 新增接口的 method + path + 关键字段;
- 验证命令与结果;
- 遗留风险(如未覆盖的失败路径、待补的鉴权等)。

## 关键约束

- **禁止**手改 `idl/http/proto/` 下的生成物。
- **禁止**跳过 `gs-http-gen` 直接手写 handler。
- controller 与 handler 分层不合并,单文件 handler 不是扩展性问题(项目约定)。
- 每次改动收敛到 gofmt + test + build 三连验证。
