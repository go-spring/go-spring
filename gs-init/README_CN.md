# gs-init

`gs-init` 是一个用于初始化 Go 项目的命令行工具。它基于 [go-spring/skeleton](https://github.com/go-spring/skeleton)
项目模板，可以快速创建一个结构完整的 Go 项目。

## 功能特性

- 基于 [go-spring/skeleton](https://github.com/go-spring/skeleton) 模板创建项目
- 自动替换模块名和包名
- 支持指定 git 分支
- 自动生成项目代码

## 安装

- **推荐方式：**

使用 [gs](https://github.com/go-spring/gs) 集成开发工具。

- 单独安装本工具：

```bash
go install github.com/go-spring/gs-init@latest
```

## 使用方法

```bash
# 基本用法，必须指定模块名
gs-init --module=github.com/your_name/your_project

# 指定分支
gs-init --module=github.com/your_name/your_project --branch=main
```

## 参数说明

- `--module`: 指定项目的模块名（必填）
- `--branch`: 指定使用的模板分支，默认为 main

## 工作原理

1. 从 go-spring/skeleton 仓库克隆指定分支的代码
2. 删除 .git 目录以解除与模板仓库的关联
3. 将模板中的占位符替换为实际的模块名和包名
4. 重命名项目目录
5. 运行 `gs gen` 命令生成项目代码

## 许可证

本项目采用 [Apache License 2.0](LICENSE) 许可证。
