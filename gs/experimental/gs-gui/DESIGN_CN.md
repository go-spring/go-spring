# gs-gui 设计说明
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`gs-gui` 是四层栈（stdlib → spring → starter → gs）工具层里 `gs` 的外
部工具。它提供一个基于浏览器的向导，本质是 `gs init` 的薄壳前端。

## 1. 职责与边界

- 提供单页 HTML 向导，收集 `module` + `lang`，然后 exec 同目录下的
  `gs` 二进制 `init -m <module> --lang <lang>`，把合并后的
  stdout/stderr 流式回推到浏览器。
- 不做任何项目生成决策。所有生成逻辑都在 `gs init` 里；`gs-gui` 纯粹
  是展示层，保证 CLI 与 GUI 用户看到的行为一致。

## 2. 关键抽象与接缝

- **外部工具协议**。二进制名 `gs-gui`，与 `gs` 二进制并排放置；
  `gs-gui --version` 打印两行 description/version。`gs gui` 通过与其它
  外部工具相同的查找逻辑派发过去（见 `gs/gs/tool/tool.go`）。
- **UI 编译进二进制**。`//go:embed web/index.html` — 向导只是一个文件
  被编进二进制，无需独立的资源打包步骤。
- **邻居 gs 二进制发现**。`siblingGS()` 解析
  `filepath.Dir(os.Executable()) + "/gs"`；若该二进制不存在，
  `gs-gui` 拒绝运行，从而守住"GUI 是 CLI 的壳"这个不变量。
- **端口选择**。`defaultPort=8639`；`EADDRINUSE` 时
  `net.Listen("tcp", "127.0.0.1:0")` 挑临时端口。绑定始终是
  `127.0.0.1`——这是本地开发工具，不是服务。
- **流式响应**。`POST /api/create` 把 stderr 合流到 stdout，每次从管
  道读 1 KB，写完立即 `Flush()`。响应是 `text/plain` 并加
  `X-Content-Type-Options: nosniff`，让浏览器把它当追加日志渲染。

## 3. 约束

- 绝不在此重新实现 `gs init` 的逻辑。任何新选项（feature flag、语言变
  体、layout tag 锁定）都必须先落到 CLI，避免两个入口分叉。
- 监听地址永远是 `127.0.0.1`。不要暴露在 `0.0.0.0`——这个工具会
  exec 一个往调用方 cwd 写文件的脚手架。

## 4. 权衡与被否决的方案

- **拒绝 WebSocket**。chunked `text/plain` + `Flusher` 更简单、浏览器
  原生就支持；向导只需要单向的进度流。
- **不扩展 `/api/create` 之外的 API**。feature 枚举、add 流程等有意缺
  席——这样 GUI 保持在"演示 / onboarding"位置，不与 CLI 抢 UX 主线。
