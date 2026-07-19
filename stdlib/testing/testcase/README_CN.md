# testcase
[English](README.md) | [中文](README_CN.md)

`testcase` 是把 `stdlib/testing/internal` 里的检查通过 [`assert`](../assert/)
**与** [`require`](../require/) 两个入口都跑一遍的共享测试套。这是纯测试
包(`package testcase_test`),不导出任何代码。要写断言请用 `assert` /
`require`;这个包存在的意义是防止两个入口的行为漂移。

## 目的

所有检查——相等、error 匹配、数值比较、字符串匹配、slice / map 操作、
panic 捕捉——都在一个地方(`internal`)。把同一批场景经 `assert`(失败继
续)与 `require`(失败停)都跑一遍,套件保证:

- 两个入口有同样的方法与同样的签名。
- 两个入口给出的失败信息格式一致。
- 唯一差异只是"测试是否立即停止"。

## 组织

六个文件,每族断言一份:

| 文件 | 覆盖 |
|------|------|
| `assert_test.go` | 泛型 `That` 与 `Panic` |
| `error_test.go`  | `Error`(`Is` / `Matches` / `String` ...) |
| `number_test.go` | `Number[T]` |
| `string_test.go` | `String` |
| `slice_test.go`  | `Slice[T]` |
| `map_test.go`    | `Map[K,V]` |

## 执行

```
go test ./stdlib/testing/...
```

没有对外可 import 的 API。
