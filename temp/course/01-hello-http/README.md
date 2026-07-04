# 01. 从 Go 原生 HTTP 服务走到 Go-Spring

本目录对应第一篇博客的最小代码。它只保留一个 `/echo` 接口，用来观察从 `http.ListenAndServe` 到 `gs.Run()` 的启动职责变化。

## 运行

```bash
go run .
```

## 验收

```bash
curl http://127.0.0.1:9090/echo
```

期望输出：

```text
BookMan Pro is running
```

## 本篇关注点

- HTTP Handler 仍然使用标准库写法。
- `gs.Run()` 接管配置加载、应用容器、HTTP Server 和优雅关闭。
- 默认 HTTP 端口为 `9090`，后续章节再通过配置文件覆盖。
