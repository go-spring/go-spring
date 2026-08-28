# starter-swagger 使用说明 — 参考手册

详细使用参考。概览见 [README_CN.md](README_CN.md)。所有行为声明均对照源码
（`starter.go`、`config.go`、`ui.go`）与自断言的 [example/](example/)
（`example/check.sh`，零外部服务）核对。Swagger UI 本身见
[swagger-ui 官方文档](https://github.com/swagger-api/swagger-ui)——本文只讲绑定、端点挂载
与 CDN 依赖。

**激活方式**：与 addr 门控的 server starter 不同，本 starter 采用**默认开启的 enabled
开关**模式——`spring.swagger.enabled` 为 `true` *或缺失*时注册 bean（`MatchIfMissing`，
starter.go:37）。空导入即可得到 UI；生产环境用 `spring.swagger.enabled=false` 关闭，无需
删 import。

---

## 1. 完整工程示例

一个提供 greeter API、文档可在 `/swagger/` 浏览的服务。两个变体：带 actuator（零接线）
与不带（自有 HTTP server）。example/ 跑的是变体 B。文件树：

```
demo/
├── go.mod
├── main.go
├── openapi.json            # 由 `gs-http-gen --openapi` 生成
└── conf/
    └── app.properties
```

**go.mod**：

```
require (
    go-spring.org/spring             v1.3.x
    go-spring.org/starter-swagger    latest
    go-spring.org/starter-actuator   latest   // 变体 A：管理端口 + 端点挂载
)
```

**变体 A —— actuator 挂载（零接线）**。main.go 只是 `gs.Run()` 加空导入：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-swagger"
)

func main() { gs.Run() }
```

**conf/app.properties**（变体 A）：

```properties
# --- actuator 管理端口 ----------------------------------------------------------
spring.actuator.addr=:9370

# --- swagger（全部默认值展开；通常只改 specFile）--------------------------------
spring.swagger.enabled=true
spring.swagger.basePath=/swagger
spring.swagger.specFile=openapi.json
spring.swagger.title=Greeter API Docs
# 提供 swagger-ui.css / swagger-ui-bundle.js 的 CDN；钉住大版本。
spring.swagger.assetBaseURL=https://unpkg.com/swagger-ui-dist@5
```

**变体 B —— 无 actuator**（即 `example/example.go:43-47`）：bean 同时是普通的 `*UI`
（`http.Handler`），自己挂载：

```go
gs.Provide(func(ui *StarterSwagger.UI) *gs.HttpServeMux {
    mux := http.NewServeMux()
    mux.Handle(ui.Path(), ui)          // ui.Path() = "<basePath>/"
    return &gs.HttpServeMux{Handler: mux}
})
```

配 `spring.http.server.enabled=true` + `spring.http.server.addr=:9696`。

**openapi.json** —— starter 可接受的最小 spec（它对任何合法 JSON 文档原样返回；结构遵循
[OpenAPI 规范](https://spec.openapis.org/oas/v3.1.0)，通常由生成器产出）：

```json
{
  "openapi": "3.0.3",
  "info": { "title": "Greeter API", "version": "1.0.0" },
  "paths": {
    "/greeter/{name}": {
      "get": {
        "summary": "Greet by name",
        "parameters": [{ "name": "name", "in": "path", "required": true,
                         "schema": { "type": "string" } }],
        "responses": { "200": { "description": "ok" } }
      }
    }
  }
}
```

真实项目里这个文件由生成器产出（对 handler 跑 `gs-http-gen --openapi`，config.go:30-34），
随代码提交或随二进制构建——starter 只负责读它。

**验证**（变体 A；变体 B 换 host）：

```bash
curl -s :9370/swagger/            | grep -c swagger-ui    # 1 —— UI 外壳
curl -s -o/dev/null -w '%{http_code}\n' :9370/swagger/index.html    # 200，同一外壳
curl -s :9370/swagger/openapi.json | jq -r .info.title    # Greeter API
```

外部服务：starter 本身无需任何服务；**浏览器**需要能访问 `assetBaseURL`（CDN），除非
自建镜像——见 §4.4。

---

## 2. 装配与时序

### 2.1 Bean 生命周期

```
import starter-swagger (+ starter-actuator)
  └─ gs.Provide(NewUI, TagArg("${spring.swagger}"))
        .Export(endpoint.Endpoint)
        .Condition(OnProperty("spring.swagger.enabled")="true" MatchIfMissing)  starter.go:35-37

gs.Run()
  ├─ 配置绑定：${spring.swagger} → Config（basePath / specFile / title / assetBaseURL）
  ├─ NewUI：
  │    ├─ basePath 归一化为 "/<trimmed>"                          ui.go:72
  │    ├─ os.ReadFile(specFile) —— 缺失/不可读则启动失败
  │    │   （只读一次，坏 spec 不会上线后 404）                     ui.go:74-77
  │    ├─ pageTemplate 一次性渲染 HTML 外壳                        ui.go:80-88
  │    └─ 记日志 "swagger ui configured basePath=… specFile=…"     ui.go:89
  ├─ actuator（若在场）autowire 每个 endpoint.Endpoint bean，按 Path() =
  │   "<basePath>/" 挂到管理端口——尾部斜杠声明整个子树               ui.go:98-100
  └─ 运行期：ServeHTTP 只是对 r.URL.Path 的纯 switch——spec 与 page 字节
      均预渲染；无每请求文件 I/O                                     ui.go:102-113
```

### 2.2 一次请求的逐层走读

actuator 端口上的 `GET /swagger/openapi.json`（变体 A）：

1. actuator mux 按前缀路由：`/swagger/` 下的一切委派给 `*UI` handler（挂载期注册其
   `Path()`）。
2. `ServeHTTP` 匹配 `r.URL.Path == specURL` → `Content-Type: application/json`，原样写出
   启动时捕获的 spec 字节（ui.go:104-106）。
3. `GET /swagger/`、`/swagger` 或 `/swagger/index.html` → 预渲染的 HTML 外壳
   （ui.go:107-109）。子树下其他路径 → 404（ui.go:110-111）。
4. 在**浏览器**里，外壳随后从 `assetBaseURL`（CDN）拉取 `swagger-ui.css` /
   `swagger-ui-bundle.js`（不经服务端代理），`SwaggerUIBundle` 再从烤进页面的同源
   `specURL` 取 spec。

### 2.3 双挂载设计的理由

starter 刻意不持有监听器（starter.go:25-31）：经 `endpoint.Endpoint` 挂载意味着 actuator
的管理端口——本就是运维流量（探针、指标）所在——顺带承载文档，应用侧零接线。普通
`http.Handler` 逃生口是为没有 actuator 的应用准备的：注入 `*UI`、挂 `Path()` 即可
（ui.go:59-61）。两种挂载提供字节一致的内容，因为 page 与 spec 都在 `NewUI` 里恰好
渲染/读取一次。

时序理由（源码注释）：spec 恰好读一次，是为了缺失文件在启动期 fail-fast，而不是上线后
变 404（config.go:30-34，ui.go:69-70）；重资产放 CDN，是为了 starter 不打包数兆静态文件、
只服务极小的外壳（config.go:22-23）。

停机：无需排空——没有监听器、没有 goroutine；bean 只是预计算好的字节。

---

## 3. 逐 key 行为参考

全部 key 在 `spring.swagger.*` 下（含开关共 5 个；Config 字段无一必填）。

| Key | 类型 | 默认 | 行为 / 联动 | 配错后果 |
|-----|------|------|------------|----------|
| `enabled` | bool | true（MatchIfMissing） | 开关——**与 server starter 的 addr 激活惯例不同**（starter.go:37）。⚠ 缺失即开启。 | 以为默认关 → 生产环境静默挂上了文档；显式设 `false`。 |
| `basePath` | string | `/swagger` | UI 声明的子树；NewUI 里 trim 归一（ui.go:72）。`Path()` 返回 `basePath + "/"`，整个子树（index、spec）由此 handler 提供。⚠ 与其他 actuator 端点路径重叠会在挂载时报错。 | 路径与别的端点重叠 → 启动期挂载冲突。 |
| `specFile` | string | `openapi.json` | OpenAPI 文档路径（通常由 `gs-http-gen --openapi` 产出）；启动读一次（ui.go:74）。⚠ 相对 cwd。⚠ **原样返回、绝不重读**——重新生成文件需重启。 | 缺失/不可读 → 启动失败（`swagger: reading spec file …`）。 |
| `title` | string | `API Documentation` | 文档页的浏览器标签标题（仅 HTML 外壳）。 | 纯外观。 |
| `assetBaseURL` | string | `https://unpkg.com/swagger-ui-dist@5` | `swagger-ui.css` / `swagger-ui-bundle.js` 的 CDN 基址；尾斜杠会 trim（ui.go:83）。⚠ 钉住大版本；隔离网环境指向自建镜像（config.go:39-43）。 | 不钉版本 → 破坏性 UI 升级会静默改页面；CDN 不可达 → 白屏，见 §5。 |

无死 key：五个都被读取（四个经 Config value tag，`enabled` 经 bean 条件）。

---

## 4. 验证与故障演练

### 4.1 端点内容检查

```bash
curl -s :9370/swagger/            | grep -o 'swagger-ui-bundle.js'      # 外壳引用 CDN bundle
curl -s :9370/swagger/            | grep -o '/swagger/openapi.json'     # spec URL 同源
curl -s -o/dev/null -w '%{ct}\n'  :9370/swagger/openapi.json            # application/json
curl -s -o/dev/null -w '%{http_code}\n' :9370/swagger/nope              # 子树内 404
```

### 4.2 关闭演练（enabled 开关）

加 `spring.swagger.enabled=false` 重启：`/swagger/` 返回 actuator 对未挂载路径的 404；
actuator 其余功能不受影响。删掉该 key → UI 回来（默认开启）。

### 4.3 fail-fast 演练

把 `specFile` 指向不存在的文件再启动：进程拒绝启动，报
`swagger: reading spec file "…": open …: no such file or directory`——坏 spec 到不了生产的
保证（ui.go:74-77）。

### 4.4 隔离网演练（CDN 依赖）

封掉对 `unpkg.com` 的出网，浏览器打开 `/swagger/`：外壳正常（200）但页面空白——资源在
客户端加载失败。修复：镜像一份资源并把 `spring.swagger.assetBaseURL` 指向
`https://assets.internal/swagger-ui-dist@5`；外壳即改从镜像加载。§4.1 的服务端 curl 不受
影响——CDN 只是浏览器侧依赖。

### 4.5 spec 重生成演练

应用运行中重新生成 `openapi.json`（路由变了）：`/swagger/openapi.json` 仍返回旧字节
（启动时只读一次）——重启后才会用新 spec。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动失败 `swagger: reading spec file …` | specFile 缺失/不可读，或 cwd 不对 | 修路径；它相对工作目录（chdir 或用绝对路径）。 |
| 生产环境意外出现 UI | `enabled` 默认开（MatchIfMissing） | 在 prod profile 里设 `spring.swagger.enabled=false`。 |
| 页面加载了但是空白 | 浏览器访问不到 `assetBaseURL`（CDN 出网被禁） | 自建资源镜像，`assetBaseURL` 指向镜像（§4.4）。 |
| 路由变了但 `/swagger/openapi.json` 是旧的 | spec 启动时读一次，绝不重读 | 重启进程（仅重生成文件不可见）。 |
| 变体 B 下整个子树 404 | mux 挂载缺尾部斜杠模式 | 恰好按 `mux.Handle(ui.Path(), ui)` 挂载——`Path()` 自带 `/` 后缀模式（ui.go:98-100）。 |
| 与其他 actuator 端点挂载冲突 | 两个端点声明了重叠子树 | 给其中一个改 `basePath`。 |
| UI 渲染但 "Failed to load API definition" | 浏览器访问不到 spec URL（与页面不同 host/port） | UI 与 spec 同源提供（构造上即是）——检查反向代理是否改写了 `/swagger/`。 |
| 没装 actuator，什么都不出 | 本 starter 不持有监听器 | 用变体 B：注入 `*UI` 挂到自己的 server（§1）。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 5（含 `enabled`） |
| 其中必填 | 0（全有默认值；真正会失败的是 spec 文件缺失） |
| quickstart 前置外部依赖 | 服务端 0（浏览器需可达 CDN） |
| 文档中"注意/坑"条数 | 4 |

设计嫌疑清单（前两条为上一轮审计保留项）：

1. `enabled` 开关模式与 server starter 的 addr 激活惯例不一致——默认开启意味着未配置的
   空导入也会挂上文档。
2. `assetBaseURL` CDN 依赖——离线集群需要镜像；失败形态（白屏）在客户端，服务端监控
   不可见。
3. spec 只读一次、永久返回：配置系统支持热刷新（`gs.Dync`）但这里没用——每次 spec 更新
   都要重启。
4. `title` 渲染进 HTML 模板（`html/template` 自动转义，安全），但它是唯一纯外观 key——
   若无人配置，按模板的单 key 删除规则属候选删除。
