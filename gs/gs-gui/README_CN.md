# gs-gui

[English](README.md) | [中文](README_CN.md)

`gs-gui` 是 [gs](../gs) 的一个外部工具,提供浏览器化的 Go-Spring 项目创建向导。
它是 `gs init` 的一层轻量前端。

## 使用

将其构建并安装到 `gs` 二进制同目录,然后运行:

```bash
gs gui
```

该命令会启动本地 web 服务(默认端口 `8639`,被占用则自动选端口),打印 URL 并
尝试打开默认浏览器。在页面上填写 module 路径与文档语言,点击 **Create project**,
向导会 exec `gs init` 并把执行日志实时回流到页面。

## 工作原理

- 遵循 gs 外部工具协议:二进制命名为 `gs-gui`,与 `gs` 同目录;`gs gui` 会分发到它。
- 单页界面通过 `go:embed` 打进二进制。
- 提交时,服务端 exec 同目录的 `gs init -m <module> --lang <lang>`,并将合并后的
  stdout/stderr 流式回传浏览器。
