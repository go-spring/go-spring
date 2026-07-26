# starter-lua-filter 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-lua-filter` 属于 **Contributor** 形态(见
[starter/DESIGN.md](../DESIGN.md) §2.3),让应用把 Lua 脚本作为可编程 filter
挂进 HTTP 管道——即 Kong / APISIX / Envoy / OpenResty 在网关数据面做的事。

## 1. 定位——HTTP filter,不是脚本 bean

Go 是编译型语言,没有 JVM(Spring 用 Groovy 做 refreshable bean)那种"脚本
bean"文化。Go 微服务真正用 Lua 的地方是网关数据面——不重启就能改原始
request/response。进程内的动态逻辑用 Go 中间件 / CEL / WASM 更合适。

因此本 starter 的定位是**HTTP handler 层的可编程 filter**,不是通用脚本
引擎。超时 / 内存限制、容器 bean 注入、热更新都按"网关型 filter"的方向来
扩展,而不是"脚本 bean"。

## 2. 缝隙——`*gs.HttpServeMux`

挂载点为 `spring/gs/http.go` 中的 `*gs.HttpServeMux`:框架仅在
`OnMissingBean[*HttpServeMux]` 时提供默认 mux,故用户 `gs.Provide` 一个包
了 filter 的 `*gs.HttpServeMux` 就能抢占。mux 与框架无关——gin/echo/hertz
最终都塌缩成 `http.Handler`——不需要逐框架适配。`spring/web` 目前是空
占位,故这道 handler 缝隙是天然归宿。

## 3. 实现要点

- **gopher-lua**,纯 Go 无 CGO——契合 starter 家族的跨编译取向。
- **一次预编译,VM 池化。**`parse.Parse` + `lua.Compile` 在构造期生成 proto。
  每次请求从 `sync.Pool` 借 `LState`,`install` 重绑 host API
  (`req` / `resp` / `deny` / `log`),`PCall` 后 `SetTop(0)` 归还。无后台
  goroutine,destroy 传 `nil`。
- **沙箱。**`SkipOpenLibs`,只开 base / table / string / math,并把
  `dofile` / `loadfile` / `load` / `loadstring` 置空,filter 无法触碰文件
  系统或 `eval` 任意代码。
- **多实例经 `gs.Group("${spring.lua.filter}", ...)`。**每个配置项一个具名
  bean;mux 用 `gs.TagArg("<name>")` 按名注入。加 filter 是纯配置变更。

## 4. 约束

- 脚本必须依赖固定的 host API(`req`、`resp`、`deny`、`log`)。扩宽表面 =
  扩宽沙箱——是有意保守。
- 无后台 goroutine 故不注册 destroy hook;后续若加热更新 watcher,须补
  Close 路径。

## 5. 取舍 / 弃选方案

- **通用脚本 bean——弃选。**Groovy 式 refreshable bean 与编译型 Go 不匹配;
  只有网关侧才能兑现相较于 Go 中间件 / CEL / WASM 的增量价值。
- **逐框架中间件适配——暂弃。**Go web 框架都提供 `http.Handler` 互操作,
  mux 缝隙已经统一覆盖;真需要时再加原生中间件。
- **v1 无热更新。**`spring/gs_dync` 的 Refresh 已就绪,后续要不重启改规则时
  可无缝接入。
