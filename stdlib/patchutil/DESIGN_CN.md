# patchutil 设计说明
[English](DESIGN.md) | [中文](DESIGN_CN.md)

属于零依赖的 `stdlib` 层。`patchutil` 是一个单独的"逃生舱"，从存在的第一天
起就是在"没有它更糟"与"请不要滥用"之间做取舍。

## 1. 职责与边界

- 只提供**一个** unsafe 原语：清掉 `reflect.Value` 上的 read-only 标记，
  让对未导出字段的赋值成为可能。
- 不是反射库。别的反射工具在 `stdlib/typeutil` 或标准 `reflect` 中。
- 不是通用"什么都能改"工具 —— 仅清 RO 位；可寻址性、Kind 等其它前提仍需
  调用方自己保证。

## 2. 约束

- 依赖 `reflect.Value` 的内存布局以及私有 `flagStickyRO` / `flagEmbedRO`
  位值。实践中跨 Go 版本稳定，但不在 Go 1 兼容承诺范围内。
- 使用 `unsafe.Pointer`。当前不与严格 `unsafeptr` 检查器兼容。
- 不是线程安全的 —— 返回的 `Value` 与入参共享底层 flag 字段。

## 3. 取舍

- 把这个包完全避掉的办法是让调用点导出必要字段。它之所以存在，是因为
  框架内部（例如把值绑到历史遗留结构体的测试场景）有时无法修改目标类型。
  这类场景范围小、可审计，因此文件保持在 40 行左右。
