# starter-config-file

[English](README.md) | [中文](README_CN.md)

`starter-config-file` 基于 github.com/fsnotify/fsnotify，将挂载目录（或单个文件）接入为
Go-Spring 的**可热更新配置源**。空导入该包即注册一个 `file-watch` 配置 Provider：
启动时从该路径加载应用配置，并在文件变更时热更新——无需重启。

其主要面向 **Kubernetes**：把 `ConfigMap` 或 `Secret` 以 volume 形式挂载后，无需任何自定义
代码即可热更新。kubelet 更新此类 volume 时会**原子地替换 `..data` 软链**，本 Provider 的
目录监听器会捕获该变化并触发一次实时属性刷新。

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

### 2. 从挂载路径导入配置

在配置文件中使用 Provider 语法声明导入
`[optional:]file-watch:<path>[?format=..]`：

```properties
# 监听挂载的 ConfigMap/Secret 目录（K8s 推荐用法）：
spring.app.imports=file-watch:/etc/config
```

`path` 既可以是**目录**（目录内每个可识别文件都会被读取并合并），也可以是**单个文件**。
两种情况下监听的都是**目录**，因此 ConfigMap 更新时的 `..data` 软链替换能被正确捕获。

查询参数：

| 参数     | 默认值        | 说明                                                                 |
|----------|---------------|----------------------------------------------------------------------|
| `format` | 按文件后缀    | 强制所有文件使用指定格式：`properties`/`yaml`/`toml`/`json`。当 ConfigMap 的 key 没有后缀时使用。 |

默认按后缀解析（`.properties`、`.yaml`/`.yml`、`.toml`/`.tml`、`.json`），后缀未知的文件会被
跳过。加上 `optional:` 前缀后，即使路径尚不存在应用也能正常启动。

### 3. 绑定动态字段

将导入的配置项绑定到 `gs.Dync[T]` 字段即可实现实时更新：

```go
type Demo struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
}
```

被监听的文件变更时，Provider 的监听器会触发一次应用属性刷新，所有绑定的 `gs.Dync`
字段都会被原子更新。完整流程参见 [example-config](example-config/example.go)——它复现了
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
spring.app.imports=file-watch:/etc/config
```

执行 `kubectl edit configmap my-app-config`（或触发一次新的发布）更新 volume 后，绑定的
`gs.Dync` 字段会在秒级刷新，无需重启 Pod。

## 工作原理

- 启动时，`spring.app.imports` 会调用 `file-watch` Provider：读取挂载路径、解析各文件，并
  启动一个**目录**监听器。
- Kubernetes 更新挂载的 ConfigMap/Secret 时，会先写入一个新的带时间戳的数据目录，再原子地把
  `..data` 软链重命名指向它。监听**目录**（而非单个文件）正是能观察到该变化的关键——名称以
  `.` 开头的条目（`..data`、带时间戳的目录）会被跳过，而真正的 key 软链会通过 `..data` 被读取。
- 变更触发监听器，回调框架的 `PropertiesRefresher`：重新加载所有配置源（重跑本 Provider），
  并通过两阶段原子提交重新绑定所有 `gs.Dync` 字段。
