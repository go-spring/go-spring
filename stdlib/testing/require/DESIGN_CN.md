# require Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`require` 是 `stdlib/testing/internal` 共享断言引擎的**薄包装**,与
[`assert`](../assert/) 唯一差异是失败走 `t.Fatalf` 而非 `t.Errorf`。它存
在的意义是把 fail-fast 的选择做到 import 期可见。

## 1. 职责与边界

- 提供与 `assert` 完全同款 fluent API,只把 `fatalOnFailure = true`。
- 拒绝写自己的断言逻辑。检查在 `internal`,与 `assert` 共享。按包装拆保
  证两种模式的检查不可能漂移。

## 2. 关键抽象与缝隙

- **`fatalOnFailure = true` 常量**——让这个包变成 "require" 而不是
  "assert" 的唯一那行。每个入口函数透传它。
- **`internal.TestingT`**——每个入口接的 `*testing.T` 最小接口。
- **Fluent 值对象**(`*internal.Assertion` 等)与 `assert` 完全同款——
  签名上两个模式互为替换。

## 3. 约束

- **除 `internal` 外零依赖。**
- **与 `assert` 行为对等。** `testcase` 套两个入口跑同一批场景强制对齐。
- **`t.Fatalf` 只停当前 test。** 通过 `require` 失败的 `t.Run` subtest 停
  掉自己;父 test 可以自己决定继不继续。

## 4. 取舍与被否决方案

- **两个薄包 > 运行期 flag。** 把 fail-fast 做成 import 期选择,避免读代
  码时还得回溯是否有 `SetMode(...)` 把行为翻过。
- **自己实现 > 依赖 testify。** API 有意接近保留肌肉记忆,实现只用标准
  库。
