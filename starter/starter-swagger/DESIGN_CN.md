# starter-swagger 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-swagger` 属于 **Contributor** 形态(见
[starter/DESIGN.md](../DESIGN.md) §2.3),基于已生成的 `openapi.json`
提供 Swagger UI。不开端口;当 `starter-actuator` 在时自动挂到 actuator,
否则应用自行挂载返回的 `http.Handler`。

## 1. 职责与边界

- **在范围内:**启动时加载 OpenAPI 3.0 spec,渲染一小段 HTML 外壳从 CDN
  拉 Swagger UI 静态资源,并将该组件以 `endpoint.Endpoint` 形态暴露给
  actuator 自动挂载。
- **不在范围内:**OpenAPI *生成*(那是 `gs-http-gen --openapi` 产 `openapi.json`);
  内置静态资源;更老的 Swagger 2.0 `--swagger` spec(有意不支持)。

## 2. 关键决策

- **`endpoint.Endpoint` 形态,零接线挂 actuator。**`Path()` 返回带尾斜杠
  的 base path;actuator 用 `mux.Handle` 接管整棵子树,`/swagger/*` 表面
  自动挂上。同一个结构体又是普通 `http.Handler`,不用 actuator 也可
  `gs.Provide` 自行挂到任意位置。
- **走 CDN,不内置静态资源。**`assetBaseURL`(默认
  `unpkg swagger-ui-dist@5`)在运行时加载。把 ~1MB 的 minified UI 打进每
  个二进制并非合理默认;离网环境覆盖 CDN URL 即可。
- **spec 读取 fail-fast。**启动时读一次;缺失或格式错为启动错,不会等
  首次访问才暴露"UI 陈旧"。

## 3. 约束

- **`html/template` 在 JS 上下文的转义。**`/` 会被转成 `\/`(仍是合法 JS)。
  冒烟断言应检 `openapi.json` 作为子串,不要死磕 `/swagger/openapi.json`
  字面——这个转义在开发中确实咬人。
- **Swagger 2.0(`--swagger`)有意不支持。**OpenAPI 3.0 是现代形态;保留
  旧 spec 会把表面翻倍换取很少价值。`gs-http-gen` 的 `--swagger` 仍可供
  需要的人使用,但本 UI 不消费它。

## 4. 取舍 / 弃选方案

- **用 `embed.FS` 内置 Swagger UI 静态资源——弃选。**膨胀每个二进制,
  升级也麻烦。用版本 tag(`@5`、`@5.x.x`)锁 CDN 既可复现,又不占字节。
- **独立 `gs.Server` 自开端口——弃选。**文档流量低且面向开发者,actuator
  (或既有应用 mux)才是归宿。姊妹 starter `starter-admin-ui` 使用
  `gs.Server` `:9280` 是因它做*轮询*工作,不宜与业务端口抢占。
