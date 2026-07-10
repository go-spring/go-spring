# add-grpc 子流程(占位,待补充)

在已有 Go-Spring 项目里新增一个 gRPC 接口:IDL → 生成 → controller/handler → 注册 → 测试 → 验证,与 `add-http` 同形态,区别在协议与生成器。

## 何时使用

- 关键词:加 gRPC 接口 / 新增 grpc 方法 / 改 proto 加 rpc

## 触及的 layout 位置

- `idl/grpc/`(`.proto` 定义)
- `api/controller/<bc>/`(协议无关业务)
- `api/server/grpcsvr/`(handler 聚合与协议适配)

## 工作流程

> **待补充**:此工具尚未编写。填充时复用 `add-http.md` 结构,改用 gRPC 生成器与 `grpcsvr` 入口;遵守 `docs/agent-rules/domain-rules.md` 与 `../../SKILL.md` 共用约定。
