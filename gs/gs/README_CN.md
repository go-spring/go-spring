# Go-Spring 工具管理器（gs）

[English](README.md) | [中文](README_CN.md)

Go-Spring 工具管理器（gs）是一个用于管理和使用 Go-Spring 生态系统中各种工具的命令行程序。

## 安装

```
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/go-spring/gs/HEAD/install.sh)"
```

该脚本会自动安装以下工具：

- `gs`: 工具管理器本身
- `gs-init`: 创建新的 Go-Spring 项目
- `gs-gen`: 根据 idl 文件生成代码
- `gs-mock`: 根据配置生成 mock 代码

### 环境要求

- Go 语言环境 (1.24+)
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

- 创建新项目: `gs init ...`
- 生成 idl 代码: `gs gen ...`
- 生成 mock 代码: `gs mock ...`

### 查看工具帮助

```shell
gs <tool> --help
```

## 工作原理

工具管理器会在其所在目录（通常是 `$GOPATH/bin`）查找以 `gs-` 为前缀的可执行文件，并将其作为可用工具进行管理。
当用户调用某个工具时，管理器会执行对应的可执行文件并传递参数。

## 许可证

本项目采用 Apache License 2.0 许可证。详情请见 [LICENSE](LICENSE) 文件。