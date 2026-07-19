# 依赖版本治理(BOM)

Go-Spring 是一个由 60+ 个独立 `go.mod` 模块组成的工作区。缺少统一对齐版本的
地方,共享依赖就会各自漂移——而版本错配已造成过真实故障(go1.26 工具链后缀
让代码生成工具崩溃)。

仓库根的 `versions.yaml` 扮演 Spring 里 BOM 的角色:记录 Go-Spring 官方"祝福
过"的第三方版本,`gs versions` 负责报告(并可选地对齐)偏离该基线的模块。

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

内部模块(`go-spring.org/...`)有意不列——工作区靠 `go.work` 解析它们,绝不能
用 `require` 锁版本。

## 命令

| 命令 | 作用 |
| --- | --- |
| `gs versions check` | 只读。打印所有 require 版本偏离基线的模块;有漂移即非零退出,可接入 CI 卡点。 |
| `gs versions diff` | 只读。按依赖分组展示偏离项(基线版本 + 各偏离模块),供人工整改决策。 |
| `gs versions apply <module>` | 写回**单个**模块的 `go.mod`,把其受治理的 require 对齐到基线。接受模块路径或工作区目录。 |

`apply` 刻意只针对单个模块,让批量整改保持串行,绝不与其他模块的并发改动冲突。
`apply` 之后,在该模块内跑 `gs go mod tidy` 以整理 `go.sum` 并恢复 `// indirect`
标记。

## CI 钩子(建议)

在 check/CI 脚本里跑这个只读检查,尽早发现漂移。它**不**接进本仓库的构建——
各项目按需自行采用:

```sh
# 在 check.sh / CI 流水线中 —— 版本漂移则让构建失败
gs versions check
```

当任一受治理模块偏离 `versions.yaml` 时,`gs versions check` 返回非零退出码,
多数 CI 会将其呈现为失败步骤。再配合定期 `gs versions diff` 复查,来决定何时
提升基线本身。
