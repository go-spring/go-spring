# starter-websocket 使用说明 — 参考手册

[English](USAGE.md) | [中文](USAGE_CN.md)

详尽使用参考。概览见 [README_CN.md](README_CN.md)。下文所有行为声明均对照 starter 源码
（`starter.go`）与自校验的 [example/](example)（`example/check.sh`）核实。
gorilla/websocket 自身语义（帧、子协议协商、`CheckOrigin`）见
[gorilla/websocket 文档](https://github.com/gorilla/websocket)——本文只写 go-spring 的增量。

**激活方式**：import 即无条件生效——blank import 注册 provider；即使零 key（全默认）也会
从 `${spring.websocket}` 创建 bean。**无端口设计**：starter 只贡献配置好的
`*websocket.Upgrader`；WebSocket 路由挂在应用既有的 HTTP 服务器上（gs 内置 server、gin、
echo……），监听地址与超时归那个服务器所有。

---

## 1. 完整工程示例

一个带子协议协商、Origin 白名单、压缩与升级前鉴权的 echo 服务。文件树：

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
    github.com/gorilla/websocket  latest
    go-spring.org/spring          v1.3.x
    go-spring.org/starter-websocket latest
    go-spring.org/starter-actuator latest   // 可选：探针
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-websocket"
)

func main() { gs.Run() }
```

**router.go** —— 应用的全部 WebSocket 面：

```go
package router

import (
    "net/http"

    "github.com/gorilla/websocket"
    "go-spring.org/log"
    "go-spring.org/spring/gs"
)

type Controller struct{}

func (c *Controller) Echo(conn *websocket.Conn) {
    defer conn.Close()
    for {
        mt, msg, err := conn.ReadMessage()
        if err != nil {
            return
        }
        if err = conn.WriteMessage(mt, msg); err != nil {
            return
        }
    }
}

func init() {
    gs.Provide(&Controller{})

    // starter 注入配置好的 *websocket.Upgrader；mux bean 把路由挂到
    // gs 内置 HTTP 服务器上。
    gs.Provide(func(c *Controller, upgrader *websocket.Upgrader) *gs.HttpServeMux {
        mux := http.NewServeMux()
        mux.Handle("/echo", requireApp(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            conn, err := upgrader.Upgrade(w, r, nil)
            if err != nil {
                log.Errorf(r.Context(), log.TagAppDef, "upgrade /echo failed: %v", err)
                return
            }
            c.Echo(conn)
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
# --- 端口归 HTTP 服务器所有（starter-websocket 无端口） ------------------------
spring.http.server.addr=:9696
# 长连接：把服务器超时清零，否则会中途切断 socket。
spring.http.server.readTimeout=0
spring.http.server.writeTimeout=0
spring.http.server.idleTimeout=0

# --- upgrader 调优（全可选；key 为精确匹配的 camelCase） ------------------------
spring.websocket.handshakeTimeout=10s
spring.websocket.readBufferSize=1024
spring.websocket.writeBufferSize=1024
spring.websocket.enableCompression=true
spring.websocket.allowedOrigins=http://127.0.0.1:9696
spring.websocket.subprotocols=echo.v1
```

**验证**：

```bash
go run . &
curl -si -H 'X-App: go-spring' 'localhost:9696/echo'    # 无升级头 → 400/426
# 用请求 echo.v1 的 ws 客户端：协商出的子协议会回传——
#  conn.Subprotocol() == "echo.v1"   （example/check.sh 有断言）
```

可运行的 [example/](example) 拨号文本/JSON echo、断言子协议协商与鉴权门的 403；
`example/check.sh` 自校验执行。

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-websocket
  └─ gs.Provide(NewUpgrader, gs.TagArg("${spring.websocket}"))
        └─ Condition：gs.OnMissingBean[*websocket.Upgrader]()

gs.Run()
  ├─ 配置绑定：${spring.websocket} → Config（value tag；全可选）
  ├─ bean 装配：你的 *gs.HttpServeMux provider 拿到注入的 *websocket.Upgrader；
  │    应用自带 Upgrader bean 时优先于 starter 的（OnMissingBean 让位）
  └─ 运行：再无其他——starter 无服务器、无端口、无生命周期、无停机钩子。
     连接生命周期是 gorilla/websocket 的事。
```

顺序说明：upgrader bean 先于路由 provider 存在，handler 接线不会与配置竞争——除标准
bean 装配外没有时序面。

### 2.2 一次连接的逐层走读

带 `Sec-WebSocket-Protocol: echo.v1`、`X-App` 头、Origin
`http://127.0.0.1:9696` 的 `GET /echo`：

1. gs HTTP 服务器（流式场景超时已清零）分发到你的 mux。
2. 你的守卫中间件在**升级之前**运行——此处 403 根本到不了 gorilla（example 的
   bad-dial 用例有断言）。
3. `upgrader.Upgrade(w, r, nil)`：
   - `CheckOrigin`——`allowedOrigins` 为空保持 gorilla 默认同源策略；非空白名单则替换它
     （与 Origin 头精确匹配；单个 `*` 条目接受任意 origin）。
   - 子协议协商：你的 `subprotocols` 中第一个也被客户端请求的条目胜出；无交集 ⇒ 不选
     子协议（连接继续）。
   - `handshakeTimeout` 限定服务端握手读取。
4. 应用读写循环（gorilla 语义——ping/pong、消息类型、关闭握手由你处理）。buffer 大小
   作用于连接的 I/O 缓冲。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.websocket` 之下——**精确匹配的 camelCase，与 value tag 逐字对应**
（无宽松绑定：`handshaketimeout`/`handshake-timeout` 解析不到）。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `handshakeTimeout` | duration | `10s` | 限定服务端握手读取（gorilla `Upgrader.HandshakeTimeout`）。 | 过低会把慢/移动客户端掐死在握手中。 |
| `readBufferSize` | int | `1024` | 连接读端 I/O 缓冲字节数。 | 过小 ⇒ 读碎片、syscall 变多；过大 ⇒ 每连接内存上升。 |
| `writeBufferSize` | int | `1024` | 写端 I/O 缓冲。 | 写路径同理。 |
| `enableCompression` | bool | `false` | permessage-deflate（RFC 7692）。⚠ 客户端也要协商。 | 开了但客户端不支持 ⇒ 走明文帧（无害）；指望压缩收益而无客户端支持 ⇒ 没有。 |
| `subprotocols` | []string | — | 按偏好顺序通告；握手中第一个与客户端请求匹配的胜出。 | 不配则协商交给你的代码；不匹配 ⇒ 连接上无子协议（静默）。 |
| `allowedOrigins` | []string | — | 空 = gorilla 默认同源 `CheckOrigin`；非空则替换为精确匹配白名单；`*` 接受任意 origin。⚠ **没有能配任意 CheckOrigin 函数的 key**——需要时自己提供 `*websocket.Upgrader` bean。 | 未列入白名单的浏览器端在升级时得到 403；`*` 关掉了与 CSRF 相关的 origin 校验。 |

⚠ 姊妹 starter [starter-websocket-coder](../starter-websocket-coder) 共享同一个
`spring.websocket` 前缀——见 §6。

---

## 4. 验证与故障演练

### 4.1 子协议协商

```bash
# 任意请求 echo.v1 的 ws 客户端：
#   websocket.DefaultDialer.Dial(url, header{"Sec-WebSocket-Protocol": "echo.v1"})
# 随后 conn.Subprotocol() == "echo.v1" —— example/check.sh 有断言
```

请求不在 `subprotocols` 里的协议 ⇒ `conn.Subprotocol()` 为空但连接照常——客户端若假定某
协议就是静默漂移。

### 4.2 Origin 门演练

```bash
curl -si -o/dev/null -H 'Origin: http://evil.example' -H 'Connection: Upgrade' \
  -H 'Upgrade: websocket' -H 'Sec-WebSocket-Version: 13' -H 'Sec-WebSocket-Key: x' \
  localhost:9696/echo        # CheckOrigin 返回 403
curl -si ... -H 'Origin: http://127.0.0.1:9696' ...   # 过门
```

### 4.3 压缩演练

`enableCompression=true` 时，用支持压缩的客户端发送大（超过 buffer）文本帧并在线上比对
帧大小（devtools/wireshark）；不支持压缩的客户端照常明文连接。

### 4.4 升级前鉴权演练

example 断言：不带 `X-App` 头的拨号得到 HTTP 403——证明守卫先于 `Upgrade` 运行，升级错误
与守卫错误不会混在一起。

### 4.5 冒烟测试

```bash
cd example && ./check.sh    # 自校验：echo、JSON echo、子协议、403 门
```

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|-----|----------|------|
| 连接约 5s/60s 后被断 | 内置 HTTP 服务器的超时切断流 | `spring.http.server.readTimeout/writeTimeout/idleTimeout=0`（同 example）。 |
| 浏览器连接即 403 | Origin 不在 `allowedOrigins`（或默认同源策略） | 加上精确 Origin；仅公共端点用 `*`。 |
| `conn.Subprotocol()` 为空 | 客户端请求的协议与 `subprotocols` 无交集 | 对齐两边的列表；检查客户端拼写。 |
| key 疑似无效（如 `handshake-timeout`） | camelCase 精确匹配——kebab/大小写变体不绑定 | 用 tag 原拼写：`handshakeTimeout`。 |
| 需要自定义 CheckOrigin 逻辑 | 函数无法用 key 表达 | 自己提供 `*websocket.Upgrader` bean——`OnMissingBean` 让位。 |
| 普通 curl 得到 400/426 | curl 不发升级头 | 属预期；用真 ws 客户端测。 |
| 大消息卡顿 | `readBufferSize`/`writeBufferSize` 小于帧节奏所需 | 调大缓冲；它们是每连接的。 |
| 两个 websocket starter 被同一 key 块配置 | 与 coder 姊妹共享 `spring.websocket` 前缀 | 每协议家族 import 一个，或把 key 限定在两者共有的字段上。 |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 6 |
| 其中必填 | 0 |
| quickstart 前置外部依赖 | 0 |
| "注意/坑"条数 | 2（服务器超时；CheckOrigin 上限） |

设计嫌疑清单：

1. camelCase key（`handshakeTimeout`）偏离仓库 kebab-case 惯例——在精确匹配绑定（无宽松
   形态）下是个坑。
2. `starter-websocket` / `starter-websocket-coder` 共用 `spring.websocket` 前缀——两个都
   import 时一个 key 块同时配置两者，且 value key 概念上相撞（`subprotocols` 两边同义）。
3. 完全无可观测（无连接计数、无升级失败指标）——对长连接 starter 而言，升级成功率恰是
   运维最缺的健康信号。
