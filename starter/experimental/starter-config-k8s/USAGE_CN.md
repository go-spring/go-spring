# starter-config-k8s 使用说明 — 参考手册

详细使用参考。概览见 [README_CN.md](README_CN.md)。本文每一条行为声明均对照源码核验
（本目录 `starter.go`、`provider.go`、`informer.go`、`provider_test.go`；核心文法
`spring/conf/provider/provider.go:59-104`；刷新链 `spring/gs/internal/gs_conf/conf.go` 与
`spring/gs/internal/gs_app/app.go`）与可运行的 [example/](example/)。ConfigMap/Secret
自身的语义见 [Kubernetes 官方文档](https://kubernetes.io/zh-cn/docs/concepts/configuration/configmap/)——
以下内容都是 go-spring 的增量。

**注意：本 starter 位于 `experimental/` 目录——该目录是"未审核"标记，不是质量分级。**

**激活方式**：blank import 注册 `k8s` 配置 provider（`provider.go:67`）；仅当
`spring.config.import` 中出现 `k8s:` 条目时才真正生效。没有 `enabled` key，也没有其他开关。

---

## 1. 完整工程示例

一个从 ConfigMap 拉取配置、把其中一个 key 绑定到可热刷新的 `gs.Dync` 字段、并在
`kubectl edit` 后无需重启即可重载的服务。与冒烟验证过的 [example/](example/) 同构。文件树：

```
demo/
├── go.mod
├── main.go
└── conf/
    └── app.properties
```

**go.mod**（关键依赖）：

```
require (
    go-spring.org/spring            v1.3.x
    go-spring.org/starter-config-k8s latest
    k8s.io/client-go                 v0.34.x   // starter 间接引入
)
```

**main.go**：

```go
package main

import (
    "fmt"

    "go-spring.org/spring/gs"

    // Blank import 注册 "k8s" 配置 provider，供 spring.config.import 消费
    // （provider.go 的 init）。
    _ "go-spring.org/starter-config-k8s"
)

// Demo 绑定一个来自 ConfigMap 的动态配置字段。demo.message 是展平 ConfigMap 的
// "application.yaml" 条目后产生的顶层绝对 key——不是实例前缀 key。注册为 root
// 对象使容器急切创建它。
type Demo struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
}

func main() {
    demoBean := gs.Provide(&Demo{}).Export(gs.As[gs.Rooter]())
    fmt.Println("demo.message =", demoBean.Interface().(*Demo).Message.Value())
    gs.Run()
}
```

**conf/app.properties** —— 完整且带注释的配置面：

```properties
spring.app.name=config-k8s-demo

# 通过 "k8s" provider 直接从 Kubernetes ConfigMap 导入配置。
# 文法：[optional:]k8s:<kind>/<name>[?namespace=..&key=..&format=..&kubeconfig=..]
#   optional:  无集群可达时以默认值启动（本地开发）
#   kind/name: configmap/app-config
#   namespace: default（query 参数；ServiceAccount 有权读的任意命名空间）
#   key:       只读 ConfigMap 的 "application.yaml" 这一个 data 条目
spring.config.import=optional:k8s:configmap/app-config?namespace=default&key=application.yaml

# 本例只验证 config provider；关闭默认 HTTP server。
spring.http.server.enabled=false
```

**ConfigMap**（取自 [example/deploy/configmap.yaml](example/deploy/configmap.yaml)）：

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
  namespace: default
data:
  application.yaml: |
    demo:
      message: hello-from-configmap
```

**RBAC** —— 直接 apply [example/deploy/rbac.yaml](example/deploy/rbac.yaml)；ServiceAccount
需要目标命名空间内 configmaps（和/或 secrets）的 `get`（初次读取）、`list` + `watch`
（informer）。

**验证**：

```bash
# 0) 完全没有集群——以默认值启动并干净退出（optional: 语义）
go run .                                    # demo.message = none

# 1) 连真实集群、集群外运行：加 &kubeconfig=$HOME/.kube/config
#    （并去掉 optional: 使对象缺失变成致命错误）

# 2) 集群内（apply example/deploy/*.yaml）：
kubectl logs deploy/config-k8s-example       # demo.message = hello-from-configmap

# 3) 热刷新演示——不重启、无卷挂载：
kubectl edit configmap app-config            # 把 demo.message 改成 "v2"
kubectl logs -f deploy/config-k8s-example    # 绑定的 gs.Dync 字段数秒内更新
```

重载路径是真实的：对象上的 informer 在每次 add/update/delete 触发（`informer.go:105-109`），
调用 `RefreshProperties`（`starter.go:55-63`），后者重跑整条 import 链并更新所有
`gs.Dync[T]` 字段（`gs_app/app.go:234-256`）。单测 `TestHotReloadTriggersRefresh`
（`provider_test.go:126-152`）用 fake clientset 证明了该触发。

---

## 2. 装配与时序

### 2.1 import 何时解析——先于 bean，这是设计

```
blank import starter-config-k8s
  ├─ init (provider.go:67): conf.RegisterProvider("k8s", k8sController.Load)
  └─ init (starter.go:29):  gs.Provide(k8sController).Name("k8sController")
                            .Export(gs.As[gs.Rooter]()).Destroy((*k8sCtrl).Destroy)

gs.Run()
  ├─ AppConfig.Refresh (gs_conf/conf.go:83-128) —— 先于任何 bean 装配：
  │    1. 加载基础属性（文件/env/参数）
  │    2. 绑定 ${spring.config.import:=} (conf.go:218) —— 只有一层：
  │       被导入文件里再声明的 import 会被忽略 (conf.go:213-215)
  │    3. 对每个条目，conf.Load 先剥离 [optional:]<provider>: 再调 provider
  │       (provider/provider.go:84-103) → k8sCtrl.Load
  │         a. parseSource (provider.go:85-117)
  │         b. buildClient —— 集群内或 kubeconfig (informer.go:38-59)
  │         c. 经 API server fetch 对象 (provider.go:175-192)
  │         d. ensureWatch —— informer 在返回前装好，初次读取之后立刻落下的
  │            变更不会被漏掉 (provider.go:158-161)
  │         e. parseEntries → flatten → 合入属性存储
  │    └─ 合并后的快照成为所有 value tag 绑定的属性存储
  ├─ 容器装配：k8sController 被注入 *gs.PropertiesRefresher（starter.go:44）；
  │  app.started 置 true (app.go:179-183)
  ├─ Runner/Server 启动；就绪
  └─ SIGTERM → bean 析构 Destroy → manager.stopAll() 停掉全部 informer
```

为什么先于 bean：provider 的输出必须在 `value:` tag 解析之前就进入属性存储，这样来自
ConfigMap 的 key 才能在首次装配时注入普通 bean 字段——这也是 `.env` 与所有 config
provider 都跑在生命周期第 2 步、先于 starter 的原因。容器装配好控制器之前，
`TriggerRefresh` 是无害 no-op（starter.go:52-63）：启动加载已捕获初始状态，且
`RefreshProperties` 在 `started` 之前本就拒绝执行（app.go:247-250）。

### 2.2 watch/refresh 路径逐层走读

1. `ensureWatch` 按 `kind/namespace/name` 去重（`informer.go:76-84`）——同一对象被多次
   import 不会堆叠 informer。
2. 创建命名空间限定、`metadata.name=` field selector 圈定单对象的
   SharedInformerFactory，resync 周期为 0（纯事件驱动；`informer.go:86-93`），按 kind
   挂在 ConfigMaps 或 Secrets 上。
3. Add/Update/Delete 三个 handler 全部汇入 `k8sCtrl.TriggerRefresh`
   （`informer.go:105-109`）。
4. watch 建立是 best-effort：handler 注册或 cache 同步失败时遗忘该 id（后续 Load 可
   重试），只损失该对象的热刷新——静态快照仍会加载（`informer.go:110-125`，
   `provider.go:158-161` 注释）。
5. 装配完成后，每个事件调用 `Refresher.RefreshProperties()` → 全量
   `AppConfig.Refresh` → **每个** provider 重跑自己的 import（ConfigMap 被重新拉取而非
   diff）→ 原子换掉合并存储 → 所有 `gs.Dync[T]` 字段更新。只有 `gs.Dync[T]` 会热刷新；
   普通字段与 `OnProperty` 条件只在启动时生效。

---

## 3. 逐 key 行为参考

本 starter **没有 `value:` tag**——它的全部配置面就是 import 字符串文法加核心的
`spring.config.import` key。整个目录里唯一的 `value:` tag 是 example 的
`value:"${demo.message:=none}"`（example/example.go），绑定的是从 ConfigMap 文档展平出的
顶层绝对 key。

### 3.1 import 字符串文法

总体形态（核心，`provider/provider.go:62-64`）：`[optional:]<provider>:<path>` —— 先切
`optional:`（provider.go:85-88），再按第一个 `:` 切分 provider 与 path（provider.go:89-92）；
裸 path 默认走 `file` provider。本 starter 的 `<path>` 部分由 `parseSource`
（`provider.go:85-117`）解析为：

```
<kind>/<name>[?namespace=..&key=..&format=..&kubeconfig=..]
```

| 参数 | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-------|------|---------|-------------|----------|
| `optional:` 前缀 | 标志 | 关 | 加上后，client 构建失败与对象 NotFound 都只记 Warn 并跳过 → 以默认值启动（provider.go:133-138、150-153）。 | ——（这正是它的用途）；对象名写错会被掩盖，直到去掉前缀。 |
| `kind` | string | — **必填** | path 中第一个 `/` 之前的段，转小写。仅支持 `configmap` 与 `secret`（provider.go:51-54、106-110）。Secret 的 Data 已由 API 层 base64 解码（provider.go:185-188）。 | `deployment/x` → 启动报错 "unsupported k8s config kind"；缺 `/` 或 name 为空 → "must be `<kind>/<name>`"。 |
| `name` | string | — **必填** | 对象名；同时是 informer 的 field selector（informer.go:91）。 | 无 `optional:` 时不存在 → 启动报错 "get configmap/secret … not found"。 |
| `namespace` | query | `default` | 同时作用于 Get 与 informer 的命名空间范围（provider.go:95-96、informer.go:89）。跨命名空间读取需要那里的 RBAC。 | Forbidden → 启动报错（`optional:` 下则静默跳过）。 |
| `key` | query | 全部条目 | 只选对象的一个 data 条目（provider.go:212-214）。⚠ `key=` 模式下，扩展名未知的条目是硬错误、要求 `format=`（provider.go:222-224）；而同一条目在全条目模式下会被静默*跳过*——见 §6。 | 对含裸 `README` 的 ConfigMap 用 `?key=README` → 启动报错 "entry … has no known format; set format="。 |
| `format` | query | 按扩展名 | 为所有读到的条目强制指定解析器（provider.go:215、111-115）。取值必须是 `reader.Has` 认识的格式——parse 阶段即校验。 | 未知取值 → 连集群都还没碰就启动报错 "unsupported k8s config format"。 |
| `kubeconfig` | query | 集群内 | 集群外运行时的 kubeconfig 文件路径（informer.go:43-47）。空 → `rest.InClusterConfig()`（informer.go:49-52）。 | 集群外且未给参数 → 启动报错 "in-cluster config (set kubeconfig when running outside a cluster)"——`optional:` 下为 Warn+跳过。 |

### 3.2 属性 key

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|---------|-------------|----------|
| `spring.config.import` | list | 空 | 核心 key（绑定于 `gs_conf/conf.go:218`）。只有一层：被导入文档里的 `spring.config.import` 会被忽略（`conf.go:213-215`）。多条目时后加载的覆盖先加载的（`conf.go:202`）。 | 缺 `k8s:` 条目 → provider 永不运行（starter 静默不生效）。 |
| ConfigMap/Secret 的 data key（如 `demo.message`） | 任意 | 无 | 由每个解析出的条目展平而来（provider.go:233）；成为顶层绝对属性，任何地方的 `value:"${...}"` tag 都可绑定。 | 条目内容是坏 YAML/properties → 启动报错 "parse entry %q"（provider.go:230-232）。 |
| `spring.http.server.enabled=false` | bool | true | 仅 example 为聚焦配置验证而关掉默认 HTTP server。 | —— |

---

## 4. 验证与故障演练

自动冒烟（`example/check.sh`）分两段：fake clientset 单测 + 集群外干净退出启动
（`optional:` 路径）。下面的演练是集群内对应物，与 example 部署同构。

### 4.1 冷加载

```bash
kubectl apply -f example/deploy/rbac.yaml -f example/deploy/configmap.yaml -f example/deploy/deployment.yaml
kubectl logs deploy/config-k8s-example
# demo.message = hello-from-configmap            <- 来自 ConfigMap，不是默认值
```

本地无集群：`cd example && go run .` 打印 `demo.message = none`（`:=` 默认值）并自行
退出——证明无控制平面时装配与注册依然成立。

### 4.2 watch 推送（热刷新，不重启）

```bash
kubectl edit configmap app-config        # demo.message: hello-v2
kubectl logs -f deploy/config-k8s-example  # 任意 gs.Dync 消费方数秒内看到 hello-v2
```

秒级 vs 卷挂载约 1 分钟的 kubelet 投影延迟：informer 直连 API server（包文档，
`provider.go:24-30`）。删除 ConfigMap 同样触发 informer（DeleteFunc），但随后的 re-import
会失败——属性只是暂时保留上一份好快照，直到下一次刷新成功；把"运行中删除"当事故，
不要当特性。

### 4.3 畸形 ConfigMap 内容

把坏 YAML 放进 `application.yaml` 条目并重启（冷路径）：

```bash
kubectl edit configmap app-config   # 弄坏 YAML
kubectl rollout restart deploy/config-k8s-example
kubectl logs deploy/config-k8s-example   # 启动报错：k8s config: parse entry "application.yaml" ...
```

注意不对称：*运行时*的畸形编辑先让刷新失败（错误记日志、存储不换——
`RefreshProperties` 不做部分更新，app.go:244-246）；进程继续以旧值服务。

### 4.4 optional vs 必填

```properties
spring.config.import=k8s:configmap/app-config?...        # 必填：对象缺失 = 启动失败
spring.config.import=optional:k8s:configmap/app-config?... # 对象缺失 = Warn + 默认值
```

`optional:` 覆盖两类不同失败（provider.go:133-138 与 150-153）：集群/client 不可构建、
对象 NotFound。其余——RBAC Forbidden、内容畸形、`format=` 非法——依然致命。

### 4.5 启动时 API server 不可达

必填模式：`go run .` 带 client 构建或 Get 错误链失败。optional 模式：Warn
`optional config build client failed (skipped)`，加载默认值。启动后 API server 失联：
informer 内部重试（client-go reflector 退避）；刷新停滞直到恢复；已加载快照继续服务。

### 4.6 key 过滤与 format 演练

```bash
# 只读 secret 的 db.properties：
spring.config.import=k8s:secret/app-creds?key=db.properties
# 条目无已知扩展名又被 key= 选中 -> 必须加 &format=properties，否则启动失败
```

由 `TestLoadSecretPropsWithKeyFilter` 与 `TestUnknownExtensionSkippedButKeyFilterErrors`
覆盖（provider_test.go:74-124）。单测套件随时可跑：

```bash
cd starter/experimental/starter-config-k8s && go test -gcflags="all=-N -l" ./...
```

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动报错 `in-cluster config (set kubeconfig when running outside a cluster)` | 本地运行且未给 `kubeconfig=` 参数 | 加 `?kubeconfig=$HOME/.kube/config`，或 `optional:` 跳过 |
| 启动报错 `get configmap … not found` | `name`/`namespace` 写错，或对象未 apply | 改 source 字符串或 apply ConfigMap；用 `optional:` 容忍 |
| 启动报错 `forbidden: User … cannot get/list/watch configmaps` | RBAC 缺失/过窄 | apply example/deploy/rbac.yaml；informer 需要 get+list+watch，且在目标命名空间 |
| 配置能加载但从不热刷新 | informer best-effort 失败（cache 同步）或缺少 watch 权限而 get 正常 | 检查 RBAC 的 `watch`；重启以重试 watch 建立；看 tag `_app_def` 下的 Warn/Error |
| 集群内 `demo.message` 一直是默认值 | import 实际没写进 `spring.config.import`，或 `key=` 选了不存在的条目 | `key=` 无匹配得到的是空（而非失败）的 import——核对 data 条目名 |
| 启动报错 `entry "README" has no known format; set format=` | `key=` 选中了无扩展名条目 | 加 `&format=properties`（等），或去掉 `key=` 让其被跳过 |
| 启动报错 `unsupported k8s config kind "deployment"` | 只支持 `configmap`/`secret` | 二选一 |
| 看不到 starter 的日志 | Debug 级日志被隐藏（provider.go:129 以 Debug 记录解析出的 source） | 调高 tag `_app_def` 的 logger 级别 |

---

## 6. 设计体检表

| 指标 | 数值 |
|--------|-------|
| 配置 key（import 字符串参数） | 7（含 `optional:`） |
| 自有属性 key | 0（仅核心 key `spring.config.import`） |
| 必填参数 | 2（`kind`、`name`） |
| quickstart 前置外部依赖 | 本地 0 / 集群 1 |
| 注意/坑条数 | 4（key/format 严重度不对称、单层 import、运行中删除、watch best-effort） |

设计嫌疑清单（保留原有 + 新增，供审计台账）：

- 无扩展名条目在 `key=` 模式下硬失败，而全条目模式对同一条目静默跳过——同一误配的
  严重度不一致（`provider.go:222-226`）。
- 每个 import 重建一次 clientset（同一 kubeconfig 不共享 client 缓存）——轻度浪费；
  候选按 etcd/nacos 的 `clientFor` 模式去重。
- 刷新是应用级而非对象级：改一个 ConfigMap 会重跑*所有* import（file、etcd、nacos……）
  ——正确但每次推送成本是 O(全部来源)；配置编辑频率下无碍，import 变多时需记住这一点。
- Delete 事件处理是潜在缺口：informer 在删除时触发刷新、随后的 re-import 失败，
  "保留上一份好快照"的行为是涌现的而非设计的。
- 没有自己的可观测面（无日志 tag、无 "watch 存活" 健康指示、无刷新计数指标）——
  静默的配置过期失效模式从外部无法察觉。
