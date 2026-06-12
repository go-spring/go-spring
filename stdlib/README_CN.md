# stdlib

<div>
   <img src="https://img.shields.io/github/license/go-spring/stdlib" alt="license"/>
   <img src="https://img.shields.io/github/go-mod/go-version/go-spring/stdlib" alt="go-version"/>
   <img src="https://img.shields.io/github/v/release/go-spring/stdlib?include_prereleases" alt="release"/>
   <a href="https://codecov.io/gh/go-spring/stdlib" >
      <img src="https://codecov.io/gh/go-spring/stdlib/branch/main/graph/badge.svg?token=SX7CV1T0O8" alt="test-coverage"/>
   </a>
   <a href="https://goreportcard.com/report/go-spring.org/stdlib">
      <img src="https://goreportcard.com/badge/go-spring.org/stdlib" alt="Go Report Card"/>
   </a>
   <a href="https://deepwiki.com/go-spring/stdlib"><img src="https://deepwiki.com/badge.svg" alt="Ask DeepWiki"></a>
</div>

[English](README.md) | [中文](README_CN.md)

`stdlib` 是一系列精心设计的独立 Go 语言工具模块集合，对 Go 标准库进行了有益补充，让日常 Go 开发更加便捷愉悦。

每个模块都是独立的，可以单独使用。每个模块目录下都有详细的文档说明。

## 可用模块

| 模块 | 说明 |
|--------|-------------|
| [ctxcache](./ctxcache/) | 基于 Context 的缓存工具 |
| [errutil](./errutil/) | 错误处理工具，提供错误包装、栈追踪捕获等功能 |
| [fileutil](./fileutil/) | 文件系统工具 |
| [flatten](./flatten/) | 嵌套数据结构扁平化 |
| [formutil](./formutil/) | 表单处理工具 |
| [funcutil](./funcutil/) | 函数工具，延迟求值、偏函数应用等 |
| [goutil](./goutil/) | Go 通用工具，上下文取消控制等功能 |
| [hashutil](./hashutil/) | 哈希计算工具 |
| [httpclt](./httpclt/) | HTTP 客户端工具 |
| [httpsvr](./httpsvr/) | HTTP 服务端工具 |
| [iterutil](./iterutil/) | 迭代器和循环处理工具 |
| [jsonflow](./jsonflow/) | JSON 流式处理工具包 |
| [listutil](./listutil/) | 列表和链表工具 |
| [mathutil](./mathutil/) | 数学数值工具 |
| [md5util](./md5util/) | MD5 哈希便捷工具 |
| [netutil](./netutil/) | 网络相关工具 |
| [ordered](./ordered/) | 有序 Map 和有序 Set 数据结构 |
| [patchutil](./patchutil/) | 补丁处理工具 |
| [testing](./testing/) | 优雅的流式 API 风格单元测试断言库，支持 `assert` 和 `require` 两种模式，提供丰富的类型专属断言 |
| [textstyle](./textstyle/) | 文本样式格式化工具 |
| [typeutil](./typeutil/) | 类型反射和转换工具 |

## 许可证

Apache License 2.0
