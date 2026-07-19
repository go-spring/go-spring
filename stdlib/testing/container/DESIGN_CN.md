# container Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`container` 是 testcontainers 风格的 helper:在 Go 测试内启一个真依赖
(redis / mysql ...)容器,测试结束时自动拆除。它在 stdlib 层,不能 import
Docker SDK。

## 1. 职责与边界

- 按 `Request` 启容器、等就绪(日志子串 / TCP 端口 / 自定义 wait 策略)、
  在 test 上挂 cleanup,并把动态映射出来的 host:port 交回。
- 在 `presets.go` 里给出常见镜像(Redis / Postgres / MySQL ...)的现成
  `Container` 值——一行就够,不用列一堆字段。
- 拒绝做容器 SDK。缝隙就是本地 `docker` CLI,包只组命令、解析文本。
- 拒绝静默"没 Docker"。`SkipIfNoDocker(tb)` 是显式 skip 助手——测试忘调
  就在第一次 `docker version` 失败时 `Fatalf`,比静默通过响得多。

## 2. 关键抽象与缝隙

- **`TB` 接口,不直接绑 `*testing.T`。** 暴露窄 `Helper/Cleanup/Fatalf/
  Logf/Skipf` 接口,让 stdlib 包不必无条件 import `testing`,也让调用方
  能塞伪实现。
- **`docker` CLI 就是缝隙。** 所有操作走 `os/exec` shell-out。代理变量从
  环境继承,与仓库现有 check.sh 约定一致。
- **`Wait` 策略按镜像插入。** preset 给出该镜像需要的等待策略(redis 看日
  志、mysql 探端口),测试不必知道具体的就绪条件。

## 3. 约束

- **stdlib 零依赖规则。** 只用 `net` / `os/exec` / `bytes` / `strings` /
  `time` 以及包内互引。不用 Docker SDK / testcontainers-go。
- **不要求 compose-v2。** 仓库 Docker 约定就是纯 `docker run`,不用
  `docker compose` 子命令(见环境记忆)。本 helper 跟随。
- **Cleanup best-effort。** 移除错误只 log 不 fatal——cleanup 跑时测试早
  已成功 / 失败结束。
- **不跨测试共享容器。** 每个 `Run` 起一个自己的容器。跨测试共享会把
  cleanup 归属搞乱,刻意不做。

## 4. 取舍与被否决方案

- **shell-out `docker` > import `moby/moby` / testcontainers-go。** SDK 会
  把庞大依赖树漏进每个 stdlib 使用者的 `go.sum`。shell-out 便携,而且与仓
  库已有跑 Docker 的方式一致。
- **`SkipIfNoDocker` 显式 skip > `Run` 里隐式 skip。** 隐式 skip 会让配错
  的 CI 静默通过所有集成测试。要求显式调用把选择摆到明面。
- **内建 preset > 镜像索引。** Go 侧维护镜像索引会变成蹩脚的 dockerhub 镜
  像,几只精挑的 preset 就够 stdlib 使用者用了,其他情况用一句
  `container.Run(t, container.Request{...})` 就行。
