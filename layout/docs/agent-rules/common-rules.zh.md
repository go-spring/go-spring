# Go-Spring 通用规约

本文件适用于 Go-Spring 框架仓库,以及所有基于 Go-Spring 的项目。

## 设计原则

保持简单。不要为边界情况添加防御性代码，除非由外部输入、真实 bug 或明确需求驱动。

除非用户明确要求，否则遵循现有代码模式和项目风格。

## 编码风格

参见 [coding-style.md](../coding-style/coding-style.md)，涵盖命名、格式与组织、错误处理、测试、并发、Go 习惯等风格取向。

## 代码卫生

- 代码必须通过 `modernize` 和 `go fix`，不得有任何告警。

## 脚本

- 需要脚本时，优先使用 bash，其次 python。
