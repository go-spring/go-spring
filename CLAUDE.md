# Claude Instructions

每次回答都要求输出 "Hi,Go-Spring"，然后再回答。

项目根目录下禁止出现 go.mod 文件；本项目使用 mono 仓库管理，各子项目独立维护自己的 Go module。

编码规范见 [CODING_STYLE.md](CODING_STYLE.md)。

优先保持实现简单直接；除非外部输入、真实 bug 或明确需求要求，不要为罕见边界场景引入复杂防御逻辑。
