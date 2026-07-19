# testcase Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`testcase` 是防止 `assert` 与 `require` 行为漂移的护栏。纯测试包
(`package testcase_test`),把共享断言引擎经两个模式包各跑一遍,任何静默
行为漂移会当场暴露。

## 1. 职责与边界

- 把每族断言(`That` / `Error` / `Number` / `String` / `Slice` / `Map` /
  `Panic`)在 `assert` **与** `require` 两个入口下都跑一遍。
- 验证同一输入在两个入口产生相同的错误信息与整体 fluent 形态,唯一差异就
  是"是否停止测试"。
- 拒绝写生产代码。什么都不导出,连非 `_test.go` 文件都没有。

## 2. 关键抽象与缝隙

- **`internal.TestingT` 伪实现**——套件记录失败输出而不真让外层 test
  失败,这样能对失败**内容**做断言。
- **每族一个测试文件**——与 `internal/*.go` 拆分对齐,某族变更就对应一个
  测试文件。

## 3. 约束

- **无导出符号。** 套件只被 `go test` 发现;扫模块公开 API 的工具在这里
  看不到东西。
- **只依赖 `stdlib/testing/internal` 与两个包装包。** 引第三方会经构建期
  耦合漏进包装包。

## 4. 取舍与被否决方案

- **共享套件 > 各包复制。** 在 `assert` 与 `require` 里各写一份测试,两
  个模式各自演化就必然漂移。同一套用两个入口跑强制对齐。
- **纯测试包,不做 helper 库。** 把伪 `TestingT` 与场景表公开会诱导使用
  方嵌进自己测试,而两模式契约不是给外部这么用的。
