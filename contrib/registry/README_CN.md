# 服务注册与发现(Go-Spring 风格)

[English](README.md) | [中文](README_CN.md)

一组围绕 **注册中心** 的可运行示例,展示 Go 微服务社区里常见的几种注册中心,
每种都用 **一个 provider + consumer** 以 Go-Spring 的方式接线。这里的重点不是
RPC 框架、也不是协议,而是 **注册中心本身**:provider 启动时把服务 *注册* 进注册
中心;consumer 完全不知道 provider 的 `host:port`,而是从同一个注册中心 *发现* 一
个存活地址。

每个示例都是自包含的 Go module,自带只拉起该注册中心的 `docker-compose.yml`,
可单独运行。

## 有哪些注册中心

| 注册中心 | 示例 | 框架 | 注册驱动 | 默认地址 | 说明 |
| --- | --- | --- | --- | --- | --- |
| **etcd** | [`etcd/`](etcd) | dubbo-go(Triple) | `etcdv3` | `127.0.0.1:2379` | Raft KV;云原生 Go 服务事实上的默认选择。 |
| **Nacos** | [`nacos/`](nacos) | dubbo-go(Triple) | `nacos` | `127.0.0.1:8848` | 注册中心 + 配置中心;自带控制台 `:8848/nacos`。 |
| **ZooKeeper** | [`zookeeper/`](zookeeper) | dubbo-go(Triple) | `zookeeper` | `127.0.0.1:2181` | 经典的 Dubbo 注册中心;久经考验,ZAB 强一致。 |
| **Polaris** | [`polaris/`](polaris) | dubbo-go(Triple) | `polaris` | `127.0.0.1:8091` | 腾讯北极星服务治理平台(发现 + 路由 + 熔断)。 |
| **Consul** | [`consul/`](consul) | Kitex(protobuf) | `registry-consul` | `127.0.0.1:8500` | HashiCorp;DNS/HTTP 发现,带主动 TCP 健康检查。 |

## 注册中心与框架如何配合

注册中心是与框架无关的基础设施,区别只在于每个 RPC 框架如何接入它。

- **dubbo-go** 原生提供 etcd、Nacos、ZooKeeper、Polaris(以及更多)的注册中心
  扩展。切换注册中心是 **纯配置** 的事:改一下
  `spring.dubbo.registries.<id>.protocol` 和 `.address` 即可 —— 这四个 dubbo-go
  示例的应用代码逐字节完全一致。正因如此,它们最适合拿来横向对比注册中心。
- **Kitex** 通过可插拔的 `kitex-contrib` resolver/registrar 做发现。dubbo-go 没
  有 Consul 扩展,所以 Consul 用 Kitex 演示,借助
  `github.com/kitex-contrib/registry-consul`,在 `provider/server.go` 里显式接线
  (没有对应 starter)。

## 每个示例的统一结构

```
                ┌──────────────┐
     注册       │   注册中心   │     发现
  ┌────────────▶│              │◀────────────┐
  │             └──────────────┘             │
  │ 服务名                                    │ 解析出存活的 provider 地址
  │                                          │
┌─┴──────────┐                        ┌──────┴─────┐
│  provider  │◀───────── RPC ─────────│  consumer  │
│ gs.Run()   │                        │ 一次性调用 │
│ 长期运行   │──────────────────────▶│ 断言后退出 │
└────────────┘                        └────────────┘
```

- **provider** 长期运行:`gs.Run()` 驱动其生命周期,收到 SIGTERM 关停时会从注册
  中心注销。
- **consumer** 无服务端:按服务名发现 provider,发起一次调用,对回显值做断言,然
  后退出 —— 因此它的退出码就是冒烟测试的结果。
- provider 与 consumer 各自 `chdir` 到自身目录并加载各自的
  `conf/app.properties`,两者不共享配置文件。

## 运行任意示例

```bash
cd etcd            # 或 nacos / zookeeper / polaris / consul

docker compose up -d          # 只拉起该注册中心
go run ./provider &           # 长期运行,注册服务
go run ./consumer             # 发现、调用、断言、退出

# 或者一次性冒烟测试(拉起 → 调用 → 拆除):
bash scripts/smoke-test.sh
```

每个示例都是可运行的 demo,**不是** 可复用的 starter 模块。与注册中心无关的 RPC
机制(协议、代码生成、可观测)请见 [`../dubbo-go`](../dubbo-go) 和
[`../kitex`](../kitex) 下的框架示例。
