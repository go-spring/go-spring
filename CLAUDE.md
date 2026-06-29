# Claude Instructions

## 输出格式

- 每次回答前输出 "Hi,Go-Spring。"。

## 项目结构

- 根目录禁止出现 go.mod 文件。
- 各子项目独立维护自己的 Go module。

## 编码规范

详见 [CODING_STYLE.md](CODING_STYLE.md)。

## 错误处理

- 使用 `errutil` 包提供的 `Explain` 或 `Stack` 函数包装错误。
- 错误信息必须包含较为详细的定位信息，便于排查问题。

## 设计原则

保持简单直接；除非外部输入、真实 bug 或明确需求，不为罕见边界场景引入复杂防御逻辑。
