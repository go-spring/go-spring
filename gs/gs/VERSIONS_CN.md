# 依赖版本治理(BOM)

Go-Spring 是一个由 60+ 个独立 `go.mod` 模块组成的工作区。缺少统一对齐版本的
地方,共享依赖就会各自漂移--而版本错配已造成过真实故障(go1.26 工具链后缀
让代码生成工具崩溃)。

仓库根的 `versions.yaml` 扮演 Spring 里 BOM 的角色:记录 Go-Spring 官方"祝福
过"的第三方版本,由**维护专用**的 `bomtool`(经 `scripts/versions.sh` 调用)
负责报告(并可选地对齐)偏离该基线的模块。

> 这是对 go-spring 单体仓库自身的治理。它**不是**用户安装的 `gs` 工具集里的
> 命令,刻意不出现在 `gs --help` 中--单模块的用户项目没有 `versions.yaml`、
> 也没有 `go.work`,一个工作区 BOM 工具只会让他们困惑。采用类似 `go.work`
> 单体仓库的团队可自行借鉴。

## versions.yaml

```yaml
go: "1.26"
disabled:
  go:
    - "1.26.0"        # runtime.Version() 的 -X:jsonv2 后缀会让代码生成崩溃
dependencies:
  go.opentelemetry.io/otel: v1.43.0
  google.golang.org/grpc: v1.80.0
  github.com/stretchr/testify: v1.11.1
  # ...只覆盖高频共享依赖
```

内部模块(`go-spring.org/...`)有意不列--工作区靠 `go.work` 解析它们,绝不能
用 `require` 锁版本。

## 命令

全部经 `scripts/versions.sh` 调用(内部执行 `gs/gs` 下的 `go run ./cmd/bomtool`):

| 命令 | 作用 |
| --- | --- |
| `./scripts/versions.sh check` | 只读。打印所有 require 版本偏离基线的模块;有漂移即非零退出,可接入检查脚本卡点。 |
| `./scripts/versions.sh diff` | 只读。按依赖分组展示偏离项(基线版本 + 各偏离模块),供人工整改决策。 |
| `./scripts/versions.sh apply <module>` | 写回**单个**模块的 `go.mod`,把其受治理的 require 对齐到基线。接受模块路径或工作区目录。 |

`apply` 刻意只针对单个模块,让批量整改保持串行,绝不与其他模块的并发改动冲突。
`apply` 之后,在该模块内跑 `go mod tidy` 以整理 `go.sum` 并恢复 `// indirect`
标记。

## 在哪里运行

漂移检查作为 `scripts/check-go-modules.sh`(本仓库维护检查脚本)中的一个步骤
运行,本地检查时即可暴露漂移。本仓库自身没有 CI 流水线;想接入自己 CI 的采用方
可直接调用 `./scripts/versions.sh check`--有漂移即非零退出。

> `versions.yaml` 里的 `disabled` Go 版本列表目前仅作参考,检查尚未强制。
