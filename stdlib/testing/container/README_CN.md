# container

[English](README.md) | [中文](README_CN.md)

`container` 是用于**切片测试**的 Testcontainers 风格 helper:在 Go 测试内部用
Docker 容器起一个真实依赖(redis、mysql……),测试结束时自动销毁。

## 与 starter 的 `check.sh` 有何不同

starter 的 `check.sh` 是进程外的 shell 冒烟测试。本 helper 面向进程内
`go test`:[`Run`](container.go) 会阻塞直到容器就绪,用 `t.Cleanup` 注册清理,并
返回**动态映射**的 host:port 让测试直接连接。无固定端口,无需手动 `docker rm`。

## 零依赖,裸 `docker`

helper 通过 `os/exec` 调用 `docker` CLI,而不引入 Docker SDK。这样既守住 `stdlib`
的零依赖约定,又复用了仓库的 Docker 约定(裸 `docker` 二进制,无 compose-v2 插件)。
拉取镜像所需的代理从环境继承——请在 `go test` 前 export,与 starter 的 `check.sh`
脚本做法一致。

务必用 `SkipIfNoDocker(t)` 兜底,让无 Docker 环境下的测试干净跳过。

## 用法

```go
func TestRedis(t *testing.T) {
    container.SkipIfNoDocker(t)

    addr := container.Redis(t) // redis:7,已映射端口,已注册清理
    // ... 连接 addr 并断言 ...
}
```

或用 `Run` 驱动任意镜像:

```go
c := container.Run(t, container.Request{
    Image:        "mysql:8",
    Env:          map[string]string{"MYSQL_ROOT_PASSWORD": "secret"},
    ExposedPorts: []string{"3306"},
    WaitFor:      container.WaitForLog("ready for connections", 90*time.Second),
})
dsn := "root:secret@tcp(" + c.Endpoint("3306") + ")/"
```

### 就绪等待(`WaitFor`)

- `WaitForListeningPort(port, timeout)`——最省、最通用;等到映射端口可建立 TCP 连接。
- `WaitForLog(substr, timeout)`——等待某行日志,适用于"能连上但还没真正就绪"的服务
  (跑初始化脚本的数据库)。

### 句柄

- `c.Endpoint(port)`——某暴露端口可连接的 `"host:port"`。
- `c.MappedPort(port)`——仅返回主机侧端口号。
- `c.Host()`——主机(回环地址)。

### 预设

- `Redis(t)`——起 `redis:7`,返回其 endpoint。
- `MySQL(t, rootPassword, database)`——起 `mysql:8`,返回其 endpoint。

真实 redis 的验收测试见 [`container_test.go`](container_test.go)。

## 许可证

Apache License 2.0
