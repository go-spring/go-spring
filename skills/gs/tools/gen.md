# gen 子流程

在已有 Go-Spring 项目里重新生成派生代码(HTTP / mock 等),对齐 `gs gen` 的行为但**直接调底层生成器**,不依赖 `gs` 命令。

## 何时使用

- 用户说「重新生成」「IDL 改了跑一下生成」「`gs-http-gen`」「`gs-mock`」;
- 改过 `.idl` 但生成物没更新,或生成物被误删需重建。

## 前置检查

- 定位到目标子 module(有 `go.mod`);不在则先定位。
- 对应生成器在 PATH:
  - HTTP:`gs-http-gen`,缺失提示 `go install go-spring.org/gs-http-gen@latest`;
  - mock:`gs-mock`,缺失提示对应安装命令。
- 目标 IDL 目录存在(如 `idl/http/`),否则说明该协议无生成任务,跳过不报错。

## 工作流程

### 1. 确定生成范围

- 默认只重生成用户指明或近期改动涉及的协议目录;
- `idl/grpc`、`idl/thrift` 目前无对应生成器,跳过不报错。

### 2. HTTP 代码生成

进入 `idl/http/` 目录(生成器从 cwd 读 `.idl`),流式执行:

```bash
rm -rf proto && mkdir -p proto
gs-http-gen --server --output proto
```

失败保留现场,不清理(错误包装遵循项目 `errutil` 约定)。

### 3. mock 生成(按需)

仅当用户要求或项目有 mock 约定时,按 `gs-mock` 既有用法生成到约定目录,同样流式输出、失败保留现场。

### 4. 验证

在子 module 根目录流式执行:

```bash
gofmt -l -w .
go build ./...
```

生成物编译不过 → 停下交出错误,常见是 IDL 与手写代码不匹配(转 fix-compile 阶段定位)。

## 完成后输出

- 重新生成的协议目录与生成器版本;
- 变更的生成物路径;
- 跳过的协议目录(及原因);
- 验证结果。

## 关键约束

- **禁止**手改生成物(`idl/http/proto/` 等);要改改 `.idl` 源再重生成。
- **禁止**依赖 `gs` 命令,一律走底层生成器(`gs-http-gen` / `gs-mock`)。
- 生成前 `rm -rf` 目标目录再重建,避免陈旧文件残留。
