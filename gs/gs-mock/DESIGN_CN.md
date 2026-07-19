# gs-mock 设计说明
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`gs-mock` 是四层栈（stdlib → spring → starter → gs）工具层里 `gs` 的
外部工具。它为接口、普通函数、结构体方法生成类型安全的 Go mock 代码，
原生支持泛型。

## 1. 职责与边界

- 解析目标目录里的 Go 源码，发现被选中的接口/函数/方法，产出对接
  `gsmock` 运行时（`go-spring.org/gs-mock/gsmock`）的 mock 代码。
- 提供两种测试作者可用的 API：`Handle` 模式（回调完全接管调用）与
  `When … Then / Return` 模式（声明式期望）。
- 不触碰容器。生成的 mock 是普通 Go 值，测试里手工注入即可；与
  `spring/` 无耦合。

## 2. 关键抽象与接缝

- **外部工具协议**。二进制名 `gs-mock`，与 `gs` 并排放置。
  `gs-mock --version` 打印两行 description + version；`gs mock` 通过
  `tool.Call` 派发。
- **运行时库（`gsmock`）**。生成代码对接固定、由泛型撑起的类型安全
  API（最多 7 参数、4 返回）。这是 CLI 写入的目标接缝；两边必须一起
  发布。
- **过滤语法**。`-i "Reader,Writer"` 包含这些；`!` 前缀排除
  （`-i "!Logger"`）。为空则默认扫描包内所有接口。
- **输出路由**。`-o path.go` 写入磁盘；不给 `-o` 则流式打到 stdout，
  方便 `go generate` 里管道使用。

## 3. 约束

- 源接口一变就得重跑；这个文件不被 `go generate` 隐式带上，工具就是权
  威写入方。
- 泛型参数上限（7 参数 / 4 返回）由 `gsmock` 运行时 helper 决定；扩展
  上限前先扩 `gsmock`。

## 4. 权衡与被否决的方案

- **不采用 `gomock` / `mockgen`**。它们早于 Go 泛型，在调用点回落到
  `reflect.Value`；`gs-mock` 走泛型保留类型，IDE 能补全，编译器能查参
  数漂移。
- **不做 gomock 包装**。代码生成直接对准 `gsmock` 运行时，两部分作为
  一体演进。
