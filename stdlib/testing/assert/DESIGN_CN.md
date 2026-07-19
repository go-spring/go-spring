# assert Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`assert` 是 `stdlib/testing/internal` 共享断言引擎的**薄包装**。它存在的意
义是把"失败继续"模式做成 import 期编译期选择,而不是每次调用传一个 flag。

## 1. 职责与边界

- 提供与 [`require`](../require/) 完全相同的 fluent API,只把
  `fatalOnFailure = false`——失败走 `t.Errorf`(继续)而不是 `t.Fatalf`
  (停止)。
- 拒绝写自己的断言逻辑。所有检查在 `internal`,与 `require` 共享。两个一
  样的包装保证两种模式的行为不可能漂移。

## 2. 关键抽象与缝隙

- **`fatalOnFailure = false` 常量**——让这个包变成 "assert" 而不是
  "require" 的唯一那行。每个入口函数把它传进 internal 构造函数。
- **`internal.TestingT`**——`*testing.T` 的最小接口。接接口让同一套断言
  能跑在真测试、subtest、`testcase` 自测 harness 里。
- **Fluent 值对象**(`*internal.Assertion`、`*internal.ErrorAssertion` ...)
  由入口返回;检查方法链下去,最终以模式的 `fatalOnFailure` 调共享引擎。

## 3. 约束

- **除 `internal` 外零依赖。** 新加依赖会漏进每个模块测试二进制。
- **与 `require` 行为对等。** 这里存在的方法那里必须存在且签名一致;
  `testcase` 套两个入口都跑同一批场景强制对齐。
- **`Panic` 是顶层函数**,不是 fluent 链——目标是回调,不是值。

## 4. 取舍与被否决方案

- **两个薄包 > 一个包 + 运行期 flag。** flag 会让测试在一函数里意外混模式;
  按 import 拆开让每个调用点选择可见。
- **自己实现 > 依赖 testify。** API 有意接近以保留肌肉记忆,实现只用标
  准库。
