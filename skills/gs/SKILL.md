---
name: gs
description: 使用 gs CLI 创建或改造 Go-Spring 项目骨架。当用户提到用 gs 初始化项目、生成 layout、添加模块时触发。
version: v0.0.1
---

# /gs Skill

面向 Go-Spring 项目全生命周期的 AI Skill。不是 CLI 外壳,而是把「起项目 → 加接口 → 接组件 → 验证 → 交付 → 沉淀」串起来的统一入口。主文档只做导航与共用约束,具体动作全部下沉到子流程文档。

## 子流程导航

根据用户意图分发,**必须先读取对应子流程文档再执行**,不要凭记忆执行:

| 子流程 | 关键词 | 文档 |
| --- | --- | --- |
| 起项目骨架 | 新建 / 初始化 / `gs init` / 切换 layout | [`init.md`](init.md) |
| 新增 HTTP 接口 | 加接口 / 新增 API / 改 IDL 加路由 | [`add-http.md`](add-http.md) |
| 重新生成代码 | 重新生成 / IDL 改了 / `gs-http-gen` / `gs-mock` | [`gen.md`](gen.md) |
| 接入组件(Starter) | 接 Redis / 接 MySQL / 接 Kafka / 加 Starter | [`wire-starter.md`](wire-starter.md) |
| 构建 / 测试 | 编译一下 / 跑测试 / gofmt | [`build-test.md`](build-test.md) |
| 编译报错定位 | 编译报错 / 修一下 / test 挂了 | [`fix-compile.md`](fix-compile.md) |
| 补测试 / 示例 | 补测试 / 加用例 / 加 example | [`add-test.md`](add-test.md) |
| 变更摘要 / 交付 | 出变更说明 / 交付 / release note | [`release-note.md`](release-note.md) |
| 沉淀约定 | 沉淀 / 回写文档 / 更新 skill | [`sink-context.md`](sink-context.md) |

## 子流程通用约束

各子流程文档必须自行落实以下要求,不要跳过:

### 上下文与结构

- **上下文优先**:执行前先读 `AGENTS.md` / `CLAUDE.md` / `CODING_STYLE.md` / 目录约定,不凭记忆假设结构。
- **多 module 感知**:仓库根目录不一定有 `go.mod`;`go build`/`go test`/`gofmt` 要下钻到具体子 module 目录。
- **layout 约定不质疑**:`job` vs `mqsvr`、`api/controller` vs `api/server/<proto>/handler` 等目录划分是项目约定。

### 组件与依赖

- **Starter 优先**:接入外部组件先查 `starter/` 有无现成 Starter(gin / go-redis / gorm-mysql / grpc / thrift / pprof / redigo),能复用就复用,不从零写初始化。
- **启动期注入**:IoC 在启动期完成,不引入运行期动态注入。
- **Bean 冲突显式化**:同名同类型 Bean 不靠隐式覆盖,用条件互斥显式选。

### 执行与验证

- **前置检查**:所需二进制在 PATH 中、目标目录状态符合预期、外部服务可达;不满足直接终止。
- **入参校验**:module path、layout、语言、接口名等参数在动手前完成合法性校验,非法值立即报错。
- **冲突检测**:目标目录/文件已存在等冲突直接终止,不覆盖不删除。
- **流式输出**:所有子进程调用 stdout/stderr 直接接 `os.Stdout/Stderr`;仅解析输出时才 buffer。
- **错误包装**:失败用 `errutil.Explain` / `errutil.Stack` 包装,保留现场,不静默清理。
- **改动收敛**:每次代码改动后跑 gofmt + `go test`(必要时 `go build`),给出验证结论。

### 交付

- **变更摘要**:代码/配置/接口/文档改动完成后,输出变更清单 + 验证命令 + 遗留风险。
- **文档同步**:接口、配置、示例的改动必须同步到对应文档,不留事后补。
