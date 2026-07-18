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

参考 [example.go](example/example.go) 文件，按实例名注入。

```go
import "github.com/casbin/casbin/v2"

type Service struct {
    Enforcer *casbin.Enforcer `autowire:"rbac"`
}
```

### 4. 执行鉴权

```go
ok, err := s.Enforcer.Enforce("alice", "/data", "write")
```

## 配置项

| Key        | 说明                             | 默认值   |
|------------|----------------------------------|----------|
| `model`    | Casbin 模型文件路径              | —        |
| `policy`   | 文件形式的策略文件路径           | —        |
| `autoSave` | 策略变更是否自动写回 CSV 文件    | `true`   |

## 核心功能

[example.go](example/example.go) 构建了一个 RBAC enforcer 并断言：

* **角色继承** —— `alice`（admin）可 `read`/`write`，`bob`（viewer）仅可 `read`。
* **默认拒绝** —— 未知主体、未授权动作一律拒绝。

## 进阶功能

* **多 enforcer**：在 `spring.casbin.*` 下定义多个实例（例如按业务域划分），按 bean 名分别注入。
* **自定义持久化**：默认文件适配器让本 starter 无需数据库。若要用 GORM、Redis 等持久化策略，
  请用对应的 [Casbin 适配器](https://casbin.org/docs/adapters) 自行构建 `*casbin.Enforcer`
  并注册为 bean，而非使用本 group。
