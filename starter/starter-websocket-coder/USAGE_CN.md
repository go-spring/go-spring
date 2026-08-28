# starter-websocket-coder 使用说明 — 参考手册

[English](USAGE.md) | [中文](USAGE_CN.md)

详尽使用参考。概览见 [README_CN.md](README_CN.md)。下文所有行为声明均对照 starter 源码
（`starter.go`）与自校验的 [example/](example)（`example/check.sh`）核实。
coder/websocket 自身语义（`websocket.Accept`、子协议、`OriginPatterns`、压缩模式）见
[coder/websocket 文档](https://github.com/coder/websocket)——本文只写 go-spring 的增量。

**激活方式**：import 即无条件生效——blank import 注册 provider；即使零 key（全默认）也会
从 `${spring.websocket}` 创建 bean。**无端口设计**：与 gorilla 不同，coder/websocket 没有
Upgrader *对象*——服务端升级是自由函数 `websocket.Accept(w, r, *AcceptOptions)`，因此本
starter 直接贡献可注入的 `*websocket.AcceptOptions`。路由挂在应用既有的 HTTP 服务器上。
它是 [starter-websocket](../starter-websocket)（gorilla）的 coder/websocket 姊妹——两个都
import 也没问题，但共享 `spring.websocket` 前缀。

---

## 1. 完整工程示例

一个带子协议协商、context-takeover 压缩、`wsjson` JSON echo 与升级前鉴权的 echo 服务。
文件树：

```
demo/
├── go.mod
├── main.go
├── router.go
└── conf/
    └── app.properties
```

**go.mod**（关键依赖）：

```
require (
    github.com/coder/websocket      latest
    github.com/coder/websocket/wsjson latest
    go-spring.org/spring            v1.3.x
    go-spring.org/starter-websocket-coder latest
    go-spring.org/starter-actuator  latest   // 可选：探针
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-websocket-coder"
)

func main() { gs.Run() }
```

**router.go** —— 应用的全部 WebSocket 面：

```go
package router

import (
    "context"
    "net/http"

    "github.com/coder/websocket"
    "github.com/coder/websocket/wsjson"
    "go-spring.org/log"
    "go-spring.org/spring/gs"
)

type Controller struct{}

func (c *Controller) Echo(ctx context.Context, conn *websocket.Conn) {
    defer conn.CloseNow()
    for {
        mt, msg, err := conn.Read(ctx)
        if err != nil {
            return
        }
        if err = conn.Write(ctx, mt, msg); err != nil {
            return
        }
    }
}

func init() {
    gs.Provide(&Controller{})

    // starter 注入配置好的 *websocket.AcceptOptions；mux bean 把路由
    // 挂到 gs 内置 HTTP 服务器上。
    gs.Provide(func(c *Controller, opts *websocket.AcceptOptions) *gs.HttpServeMux {
        mux := http.NewServeMux()
        mux.Handle("/echo", requireApp(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            conn, err := websocket.Accept(w, r, opts)
            if err != nil {
                log.Errorf(r.Context(), log.TagAppDef, "accept /echo failed: %v", err)
                return
            }
            c.Echo(r.Context(), conn)
        })))
        mux.Handle("/json", requireApp(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            conn, err := websocket.Accept(w, r, opts)
            if err != nil {
                return
            }
            defer conn.CloseNow()
            var req struct{ Name string `json:"name"` }
            for {
                if err := wsjson.Read(r.Context(), conn, &req); err != nil {
                    return
                }
                _ = wsjson.Write(r.Context(), conn, map[string]string{"message": "Hi, " + req.Name})
            }
        })))
        return &gs.HttpServeMux{Handler: mux}
    })
}

func requireApp(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Header.Get("X-App") != "go-spring" {
            http.Error(w, "forbidden", http.StatusForbidden)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

**conf/app.properties** —— 上述代码用到的完整注释配置面：

```properties
# --- 端口归 HTTP 服务器所有（本 starter 无端口） --------------------------------
spring.http.server.addr=:9797
# 长连接：把服务器超时清零，否则会中途切断 socket。
spring.http.server.readTimeout=0
spring.http.server.writeTimeout=0
spring.http.server.idleTimeout=0

# --- AcceptOptions 调优（全可选；精确匹配的 camelCase key） ---------------------
spring.websocket.subprotocols=echo.v1
spring.websocket.compressionMode=1          # 0 关、1 开（context takeover）、
                                            # 2 开但不带 context takeover——裸 int，见 §3
# spring.websocket.compressionThreshold=0
# spring.websocket.insecureSkipVerify=false
# spring.websocket.originPatterns=
```

**验证**：

```bash
go run . &
curl -si -H 'X-App: go-spring' localhost:9797/echo     # 无升级头 → 400/426
# 用请求 echo.v1 的 ws 客户端：conn.Subprotocol() == "echo.v1"
# （example/check.sh 有断言）
```

可运行的 [example/](example) 拨号文本/JSON echo、断言子协议协商与鉴权门的 403；
`example/check.sh` 自校验执行。

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-websocket-coder
  └─ gs.Provide(NewAcceptOptions, gs.TagArg("${spring.websocket}"))
        └─ Condition：gs.OnMissingBean[*websocket.AcceptOptions]()

gs.Run()
  ├─ 配置绑定：${spring.websocket} → Config（value tag；全可选）
  ├─ bean 装配：你的 *gs.HttpServeMux provider 拿到注入的 *websocket.AcceptOptions；
  │    应用自带 AcceptOptions bean 时优先（OnMissingBean 让位）
  └─ 运行：再无其他——无服务器、无端口、无生命周期、无停机钩子。
     连接生命周期（ctx 作用域的 Read/Write、CloseStatus 握手）是
     coder/websocket 的事。
```

### 2.2 一次连接的逐层走读

带 `Sec-WebSocket-Protocol: echo.v1`、`X-App` 头的 `GET /echo`：

1. gs HTTP 服务器（流式场景超时已清零）分发到你的 mux。
2. 你的守卫中间件在 **accept 之前**运行——此处 403 根本到不了 coder/websocket（example
   的 bad-dial 用例有断言）。
3. `websocket.Accept(w, r, opts)`：
   - origin 校验：coder/websocket 默认校验 Origin；`originPatterns`（glob 匹配）是
     `CheckOrigin` 的等价物；`insecureSkipVerify` 关闭校验（仅影响浏览器客户端——非浏览器
     客户端没有 Origin，本就不校验）。
   - 子协议协商：你的 `subprotocols` × 客户端请求——服务端列表中第一个匹配者胜出；无
     交集 ⇒ 连接上无子协议。
   - `compressionMode` 选择 permessage-deflate 行为（裸整数的坑见 §3）；
     `compressionThreshold` 按消息大小决定是否压缩（0 = coder 默认行为）。
4. 应用循环：读写都是 ctx 作用域（`conn.Read(ctx)`/`conn.Write(ctx, ...)`）；关闭带状态码
   （`conn.Close(code, reason)`）或硬关（`CloseNow`）。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.websocket` 之下——**精确匹配的 camelCase，与 value tag 逐字对应**
（无宽松绑定），且**与 gorilla 姊妹共享**（§6）。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `subprotocols` | []string | — | 按顺序通告，与客户端列表协商。 | 不匹配 ⇒ 连接上无子协议（静默漂移）。 |
| `insecureSkipVerify` | bool | `false` | 跳过 origin 校验——仅浏览器客户端；非浏览器本就不校验。 | `true` 对浏览器客户端关掉与 CSRF 相关的 origin 校验。 |
| `originPatterns` | []string | — | Origin 头的 glob 匹配模式（coder 的 `CheckOrigin` 等价物），如 `*.example.com`。 | glob 写错 ⇒ 浏览器客户端在 accept 被拒。 |
| `compressionMode` | int | `0` | 直接强转 `websocket.CompressionMode`：`0` 关、`1` 开且带 context takeover、`2` 开且不带。⚠ **裸 int**——拼错（如 `3`）会静默变成别的/无效模式而非报错。 | 越界值给出未定义的压缩行为，启动期无任何信号。 |
| `compressionThreshold` | int | `0` | 触发压缩的最小消息字节数；0 保持 coder/websocket 默认门槛。 | 过低 ⇒ 小帧烧 CPU；过高 ⇒ 实际上不压缩。 |

⚠ 除这些字段外没有别的 key——需要更多能力时自己提供 `*websocket.AcceptOptions` bean
（`OnMissingBean` 让位）。

---

## 4. 验证与故障演练

### 4.1 子协议协商

```go
conn, _, err := websocket.Dial(ctx, "ws://127.0.0.1:9797/echo", &websocket.DialOptions{
    Subprotocols: []string{"echo.v1"},
})
// conn.Subprotocol() == "echo.v1" —— example/check.sh 有断言
```

请求未列出的协议 ⇒ `Subprotocol()` 为空但连接照常——静默漂移。

### 4.2 Origin 门演练

```bash
curl -si -o/dev/null -H 'Origin: http://evil.example' -H 'Connection: Upgrade' \
  -H 'Upgrade: websocket' -H 'Sec-WebSocket-Version: 13' -H 'Sec-WebSocket-Key: x' \
  localhost:9797/echo       # 被 origin 校验拒绝
# 配 spring.websocket.originPatterns=*.example.com 则放行——双向验证
```

### 4.3 压缩模式演练

`compressionMode=1` 时，用支持压缩的客户端重复发送相似载荷——context takeover 使第二条
消息在线上明显更小（devtools/wireshark）。模式 `0` 时无论客户端是否支持都不压缩。

### 4.4 升级前鉴权演练

example 断言：不带 `X-App` 头的拨号得到 HTTP 403——守卫先于 `websocket.Accept` 运行。

### 4.5 冒烟测试

```bash
cd example && ./check.sh    # 自校验：echo、JSON echo、子协议、403 门
```

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|-----|----------|------|
| 连接约 5s/60s 后被断 | 内置 HTTP 服务器的超时切断流 | 清零 `spring.http.server.*Timeout`/`idleTimeout`（同 example）。 |
| 浏览器客户端 accept 被拒 | 默认 origin 校验；Origin 未覆盖 | 加 `originPatterns` glob（仅公共端点才用 `insecureSkipVerify=true`）。 |
| `conn.Subprotocol()` 为空 | 客户端协议与 `subprotocols` 无交集 | 对齐列表。 |
| key 疑似无效（如 `compression-mode`） | camelCase 精确匹配——kebab/大小写变体不绑定 | 用 tag 原拼写：`compressionMode`。 |
| 压缩行为异常 | `compressionMode` 是裸 int——拼错即静默换模式 | 只用 0/1/2；用 §4.3 验证。 |
| 需要自定义 accept 行为 | 没有对应配置 key | 自己提供 `*websocket.AcceptOptions` bean——`OnMissingBean` 让位。 |
| 普通 curl 得到 400/426 | curl 不发升级头 | 属预期；用真 ws 客户端测。 |
| 两个 websocket starter 被意外互相配置 | 与 gorilla 姊妹共享 `spring.websocket` 前缀 | 每协议家族 import 一个，或把 key 限定在共有字段（`subprotocols`）。 |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 5 |
| 其中必填 | 0 |
| quickstart 前置外部依赖 | 0 |
| "注意/坑"条数 | 3（服务器超时；共享前缀；裸 int compressionMode） |

设计嫌疑清单：

1. 与 starter-websocket（gorilla）共享 `spring.websocket` 前缀——两个都 import 时一个
   key 块同时配置两者；key 名不同但 `subprotocols` 两边同义。
2. camelCase key（`insecureSkipVerify`）偏离仓库 kebab-case 惯例——在精确匹配绑定（无宽松
   形态）下是个坑。
3. `compressionMode` 用裸 int 把 coder/websocket 的枚举推进配置——拼错会静默换模式而非
   报错。
4. 完全无可观测（无连接计数、无 accept 失败指标）——升级成功率恰是运维最缺的健康信号。
