# starter-config-file

[English](README.md) | [中文](README_CN.md)

`starter-config-file` 基于 github.com/fsnotify/fsnotify，将**本地文件系统配置**接入为
Go-Spring 的**可热更新配置源**。空导入一次即注册**两个** Provider，覆盖 Kubernetes
ConfigMap/Secret 挂载的两种形态：

- **`file-watch`** —— 单份配置文档（承载 `application.yaml` 的 ConfigMap key）。要做分层覆盖，
  按优先级每个文件写一行 import，后声明的覆盖先声明的，规则与 `spring.app.imports` 组合其它
  配置源完全一致。
- **`configtree`** —— 标量 key 文件目录（Secret / env 风格 ConfigMap 挂载）。每个文件的路径成为
  带点的属性 key、未解析的内容成为 value；路径唯一，因此天然没有优先级问题。

两者都监听父目录（而非文件本身），所以 kubelet 在 ConfigMap/Secret 更新时的 `..data` 原子软链
替换能被捕获，并触发一次实时属性刷新——无需重启。

本 starter 只承担本地文件/卷监听。远程配置中心（Nacos、etcd、Consul）是独立 starter。

## 安装

```bash
go get go-spring.org/starter-config-file
```

## 快速开始

### 1. 引入包

```go
import _ "go-spring.org/starter-config-file"
```

### 2. 从单个文件导入配置

在配置文件中使用 Provider 语法声明导入
`[optional:]file-watch:<file>`：

```properties
# 监听挂载的 ConfigMap/Secret 单个 key 文件（K8s 推荐用法）：
spring.app.imports=file-watch:/etc/config/application.yaml

# 分层覆盖：后声明的覆盖先声明的。
spring.app.imports=file-watch:/etc/app/application.yaml
spring.app.imports=file-watch:/etc/app/application-prod.yaml
```

`path` 必须是**单个文件**（传目录会被拒绝——目录场景请用 configtree provider）。按扩展名
解析（`.properties`、`.yaml`/`.yml`、`.toml`/`.tml`、`.json`），走共享的 conf reader 注册表。
监听器始终注册在文件所在的**父目录**上，因此 ConfigMap 更新时的 `..data` 软链替换能被正确
捕获。加上 `optional:` 前缀后，即使文件尚不存在应用也能正常启动。

### 3. 绑定动态字段

将导入的配置项绑定到 `gs.Dync[T]` 字段即可实现实时更新：

```go
type Demo struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
}
```

被监听的文件变更时，Provider 的监听器会触发一次应用属性刷新，所有绑定的 `gs.Dync`
字段都会被原子更新。完整流程参见 [example](example/example.go)——它复现了
Kubernetes `..data` 原子软链替换，并断言绑定字段发生热更新。

## Kubernetes 示例

```yaml
volumeMounts:
  - name: config
    mountPath: /etc/config
volumes:
  - name: config
    configMap:
      name: my-app-config
```

```properties
# 指向承载你配置文档的那个具体 key 文件。
spring.app.imports=file-watch:/etc/config/application.yaml
```

执行 `kubectl edit configmap my-app-config`（或触发一次新的发布）更新 volume 后，绑定的
`gs.Dync` 字段会在秒级刷新，无需重启 Pod。

## configtree —— 标量 key 文件目录

当每个文件是**一个标量值**（而非整份配置文档）时——即 Kubernetes Secret 或 env 风格 ConfigMap
挂载的形态——用 `configtree`。它会遍历目录树，每个叶子文件成为一个属性：key 为带点的相对路径，
value 为 trim 后的**裸内容**（不解析）。

```properties
# 一个 Secret 挂载：db.user、db.password、server.port（每个文件一个值）
spring.app.imports=configtree:/etc/secret
```

```
/etc/secret/
  db.user          -> "alice"        # 属性 db.user=alice
  db.password      -> "s3cr3t"       # 属性 db.password=s3cr3t
  server.port      -> "8080"         # 属性 server.port=8080
```

K8s Secret/ConfigMap 挂载是扁平的（一个 key 一个文件；key 名允许含点，所以 `db.user` 是合法 key）。
`configtree` 也支持真正的嵌套树（`db/user` → 属性 `db.user`）；名字以 `.` 开头的条目
（`..data`、时间戳目录）在每一层都会被跳过。不接受 `?format=`——value 就是裸字符串。完整热更新
流程见 [example-configtree](example-configtree/example.go)。

### file-watch 与 configtree 的选择

| 形态 | Provider | 优先级 |
|---|---|---|
| 一整份配置文档 | `file-watch:<file>` | 多文件靠 `spring.app.imports` 行序叠加 |
| 大量标量 key 文件 | `configtree:<dir>` | 无需——路径唯一，key 不会冲突 |

## 工作原理

- 启动时，`spring.app.imports` 会调用 `file-watch` / `configtree` Provider：读取来源，并在其
  **父目录**（`configtree` 是树中的每个目录）上启动监听器。
- Kubernetes 更新挂载的 ConfigMap/Secret 时，会先写入一个新的带时间戳的数据目录，再原子地把
  `..data` 软链重命名指向它。你 import 的 key 文件是经 `..data` 解析的软链，每次更新其 inode
  都会变——这正是监听必须落在**父目录**（稳定）而非**文件本身**（每次更新 inode 都被替换）上的原因。
- 变更触发监听器，回调框架的 `PropertiesRefresher`：重新加载所有配置源（重跑本 Provider），
  并通过两阶段原子提交重新绑定所有 `gs.Dync` 字段。
