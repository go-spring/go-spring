# fileutil 设计说明
[English](DESIGN.md) | [中文](DESIGN_CN.md)

属于零依赖 `stdlib` 层。两个小工具，让调用 `os` 相关 API 时更少样板。

## 1. 职责与边界

- 把"判断 `os.ErrNotExist`"的常见模式收敛为单次调用（`PathExists`），
  把"是否存在"与"是否发生错误"两个语义清晰地分开。
- 读取目录条目名而不泄漏 `*os.File` —— `ReadDirNames` 内部开、读、关一气呵成。
- 不是文件系统抽象层。遍历、监听、原子写、路径拼接等能力都不在这里，
  它们要么在 `os` / `filepath` 中，要么属于更上层的包。

## 2. 设计说明

- `PathExists` 永远不会把 `os.ErrNotExist` 当错误抛出，"不存在"用
  `(false, nil)` 表达；其它 stat 错误原样返回。
- `ReadDirNames` 直接返回 `f.Readdirnames(-1)` 的结果，可能返回部分切片
  加上非空 err，调用方需要同时检查两者。
