# testing Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`testing` 是 Go-Spring 各模块自己写测试用的零依赖断言库。它给出了 Java 里
AssertJ + JUnit assumptions 那两档能力的对齐(`assert` / `require`),并加了
两个高层 helper——`container`(testcontainers 风)和 `contract`(Spring
Cloud Contract 风)。它待在 stdlib 层,业务运行时不依赖它;整个包图只用标
准库。

## 1. 职责与边界

- 提供 fluent、按类型分族的断言 API(`That` / `Error` / `Number` /
  `String` / `Slice` / `Map` / `Panic`),替代 `stretchr/testify` 与
  `gomega`——不引第三方。
- 把"失败停 / 失败继续"两种模式拆到两个薄包(`require` / `assert`),共
  享 `internal` 一套实现;写测试时按 import 选行为。
- 提供 testcontainers 风格的 Docker helper(`container`),用本地 `docker`
  CLI shell-out,不把 Docker SDK 拉进 stdlib。
- 提供 JSON 契约测试对(`contract`):生产者 `Verify`、消费者 `StubServer`,
  两侧读同一份契约文件。

## 2. 关键抽象与缝隙

- **`internal.TestingT` 接口。** 断言库需要的 `*testing.T` 最小接口。所有
  断言函数都接它,所以同一套库在真 `*testing.T`、subtest、以及 `testcase`
  用来自测的伪 harness 里都能跑。
- **`fatalOnFailure` bool。** `assert` 与 `require` 唯一的行为差异。两个薄
  包设置它然后转派;fluent API 与检查逻辑全部在 `internal`。
- **`testcase` 包 = 共享断言测试套。** `package testcase_test`,把共享引
  擎在两个入口都跑一遍,防止 `assert` 与 `require` 检查不一致。刻意不导出。
- **`container` shell-out 到 `docker`**,不用 Docker SDK——缝隙就是本地
  docker CLI。`presets.go` 内置 Redis / Postgres 等常用容器作为 `Container`
  字面量。
- **`contract` 三文件**——`contract.go` 定义 JSON 结构;`verify.go` 对
  真生产者跑;`stub.go` 起消费者侧 stub server。三份代码读同一 JSON,契约
  文件就是唯一 source of truth。

## 3. 约束

- **`stdlib/testing` 及其子包只能 import 标准库**(以及自己人 + `stdlib/
  errutil` 让共享引擎里错误格式统一)。任何其他依赖都会漏进每个模块的测
  试二进制。
- **`internal` 不导出**——机制是共享的,但对外可调 API 必须走两个模式包,
  让 fail-fast / continue 的选择在调用点显式。
- **`container` 需要 PATH 上有可用的 `docker` CLI。** 找不到 CLI 是测试
  skip(在 `container` DESIGN 里说明),不是静默通过。
- **`contract` matcher 保持最小。** JSON 结构相等 + 正则提示——够与 Java
  Spring Cloud Contract 生产者互操作即可;更花哨的 matcher 等实际场景。

## 4. 取舍与被否决方案

- **自己实现 > 依赖 testify。** 两模式 fluent 断言足够简单,自己扛掉了一
  个每个 stdlib 用户必须带的第三方依赖。API 有意接近 testify(肌肉记忆),
  但实现是我们的。
- **Shell out `docker` > import `moby/moby` / Go SDK。** Testcontainers-Go
  对下游没问题,但 SDK 会漏进每个 stdlib 用户的 `go.sum`。Shell-out 便携,
  也把在做什么明说。
- **JSON 契约 > Go DSL。** Go DSL 与 Java 生产者互操作不了,违背"消费者
  驱动"前提。纯 JSON 是互通格式。
- **共享 `testcase` > 各包复制测试。** 各自复制一份必然漂移;把同一套用两
  个入口跑一遍强制对齐。
