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

## 可用模块

### Web 与网络

| 模块 | 说明 |
|--------|-------------|
| [httpclt](./httpclt/) | 声明式 HTTP 客户端运行时工具：请求元数据、请求选项与流式 JSON 解码 |
| [httpsvr](./httpsvr/) | 轻量 HTTP 服务端工具：`ServeMux` 服务缝、请求上下文、JSON 与 SSE 处理器封装 |
| [httputil](./httputil/) | 从入站请求推导 OTel-free 的 HTTP 语义约定属性，供各 server starter 复用 |
| [formutil](./formutil/) | 表单处理工具：把表单值解码进类型化结构体 |
| [netutil](./netutil/) | 网络相关工具 |

### JSON 与数据整型

| 模块 | 说明 |
|--------|-------------|
| [jsonflow](./jsonflow/) | JSON 流式处理工具包 |
| [flatten](./flatten/) | 嵌套数据结构扁平化为平铺 key 路径 |
| [patchutil](./patchutil/) | 补丁处理工具 |

### 集合与泛型

| 模块 | 说明 |
|--------|-------------|
| [goutil](./goutil/) | Go 泛型通用工具，上下文取消控制等功能 |
| [iterutil](./iterutil/) | 迭代器和循环处理工具 |
| [listutil](./listutil/) | 给 `container/list` 加泛型类型安全外壳，附切片工具 |
| [ordered](./ordered/) | map key 排序遍历工具 |

### 错误、上下文与文件

| 模块 | 说明 |
|--------|-------------|
| [errutil](./errutil/) | 错误处理工具：错误包装、栈追踪捕获、构造器前置条件 |
| [ctxcache](./ctxcache/) | 基于 Context 的请求级缓存工具 |
| [fileutil](./fileutil/) | 文件系统工具 |
| [funcutil](./funcutil/) | 函数工具：延迟求值、偏函数应用等 |

### 文本、哈希与数值

| 模块 | 说明 |
|--------|-------------|
| [textstyle](./textstyle/) | 文本样式格式化工具 |
| [hashutil](./hashutil/) | 哈希计算工具 |
| [md5util](./md5util/) | MD5 哈希便捷工具 |
| [mathutil](./mathutil/) | 数学数值工具，含防溢出算术 |
| [bufutil](./bufutil/) | 有界缓冲工具，旁路拷贝永不阻塞主流程 |
| [typeutil](./typeutil/) | 类型反射和转换工具 |

### 测试

| 模块 | 说明 |
|--------|-------------|
| [testing](./testing/) | 流式 API 断言库：`assert`（失败继续）与 `require`（失败即停）两种模式，类型专属断言 |

## 许可证

Apache License 2.0
