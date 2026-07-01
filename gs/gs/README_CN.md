# Go-Spring 工具管理器（gs）

[English](README.md) | [中文](README_CN.md)

Go-Spring 工具管理器（gs）是一个用于管理和使用 Go-Spring 生态系统中各种工具的命令行程序。

## 安装

```
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/go-spring/go-spring/HEAD/gs/gs/install.sh)"
```

脚本会安装 `gs` 本体（内置 `init`、`gen`、`add` 子命令）。

### 可选的外部工具

以下工具通过 `gs <tool>` 转发调用，按需单独安装：

- `gs-http-gen`: HTTP IDL 代码生成器（由 `gs gen` 调用）
- `gs-mock`: 根据配置生成 mock 代码

### 环境要求

- Go 语言环境 (1.26+)
- GOPATH 和 GOBIN 正确配置
- GOBIN 路径需要添加到系统 PATH 中

## 使用方法

安装完成后，您可以使用以下命令：

```shell
gs --help
```

这将显示所有可用的工具及其版本和描述信息。

### 使用特定工具

```shell
gs <tool> [args]
```

例如：

- 创建新项目: `gs init -m github.com/you/hello`
- 生成 idl 代码: `gs gen`（需在包含 `gs.json` 和 `idl/` 的项目根目录下执行）
- 生成 mock 代码: `gs mock ...`（需已安装 `gs-mock`）

### 查看工具帮助

```shell
gs <tool> --help
```

## 工作原理

工具管理器直接内置 `init`、`gen`、`add` 子命令；对于其他命令，会在其所在目录（通常是 `$GOPATH/bin`）查找以 `gs-` 为前缀的可执行文件并转发调用。

## 许可证

本项目采用 Apache License 2.0 许可证。详情请见 [LICENSE](LICENSE) 文件。