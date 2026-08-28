# starter-config-file 使用说明 — 参考手册

详细使用参考。总览见 [README_CN.md](README_CN.md)。所有行为声明均对照 starter 源码
（`starter.go`、`filewatch.go`、`configtree.go`）、gs 核心
（`spring/conf/provider/provider.go`、`spring/gs/internal/gs_conf/conf.go`、
`spring/gs/internal/gs_app/app.go`）以及冒烟通过的 [example/](example/) 与
[example-configtree/](example-configtree/)（两个 `check.sh` 均为绿色）核对。

**激活方式**：一次空导入在共享的 watch+refresh 桥上注册**两个** configuration provider，
各自由 `spring.config.import` 中的对应条目独立激活：

- `file-watch:<file>` — 每条 import 一个配置文档（承载 `application.yaml` 的 ConfigMap key），
  按扩展名解析（filewatch.go:50）。
- `configtree:<dir>` — 标量 key 文件组成的目录树（Secret / env 风格 ConfigMap 挂载）；
  每个叶子文件是一条 property，key 为其点分相对路径（configtree.go:43）。

本 starter 的核心是**免重启热更新**：两个 provider 都 watch 父目录，因此 kubelet 更新
ConfigMap/Secret 时的原子 `..data` 符号链接交换会被转成 `gs.Dync` 字段的实时刷新。
远程配置中心（Nacos/etcd/Consul）是独立 starter。

---

## 1. 完整工程示例

一个同时演示两种来源的服务：数据库凭据来自 Secret 风格挂载（`configtree`），应用配置
来自 ConfigMap 文档（`file-watch`），全部绑定 `gs.Dync` 字段以支持热更新。文件树：

```
demo/
├── go.mod
├── main.go
├── config.go
├── conf/
│   └── app.properties
└── mount/                  # 按 K8s projected volume 的布局摆放
    ├── application.yaml -> ..data/application.yaml
    ├── db.user           -> ..data/db.user
    ├── db.password       -> ..data/db.password
    └── ..data            -> ..2025_...   （符号链接，更新时被原子交换）
```

**go.mod**（关键依赖）：

```
require (
    go-spring.org/spring            v1.3.x
    go-spring.org/starter-config-file latest
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-config-file"
)

func main() { gs.Run() }
```

**config.go** — 把挂载值绑定为动态字段：

```go
package main

import "go-spring.org/spring/gs"

// DbConfig 读取 Secret 风格的 configtree 挂载。每个 key 文件是一条
// property；值是原始字符串（绝不按 yaml/properties 解析）。
// Dync 字段在 watch 事件时热更新；普通字段只有启动期一次生效。
type DbConfig struct {
    User     gs.Dync[string] `value:"${db.user:=none}"`
    Password gs.Dync[string] `value:"${db.password:=}"`
}

// AppSettings 读取一条经 file-watch 导入的 ConfigMap key 文档，
// 按扩展名解析（此处 .yaml -> 嵌套 key 展平为点分 key）。
type AppSettings struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
    Port    gs.Dync[string] `value:"${server.port:=none}"`
}

func init() {
    // Export 成 gs.Rooter 让容器急切创建这些 bean
    // （无注入点的未 Export Provide bean 在 prod 会被裁剪）。
    gs.Provide(&DbConfig{}).Export(gs.As[gs.Rooter]())
    gs.Provide(&AppSettings{}).Export(gs.As[gs.Rooter]())
}
```

**conf/app.properties** — 完整且带注释的配置面：

```properties
# 两条 import，逗号分隔；后声明的覆盖先声明的
# （与所有 spring.config.import 来源的分层规则一致）。
spring.config.import=file-watch:./mount/application.yaml,optional:configtree:./mount

# optional 前缀：即使本机没有 Secret 挂载也能启动。
# 本示例只验证配置 provider，因此不开 HTTP server：
spring.http.server.enabled=false
```

首次启动前先摆放初始挂载（生产环境由 kubelet 完成）：

```bash
mkdir -p mount
printf 'demo:\n  message: initial\nserver:\n  port: "8080"\n' > mount/application.yaml
printf 'alice'   > mount/db.user
printf 's3cr3t'  > mount/db.password
```

**验证**（冷加载 + 热更新演练 —— 与两个 example 的 `check.sh` 同构，失败即非零退出）：

```bash
cd example && ./check.sh                    # file-watch provider，自动化
cd ../example-configtree && ./check.sh      # configtree provider，自动化

# 或手动驱动：
go run ./example -manual                    # 保持运行
echo 'demo:\n  message: flipped' > example/mount/application.yaml
# ~1s 内 gs.Dync 字段变为 "flipped" —— 无需重启
```

---

## 2. 装配与时序

### 2.1 import 在何处解析 —— bean 装配之前，以及为什么这很重要

```
import starter-config-file
  ├─ init: conf.RegisterProvider("file-watch", controller.Load)      (filewatch.go:50)
  ├─ init: conf.RegisterProvider("configtree", controller.LoadConfigTree)  (configtree.go:43)
  └─ init: gs.Provide(controller).Export(As[gs.Rooter]())            (starter.go:57)
        │
gs.Run() → App.Start()                                               (app.go:285)
  1. Provide PropertiesRefresher 与 ContextProvider bean
  2. app.p.Refresh(): 加载 conf/app.* 文件，解析 spring.config.import
     → provider.Load 在这一步执行：读文件/目录树，ensureWatch() 在父目录
       /树的每个目录上安装 fsnotify watcher
  3. initLog，随后 IoC 容器装配 —— controller 的 *gs.PropertiesRefresher
     autowire 字段此刻才被注入 (starter.go:67)
  4. app.started = true → RefreshProperties 从此合法                (app.go:309)
  5. Runners、Servers、就绪信号
  └─ 任一被 watch 目录有事件: watchLoop → TriggerRefresh
       → Refresher.RefreshProperties(): 重新加载全部来源、按优先级合并、
         原子更新所有 gs.Dync 字段                                   (app.go:247)
```

controller 之所以同时是个 bean，尽管配置加载发生在 bean 装配前：`PropertiesRefresher`
在装配后才存在，因此 `TriggerRefresh` 在装配前被刻意设计为 no-op（starter.go:76-80）——
早期的 watch 事件被无害丢弃，因为启动加载刚捕获过状态。正是这种"provider 先于 bean
注册、refresh 桥装配后注入"的拆分，使两个 provider 挂在同一个 controller 单例上。

其它时序事实：

- import 在配置加载步骤内解析（gs_conf/conf.go 的 `loadFileImports`），即**先于** bean
  绑定 —— 必填 import 失败时在任何 bean 代码运行前中止启动。
- import 只处理**一层**：被导入文件里声明的 `spring.config.import` 会被静默忽略
  （gs_conf/conf.go:210-215）。
- import 列表逗号分隔并去重（`Imports []string value:"${spring.config.import:=}"`，
  gs_conf/conf.go:218）。
- watcher 按目录去重（`watched` map，starter.go:86-106）：启动加载和之后的每次刷新都会
  重新调用 `Load`/`LoadConfigTree`，不去重则每次刷新叠加一个 fsnotify watcher。

### 2.2 一次文件变更，逐层走读

1. 编辑器或 kubelet 写入新的时间戳目录并把 `..data` rename 到它上面（原子）。
2. fsnotify 在**目录上**送出 CREATE/RENAME 事件（文件 inode 已变 —— 这正是 watch 挂在
   父目录而非文件本身的原因，filewatch.go:80-86）。
3. `watchLoop` 对每个事件都响应、不过滤文件名（这是正确的：K8s 更新表现为 `..data` 上的
   事件而非 key 文件上的事件 —— starter.go:112-116）→ `TriggerRefresh`。
4. `RefreshProperties` 在应用未完成装配时拒绝执行（app.go:248）；否则重新跑完整配置
   加载 —— 两个 provider、环境变量、命令行参数 —— 按优先级合并后原子替换 `gs.Dync`
   值。一次交换通常产生两个事件（临时符号链接 create + rename），因此每次更新会看到
   两行 `loaded ... config` 日志：无防抖，属刻意设计。

---

## 3. 逐 key 行为参考

starter 自身**没有任何 property key** —— 本仓库内全部 `value:` tag 都属于 example bean
（`${demo.message:=none}` 等），是普通的绝对 property 引用而非 starter key。整个配置面就是
gs 核心解析的 import 字符串文法 `[optional:]<provider>:<path>`（provider.go:74-104）。

### 3.1 import 字符串文法

| 片段 | 类型 | 默认 | 行为 / 联动 | 配错后果 |
|------|------|------|-------------|----------|
| `optional:` 前缀 | flag | 无 | 在 provider 拆分前解析；provider 的 `Load` 收到 `optional=true`，路径缺失时打 Warn 日志并返回 `(nil, nil)`（filewatch.go:67-70、configtree.go:61-64）。 | 缺前缀且路径缺失 → stat 错误，启动中止。 |
| `file-watch` provider | 枚举 | `file` | 每条 import 一个文档，按扩展名识别格式。必须是文件。 | 拼写错误 → 启动报 `unsupported provider type`。注意核心对裸路径的默认 provider 是 `file` 而非 `file-watch` —— 裸路径没有 watch。 |
| `configtree` provider | 枚举 | `file` | 目录树；每个非点前缀叶子文件 = 一条 property。必须是目录。 | 指向文件 → 显式报错并指向 `file-watch`（configtree.go:68-71）。 |
| `<path>`（file-watch） | 文件路径 | — | 加载前先解析 property 占位符（`conf.Resolve`，gs_conf/conf.go:229）。扩展名经共享 reader registry 路由：`.properties/.yaml/.yml/.toml/.tml/.json`。父目录被 watch。 | 目录路径 → 报错并指向 `configtree`（filewatch.go:75-78）。不支持的扩展名（`.md`）→ 读取错误（有单测覆盖）。 |
| `<path>`（configtree） | 目录路径 | — | key = 点分相对路径（`db/user` 文件 → `db.user`；名为 `db.user` 的平面文件同样 → `db.user`，因为 K8s key 可含点）；值 = 文件内容 `TrimSpace` 后的原文，不做解析（configtree.go:116-125）。树内每个目录都被 watch。⚠ 看似数字/布尔的值仍是原始字符串 —— 在业务代码里自行转换。 | 期望 key 文件内是 yaml 解析 → 字面文本成为值。 |
| 点前缀条目 | 过滤规则 | 跳过 | `..data`、`..时间戳` 目录与 dotfile 在每层被跳过；根目录本身永不跳过（configtree.go:103-108）。符号链接叶子会被跟随（`os.ReadFile`）。 | 无 —— 这正是 K8s 挂载保持干净的原因。 |
| import 列表顺序 | 列表 | — | `spring.config.import` 逗号分隔、去重；**后声明覆盖先声明**；import 覆盖导入文件自身的 key（分层存储，gs_conf/conf.go:244-251）。⚠ 不存在任何 query 参数 —— 来源字符串就是裸路径。 | 顺序写反 → 分层覆盖静默反转。 |
| `spring.http.server.enabled` | bool | true | gs 核心 key，非本 starter 所有；示例关闭它因为演示没有 HTTP 面。 | 保持 true → 默认 server 起在 :8080。 |

### 3.2 不存在的东西

没有刷新间隔、防抖、include/exclude 或格式 query 参数；没有 `enabled` key；没有专属
log tag（用共享的 `_app_def` 基础设施 tag）；没有健康检查；没有指标。

---

## 4. 验证与故障演练

每个演练均与冒烟验证过的 example（`example/check.sh`、`example-configtree/check.sh`，
均已确认绿色）同构。

### 4.1 冷加载

```bash
go run ./example            # 日志: "loaded file-watch config from file=./mount/application.properties keys=1"
                            #      "initial value: initial"  <- 绑定的 gs.Dync 看到值
go run ./example-configtree # 日志: "loaded configtree from dir=./mount keys=3"
```

### 4.2 编辑触发热更新（核心场景）

```bash
go run ./example -manual &     # server 保持运行
echo 'demo.message=flipped' > example/mount/application.properties
sleep 1
# 日志再次出现 "loaded file-watch config ... keys=1"（两次：无防抖），
# 读取 Dync 字段的 handler 此刻看到 "flipped"。
```

原子变体（kubelet 的做法，`check.sh` 已自动化）：写新的 `..2025_...` 目录，符号链接
`..data_tmp` 指向它，`rename(..data_tmp, ..data)`。尽管 key 文件 inode 变了，目录 watch
照样捕获交换。本地 ~1s 内观察到热更新；Kubernetes 上需叠加 kubelet 的投影同步延迟
（默认 ~1 分钟）。

### 4.3 坏文件处理

```bash
echo 'demo: [broken' > example/mount/application.yaml   # 非法 yaml
```

下一次刷新的 `Load` 失败；`RefreshProperties` 在**任何**部分更新之前中止
（app.go:246-255：校验失败 ⇒ 不做部分更新），因此最近一次的正确值继续生效。
刷新错误现在由 `TriggerRefresh` 打 WARN（`property refresh after file change failed,
previous snapshot retained: ...`）；watcher 创建失败与 fsnotify channel 错误同样各打 WARN。
反之，启动时必需文件损坏会以 `file-watch: read ... failed` 大声失败。

### 4.4 optional 与 required

```bash
spring.config.import=optional:file-watch:/etc/config/application.yaml
```

- 存在 → 正常加载；缺失 → Warn `optional config path ... not found (skipped)`，启动继续。
- 不带 `optional:` → 路径缺失以 stat 错误中止启动。
- 路径存在但类型错误（文件 vs 目录）则无论是否 `optional:` 都报错 —— optionality 只
  覆盖"不存在"（os.IsNotExist 判断，filewatch.go:67）。

### 4.5 可观测信号

log tag `_app_def`（基础设施默认）：每次加载 —— `loading ... from <path>`（Debug）、
`loaded ... keys=<n>`（Info）、`optional ... not found (skipped)`（Warn）。无指标/span；
刷新次数只能从重复的 `loaded` 行推断。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动失败: `file-watch: stat ... failed` | 必填路径缺失 | 补文件，或加 `optional:` 前缀。 |
| 启动失败: `expects a single file, got directory` / `expects a directory, got file` | 路径形态与 provider 不匹配 | 互换 `file-watch:` ⇄ `configtree:` —— 错误文本会指明正确的一方。 |
| 启动失败: `unsupported provider type` | provider 名拼错，或以为裸路径等于 file-watch | 裸路径走 gs 内置 `file` provider，**没有 watch**；必须显式写 `file-watch:`。 |
| 值从不热更新 | watcher 创建失败（best-effort 降级为静态快照，现打一条点名目录的 WARN）或 fsnotify/inotify 配额耗尽 | 检查 WARN 日志与 `ulimit`/inotify 限制；编辑后看是否缺少 `loaded` 行；重启可恢复。 |
| 编辑生效一次后配置"卡住" | 后续编辑破坏了解析；刷新错误被吞（§4.3） | 修好文件；等下一行 `loaded ... config` 出现确认恢复。 |
| 值变了但字段没动 | 绑成普通字段而非 `gs.Dync` | 只有 `gs.Dync[T]` 热更新；普通 `value:` tag 仅启动期生效。 |
| 嵌套 import 不生效 | 被导入文件里的 import 被忽略（只有一层，gs_conf/conf.go:210-215） | 所有 import 声明在 `conf/app.*` 里。 |
| 覆盖顺序不对 | 后声明的 import 胜出；import 胜过导入文件自身 | 调整逗号列表顺序 —— 没有优先级参数。 |
| 出现多余的 `..data` 类 key | 不应出现 —— 点前缀条目都被跳过 | 若出现，说明该路径不是 `configtree` 在服务（例如裸 `file:` import）。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 自有配置 key | 0（只有文法: `[optional:]<provider>:<path>`） |
| 必填 | 0（一旦指名 provider 则路径必填） |
| quickstart 前置外部依赖 | 0 |
| "注意/坑" 条数 | 6 |

设计嫌疑清单（保留上一轮审计条目并新增 —— 供设计裁决账本）：

- watcher 创建失败仍退化为静态快照，但现在会打一条点名目录的 WARN（本轮已修），降级可见。
- 每个目录事件都触发全量刷新（不按文件名过滤、无防抖）—— 正确但在编辑风暴下啰嗦；
  每次 K8s 交换会触发 ~2 次全量重载。
- `TriggerRefresh` 现在会把 `RefreshProperties` 的错误打成 WARN（本轮已修），被 watch 的
  文件损坏时不再静默降级。
- `optional:` 语义落在各 provider 内（两个 Load 函数重复 os.IsNotExist 判断）而非
  provider 框架 —— 轻度重复。
- provider 框架的裸路径默认（`file`）遮蔽了本 starter 的用途；写 `./config.yaml` 的
  用户得到无 watch 且无提示。
- K8s 投影延迟（~1 分钟）超出 starter 控制范围；API-watch 替代方案在 starter-config-k8s
  —— 仅交叉引用，此处无可动作。
