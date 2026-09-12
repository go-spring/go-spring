# starter-lua-filter 使用说明 — 参考手册

详细使用参考。概览见 [README_CN.md](README_CN.md)。所有行为声明均对照源码
（`starter.go`、`filter.go`、`config.go`）与自断言的 [example/](example/)
（`example/check.sh`，零外部依赖）核对。Lua 语言语义与 gopher-lua 运行时见
[gopher-lua 文档](https://github.com/yuin/gopher-lua)——本文只讲 go-spring 接线、host API
与沙箱。

**激活方式**：任一 `spring.lua.filter.instances.*` key —— `gs.Group("${spring.lua.filter}")` 为每个
`spring.lua.filter.instances.<name>` 条目创建一个名为 `<name>` 的 `*Filter` bean（starter.go:39）。
没有 `enabled` 开关；不配 key 就没有 bean。

---

## 1. 完整工程示例

一个 `/hello`、`/admin` 路由位于 Lua guard 之后的服务：每个响应打标、`/admin` 需要
token、规则支持运行时热更新。文件树（即 `example/` 运行的形态）：

```
demo/
├── go.mod
├── main.go
├── conf/
│   └── app.properties
└── scripts/
    └── guard.lua
```

**go.mod**：

```
require (
    go-spring.org/spring            v1.3.x
    go-spring.org/starter-lua-filter latest
)
```

**main.go**：

```go
package main

import (
    "net/http"

    "go-spring.org/spring/gs"
    StarterLuaFilter "go-spring.org/starter-lua-filter"
)

func main() {
    // 提供一个 *gs.HttpServeMux，其 handler 是业务 mux 经 "guard" Lua filter
    // 包装后的产物。gs 只在没有自定义 HttpServeMux 时注册默认的，所以这份生效：
    // :9090 上的每个请求都先跑 Lua 脚本，与背后用什么框架无关。
    gs.Provide(func(guard *StarterLuaFilter.Filter) *gs.HttpServeMux {
        mux := http.NewServeMux()
        mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
            _, _ = w.Write([]byte("hello"))
        })
        mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
            _, _ = w.Write([]byte("admin ok"))
        })
        return &gs.HttpServeMux{Handler: guard.Wrap(mux)}
    }, gs.TagArg("guard"))
    gs.Run()
}
```

**scripts/guard.lua**（原样取自示例，`example/scripts/guard.lua`）：

```lua
-- observe：把每个请求经 go-spring 日志管道记下来。
log("incoming " .. req.method .. " " .. req.path)

-- mutate：给每个响应打标，让客户端看到 filter 跑过。
resp.set_header("X-Lua-Filter", "guard")

-- gate：无 token 则拦 /admin。deny() 写响应并短路整条链；
-- 调用后必须立即 return。
if req.path == "/admin" then
    if req.header("X-Token") ~= "sesame" then
        deny(403, "forbidden: bad token")
        return
    end
end
```

**conf/app.properties** —— 全量配置面：

```properties
# 每个 spring.lua.filter.instances.<name> 条目一个 filter；bean 名 = <name>。
spring.lua.filter.instances.guard.script=./scripts/guard.lua
```

监听器是 gs 内建 HTTP server（`spring.http.server.addr`，默认 `:9090`，
`spring/gs/http.go:66`）——filter 本身不监听端口。

**验证**：

```bash
go run . &
curl -i :9090/hello                              # 200，X-Lua-Filter: guard
curl -i :9090/admin                              # 403 forbidden: bad token
curl -i -H 'X-Token: sesame' :9090/admin         # 200 admin ok
```

外部依赖：无（Lua VM 纯进程内）。

---

## 2. 装配与时序

### 2.1 Bean 生命周期

```
import starter-lua-filter
  └─ gs.Group("${spring.lua.filter}", newFilter, destroyFilter)      starter.go:39

gs.Run()
  ├─ 配置绑定：${spring.lua.filter.instances.<name>} → Config（script 路径）
  ├─ newFilter：compileFile 读取 + 解析 + 编译脚本为唯一可复用的
  │   *lua.FunctionProto，原子存储（filter.go:49-66）。编译失败或文件缺失都会
  │   使启动失败——拼写错误到不了生产。
  ├─ VM 池惰性创建：每次 pool.New 构建一个新沙箱 *lua.LState 并登记进 f.states，
  │   以便停机时关闭（filter.go:58-65, 188-207）
  ├─ 装配：gs.TagArg("<name>") 把 filter 注入你的 HttpServeMux provider
  └─ SIGTERM/停机：destroyFilter 关闭池创建过的每一个 VM
      （starter.go:44-47，filter.go:83-90）
```

### 2.2 一次请求的逐层走读

无 token 的 `GET /admin`，穿过 `guard.Wrap(mux)`（filter.go:95-116）：

1. 从池借用 VM（`pool.Get`，首次使用时创建沙箱）。
2. `install` 把 host API 重新绑定为绑定到**本次请求** `reqState` 的全局变量
   （filter.go:127-164）——这让池化 VM 在请求之间无状态。
3. 推入编译好的原型并 `PCall`：脚本执行 `log()`、`resp.set_header()`，再到门禁。
   `deny(403, …)` 写状态码+响应体并置 `st.denied`。
4. `st.denied` 为真 → 返回：被包装的 handler 不会被触达（短路）。否则 →
   `next.ServeHTTP(w, r)`。
5. defer：`SetTop(0)` 清栈，VM 归还池。
6. 脚本**运行时**错误（非 deny）表现为 HTTP 500 `lua filter error: …`，被包装的
   handler 同样被跳过（filter.go:107-110）。

### 2.3 Host API —— 脚本能触达的全部

每请求重绑（filter.go:127-164）；除此之外一无所有：

| 全局 | 形式 | 效果 |
|------|------|------|
| `req.method`、`req.path` | string 字段 | 请求 method / URL path |
| `req.header(name)` | 函数 → string | 请求头取值 |
| `req.query(name)` | 函数 → string | 查询参数取值 |
| `resp.set_header(name, value)` | 函数 | 修改响应头 |
| `deny(status, message)` | 函数 | 默认 403 / "denied by lua filter"；写响应并短路 |
| `log(message)` | 函数 | 以 Info 级打到 `_app_lua_filter` tag，前缀脚本路径 |

### 2.4 沙箱

池化 VM 只开 `base` / `table` / `string` / `math`；`dofile`、`loadfile`、`load`、
`loadstring`、`collectgarbage` 置 nil（filter.go:188-207）。无文件系统、无网络、无
coroutine、无 `os`/`io`。脚本能做的 I/O 只有上述 host API。

### 2.5 Gateway 集成（bean 形态 filter）

gateway 的 `FilterWrapper` 恰好是 `Wrap(next http.Handler) http.Handler`
（`starter-gateway/compile.go:38-46`）——`*StarterLuaFilter.Filter` 无需适配即满足。
导出 bean 并在路由 filter 列表按名引用：

```go
gs.Provide(func(guard *StarterLuaFilter.Filter) gateway.FilterWrapper {
    return guard
}).Name("guard")   // DSL 里写作 lua(guard)
```

```properties
spring.gateway.routes.demo.filters=lua(guard)
```

解析发生在路由编译期，从 gateway 注入的 wrapper map 取
（`compile.go:276-285`）；bean 名不存在则启动失败，报
`no FilterWrapper bean named …`。

---

## 3. 逐 key 行为参考

### 3.1 本 starter —— `spring.lua.filter.instances.<name>.*`（1 个 key）

| Key | 类型 | 默认 | 行为 / 联动 | 配错后果 |
|-----|------|------|------------|----------|
| `<name>.script` | string | — | **必填**。Lua 源文件路径，按**工作目录**解析。bean 创建时读取并编译一次（filter.go:49-53, 168-182）。⚠ 环境变量覆盖为 `GS_SPRING_LUA_FILTER_<NAME>_SCRIPT`。⚠ 也是 `Reload()` 之后重读的路径。 | 缺失/不可读/语法错误 → 启动失败。从别的 cwd 运行 → 路径解析到别处（或解析不到）；测试里先 chdir 到模块目录。 |

这就是全部 key。多个条目创建多个 filter，按 bean 名选用。

### 3.2 消费方侧的承重 key

| Key | 归属 | 行为 |
|-----|------|------|
| `spring.http.server.addr` | gs 内建 HTTP server | filter 前置的监听器（默认 `:9090`）。 |
| `spring.gateway.routes.<id>.filters` | starter-gateway | `lua(<beanName>)` token 解析导出的 FilterWrapper bean（§2.5）。 |

---

## 4. 验证与故障演练

### 4.1 门禁 + 改写验证

```bash
curl -sD- -o/dev/null :9090/hello | grep -i x-lua-filter   # guard
curl -s -o- -w '%{http_code}\n' :9090/admin                # 403 + body
curl -s -o- -w '%{http_code}\n' -H 'X-Token: sesame' :9090/admin   # 200
```

### 4.2 每请求日志 tag

脚本每行 `log()` 落在 `_app_lua_filter`（filter.go:160-163），携带请求 context。静音或
改级别：

```properties
logger.lua_filter.type=Logger
logger.lua_filter.level=WARN
logger.lua_filter.tag=_app_lua_filter
```

### 4.3 热更新演练（免重启）

`Reload()` 是**纯 API**——没有配置驱动的刷新。把它接到你有的任何触发器（管理端点、
信号）：

```go
if err := guard.Reload(); err != nil { /* 坏编辑：旧脚本仍在运行 */ }
```

演练（正是 `example/example.go:118-131` 断言的内容）：

1. 编辑 `guard.lua`，让 `/hello` 也 `deny(403, "hello disabled")`。
2. 调 `Reload()` —— 成功：后续请求在 `/hello` 得到 403，无需重启。
3. 再保存一个语法坏掉的编辑并 `Reload()` → 返回 error；请求继续由上一个好脚本服务
   （filter.go:72-79）。坏编辑永远打不挂 filter。
4. 换装成功期间在途的请求按旧原型跑完（原子 `proto.Store`）。

### 4.4 沙箱演练

在脚本里调 `dofile("/etc/passwd")` 或 `os.getenv("HOME")` → 运行时错误 → HTTP 500
`lua filter error: attempt to index a nil value (global 'os')`（或 nil 调用）——证明 VM
没有打开任何逃逸库。

### 4.5 运行时错误观测

脚本抛错（如 `error("boom")`）→ 500，响应体 `lua filter error: boom`；被包装的 handler
被跳过。在前置服务的访问日志里 grep filter 子树的 500。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动失败 "lua filter: read script …" / "compile script …" | 脚本路径错误（相对 cwd）或 Lua 语法错误 | 修路径/key 或脚本；报错带文件+行号（filter.go:168-182）。 |
| filter 静默不生效 | 没配 `spring.lua.filter.instances.*` key（无 bean），或 mux 没经 `guard.Wrap` 挂载 | 补 key；包装 handler（`TagArg("<name>")`）。 |
| 每个请求都 500 `lua filter error: …` | 脚本运行时错误（nil 索引、参数错） | 读错误文本，标注 Lua 行号。 |
| 响应有头但 deny 没触发 | 脚本 `resp.set_header` 后落穿了 | `deny()` 之后必须 `return`；deny 只置短路标志。 |
| 双重 deny / "http: superfluous WriteHeader" | 脚本调了两次 `deny()` 或 deny 后再写 | `deny()` 后立即 return。 |
| 脚本改了没效果 | 没人调 `Reload()`——编译原型还是启动时的 | 触发 `Reload()`（纯 API，见 §4.3）或重启。 |
| 高负载下 VM/内存增长 | 池保留创建过的每个 VM（复用，停机才关闭） | 有界的预期行为；脚本内 table 无界增长会按 VM 泄漏——脚本保持无状态（全局变量每请求重绑）。 |
| gateway 应用报 `no FilterWrapper bean named "guard"` | filter bean 未在 warmup 前导出为 `gateway.FilterWrapper` | 用 `.Export(gs.As[gateway.FilterWrapper]())` 或具名 provider 导出（§2.5）。 |
| bean 名冲突 | 另一个 group starter 也注册了 `guard` bean | 改配置子键名（bean 名 = 子键，无命名空间）。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 每实例 1 |
| 其中必填 | 1（`script`） |
| quickstart 前置外部依赖 | 0 |
| 文档中"注意/坑"条数 | 4（cwd 相对脚本路径；Reload 纯 API；deny 后须 return；bean 名无命名空间） |

设计嫌疑清单（第 1-3 条为上一轮审计保留项）：

1. 无配置驱动热更新 —— `Reload()` 纯 API，运维无法在没有调用代码的情况下刷新脚本。
2. bean 名直接用配置子键且无命名空间（`guard`），可能与其他 group starter 的同名 bean
   冲突。
3. host API 面（无 body 访问、无上游信息）固化在 starter 里；要更多能力得改本模块。
4. `log()` 只有 Info 级——脚本无法选级别，啰嗦的脚本只能按 tag 整体调 logger 级别。
5. VM 池无上界（`sync.Pool` 语义 + 永久的 `states` 登记表）：并发突发会铸造那么多 VM，
   且只在停机时关闭。
6. 每请求重绑全局变量是 O(host API) 的开销——当前 API 面下无碍，API 扩张后需关注。
