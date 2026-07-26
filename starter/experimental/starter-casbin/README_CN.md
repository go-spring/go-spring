# starter-casbin

[English](README.md) | [中文](README_CN.md)

> 项目已正式发布，欢迎使用！

`starter-casbin` 基于 [Casbin](https://casbin.org) 封装访问控制能力，
让 Go-Spring 应用可以方便地接入鉴权（RBAC/ABAC/ACL）。
Enforcer 以 bean 形式注册，完全通过注入方式使用。

## 安装

```bash
go get go-spring.org/starter-casbin
```

## 快速开始

### 1. 引入 `starter-casbin` 包

参考 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-casbin"
```

### 2. 准备模型与策略

Casbin 需要一个[模型文件](example/conf/model.conf)（匹配规则）和一个
[策略文件](example/conf/policy.csv)（具体规则）。随后在[配置文件](example/conf/app.properties)中
声明一个 enforcer 实例：

```properties
spring.casbin.rbac.model=./conf/model.conf
spring.casbin.rbac.policy=./conf/policy.csv
```

最后一段 key（`rbac`）即为 bean 名称。

### 3. 注入 Enforcer

参考 [example.go](example/example.go) 文件，按实例名注入。注入的 bean 是
`*StarterCasbin.Enforcer`，内嵌 `*casbin.Enforcer`，因此可直接调用
`Enforce`、`AddPolicy` 等常用方法。

```go
import StarterCasbin "go-spring.org/starter-casbin"

type Service struct {
    Enforcer *StarterCasbin.Enforcer `autowire:"rbac"`
}
```

### 4. 执行鉴权

```go
ok, err := s.Enforcer.Enforce("alice", "/data", "write")
```

## 配置项

| Key        | 说明                                                              | 默认值   |
|------------|-------------------------------------------------------------------|----------|
| `model`    | Casbin 模型文件路径                                               | —        |
| `policy`   | 文件形式的策略文件路径；设置 `adapter` 后忽略                      | —        |
| `autoSave` | 策略变更是否自动写回存储                                          | `true`   |
| `adapter`  | 通过 `RegisterAdapter` 注册的 `persist.Adapter` 名称（DB/文件/…） | —        |
| `watcher`  | 通过 `RegisterWatcher` 注册的 `persist.Watcher` 名称（热更新）    | —        |

## 核心功能

[example.go](example/example.go) 构建了一个 RBAC enforcer 并断言：

* **角色继承** —— `alice`（admin）可 `read`/`write`，`bob`（viewer）仅可 `read`。
* **默认拒绝** —— 未知主体、未授权动作一律拒绝。
* **热更新** —— 由 peer 追加一条授权并通知 watcher，enforcer 重新加载策略，
  新授权无需重启即可生效。

## 进阶功能

* **多 enforcer**：在 `spring.casbin.*` 下定义多个实例（例如按业务域划分），按 bean 名分别注入。
* **可插拔持久化**：默认文件适配器让本 starter 无需数据库。若要用 GORM、Redis 等持久化策略，
  在启动阶段按名称注册一个 [Casbin 适配器](https://casbin.org/docs/adapters)，再让实例指向它：

  ```go
  func init() {
      StarterCasbin.RegisterAdapter("gorm", gormAdapter)
  }
  ```
  ```properties
  spring.casbin.rbac.adapter=gorm
  ```

  本 starter 刻意不引入任何存储驱动——在应用内注册适配器，可避免把 GORM/Redis/etcd 拖进
  只需文件策略的项目。由于 gs.Group 工厂无法注入其他 bean，适配器/watcher 通过包级注册表
  按名称查找。
* **热更新 / 多实例同步**：用 `RegisterWatcher` 注册一个 [Casbin watcher](https://casbin.org/docs/watchers)，
  并设置 `spring.casbin.<inst>.watcher=<name>`。当 peer 通知策略变更时，enforcer 会自动调用
  `LoadPolicy`。watcher 的后台资源会在关闭时由 starter 的 destroy 回调释放。
