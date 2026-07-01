# gs

`gs` 目录存放 Go-Spring 生态的命令行工具和项目脚手架。

- `gs` 是主入口，内置 `init`、`gen`、`add` 子命令，并会自动分发同目录下前缀为 `gs-` 的外部工具（如 `gs-http-gen`、`gs-mock`）。
- `gs-http-gen`、`gs-mock` 保持独立模块，各自维护，用于 HTTP IDL 处理和测试 mock 代码生成等场景。
