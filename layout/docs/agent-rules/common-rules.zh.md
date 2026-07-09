# Go-Spring 通用规约

本文件适用于 Go-Spring 框架仓库,以及所有基于 Go-Spring 的项目。

## 设计原则

保持简单。不要为边界情况添加防御性代码，除非由外部输入、真实 bug 或明确需求驱动。

除非用户明确要求，否则遵循现有代码模式和项目风格。

## 编码风格

参见 [coding-style.md](../coding-style/coding-style.md)。

## 代码卫生

- 代码必须通过 `modernize` 和 `go fix`，不得有任何告警。
- 不保留被注释掉的死代码；应删除过时代码，而不是注释掉。

## 全局状态

- 不推荐使用全局变量。Go-Spring 通过 IoC/DI 提供依赖；通过注入获取配置、单例和客户端。

## 错误处理

- **禁止**用 `errors.New` / `fmt.Errorf` 直接构造错误,统一走 `errutil`。
- 业务语义补充用 `errutil.Explain(err, "...")`;调用链跟踪用 `errutil.Stack(err, "Xxx")`。
- 包含足够的上下文，以便定位错误的来源。

## 测试

- 使用 `stdlib/testing` 提供的 `assert`/`require` 辅助函数进行值和错误断言。
- 默认优先使用 `assert`；仅当断言失败会导致后续代码 panic 或失去意义时才使用 `require`（例如解引用前的 nil 检查）。
- 不引入第三方断言库（testify 等）。
- 仅在没有对应断言的场景下才使用原始的 `t.Error`/`t.Fatal` —— 例如超时保护、`select` 分支或无法恢复的初始化失败。

## 脚本

- 需要脚本时，优先使用 bash，其次 python。
