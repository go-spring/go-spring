# starter-lua-filter

[English](README.md) | [中文](README_CN.md)

`starter-lua-filter` 基于 [gopher-lua](https://github.com/yuin/gopher-lua) 让你用
Lua 编写 HTTP 请求过滤器。每个过滤器运行在 `net/http` 层——与 Envoy、OpenResty
的 Lua filter 在网关数据面所处的位置相同——因此它与背后服务路由的 Web 框架
（gin/echo/hertz/net-http）无关。过滤器可以观察请求、改写响应头，或直接拦截返回，
全程无需重新编译 Go 程序。

## 安装

```bash
go get go-spring.org/starter-lua-filter
```

## 快速开始

### 1. 引入 `starter-lua-filter` 包

参考 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-lua-filter"
```

### 2. 配置 Lua 过滤器

在项目的[配置文件](example/conf/app.properties)中添加过滤器配置。每一项指向一个
Lua 脚本文件（相对工作目录解析）：

```properties
spring.lua.filter.guard.script=./scripts/guard.lua
```

### 3. 将过滤器接入 HTTP 服务

过滤器按配置子键（`guard`）注入，用来包裹你的业务处理器。把包好的处理器交给
`*gs.HttpServeMux`，过滤器就位于服务最前端。参考 [example.go](example/example.go) 文件。

```go
gs.Provide(func(guard *StarterLuaFilter.Filter) *gs.HttpServeMux {
    mux := http.NewServeMux()
    mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
        _, _ = w.Write([]byte("hello"))
    })
    return &gs.HttpServeMux{Handler: guard.Wrap(mux)}
}, gs.TagArg("guard"))
```

### 4. 编写 Lua 脚本

参考 [guard.lua](example/scripts/guard.lua) 文件。脚本每请求执行一次，可调用下方宿主 API：

```lua
log("incoming " .. req.method .. " " .. req.path)
resp.set_header("X-Lua-Filter", "guard")
if req.path == "/admin" and req.header("X-Token") ~= "sesame" then
    deny(403, "forbidden: bad token")
    return
end
```

## 宿主 API

脚本可见以下全局对象，每请求重新绑定：

| 符号 | 说明 |
| --- | --- |
| `req.method` / `req.path` | 请求方法与 URL 路径（字符串） |
| `req.header(name)` | 读取请求头，不存在返回 `""` |
| `req.query(name)` | 读取 URL 查询参数，不存在返回 `""` |
| `resp.set_header(name, value)` | 设置响应头 |
| `deny(status, message)` | 写入 `status`+`message` 并短路调用链；调用后应立即 `return` |
| `log(message)` | 通过 go-spring 日志管道输出 |

## 核心特性

[example.go](example/example.go) 程序演示并断言了四个核心过滤器行为：

* **放行 + 改写** —— 正常的 `/hello` 请求到达业务处理器，并带上脚本注入的
  `X-Lua-Filter` 响应头。
* **拦截** —— `/admin` 缺少 token 时被 `403` 短路，业务处理器永不执行。
* **条件放行** —— 满足脚本的 token 条件后，同一 `/admin` 路径正常放行。
* **热更新** —— 改写磁盘上的脚本并调用 `Reload()` 后，新规则无需重启进程即可生效。

## 高级特性

* **多过滤器**：在 `spring.lua.filter.*` 下定义多项，用 `gs.TagArg("...")` 按名选择。
* **框架无关**：过滤器包裹的是普通 `http.Handler`，无论 gin/echo/hertz/net-http
  服务路由，同一份脚本都适用。
* **沙箱 VM**：每请求从池中借用 `*lua.LState`，仅开启 `base`/`table`/`string`/`math`
  库，文件系统与加载器逃逸（`dofile`/`loadfile`/`load`）已被剥除。
* **热更新**：调用 `Filter.Reload()` 重新编译脚本并原子替换新字节码。编译失败时保留原脚本，
  因此一次错误编辑绝不会让过滤器失效。
* **资源清理**：starter 注册了 destroy 回调，在关闭时关闭池中每个 `*lua.LState`，
  释放过滤器创建的 VM。
