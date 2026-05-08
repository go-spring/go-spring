# 交付前把默认路径、文档和自测收拢

`course/12-final` 是 BookMan Pro 的收尾版本。

前面已经讲过配置、Bean 注入、HTTP 路由、生命周期、分层、日志、测试、starter、集成开关和自定义 Server。最后一篇不再加新概念，重点放在交付质量上：别人拿到这个示例后，能不能直接运行，能不能看懂目录，能不能知道 API 怎么调用，出错时能不能找到排查入口。

## README 要回答运行问题

最终目录里同时有 `README.md` 和 `README_CN.md`。中文 README 先给出项目用途，再列目录结构：

```text
conf/                     配置文件目录
logs/                     日志文件目录
public/                   静态文件目录
internal/
  app/                    应用层模块
    common/httpsvr/       HTTP Server 与中间件
    controller/           HTTP Controller
  biz/                    业务层模块
    job/                  后台任务
    service/book_service/ 图书业务服务
  dao/book_dao/           内存数据访问层
  idl/http/proto/         HTTP 接口定义与路由注册
  sdk/book_sdk/           外部服务 SDK 封装示例
main.go                   启动入口与自测 Runner
init.go                   Banner 与工作目录初始化
```

这种 README 不需要写成长篇教程，但要覆盖最基本的问题：

- 项目做什么。
- 目录怎么组织。
- 本地怎么启动。
- 有哪些 HTTP 接口。
- 示例运行后会自动做什么。
- 配置和日志在哪里。

读者不看前面 11 篇文章，只看 README，也应该能把项目跑起来。

## 默认运行不依赖外部基础设施

最终配置仍然保持轻量：

```properties
spring.http.server.addr=0.0.0.0:8080

dync.refresh.time=0

logging.logger.root.type=FileLogger
logging.logger.root.level=INFO
logging.logger.root.dir=./logs
logging.logger.root.file=app.log
logging.logger.root.layout.type=JSONLayout
logging.logger.root.layout.fileLineMaxLength=20
```

没有 MySQL DSN，没有 Redis 地址，也没有必须存在的外部价格服务。

启动命令就是：

```bash
go run .
```

默认路径越简单，越适合作为示例项目。外部依赖可以出现在集成 Profile 或生产配置里，但不应该挡住第一次运行。

## API 示例要和代码对齐

README 里列出的接口是：

```text
GET    /books          查询图书列表
GET    /books/{isbn}   查询单本图书
POST   /books          新增或更新图书
DELETE /books/{isbn}   删除图书
GET    /               静态首页
```

`POST /books` 请求体示例：

```json
{
  "title": "Clean Architecture",
  "author": "Robert C. Martin",
  "isbn": "978-0134494166",
  "publisher": "Prentice Hall"
}
```

这些示例应该能直接复制执行。文档里的接口如果和代码不一致，读者会先怀疑自己环境有问题，然后才怀疑文档过期。

## 自测 Runner 覆盖主流程

`main.go` 注册了一个 `TestRunner`：

```go
func init() {
	gs.Provide(&TestRunner{}).Export(gs.As[gs.Runner]())
}
```

应用启动后，它会在 goroutine 里请求本机 HTTP 接口：

```go
runStep("list initial books", http.MethodGet, "/books", "")
runStep("get one book", http.MethodGet, "/books/978-0134190440", "")
runStep("save a new book", http.MethodPost, "/books", `{
	"title": "Clean Architecture",
	"author": "Robert C. Martin",
	"isbn": "978-0134494166",
	"publisher": "Prentice Hall"
}`)
runStep("list after save", http.MethodGet, "/books", "")
```

这不是替代单元测试。它更像一个示例级 smoke test：启动后自动跑一遍主要 HTTP 链路，让读者马上看到项目能工作。

自测完成后，它会发送 `SIGTERM` 给当前进程：

```go
syscall.Kill(os.Getpid(), syscall.SIGTERM)
```

这样可以顺便验证后台任务和 Server 的优雅退出。

## 动态配置刷新也要演示清楚

最终版本里还有一个动态配置字段：

```go
RefreshTime gs.Dync[int64] `value:"${dync.refresh.time:=0}"`
```

`TestRunner` 通过环境变量更新配置：

```go
refreshTime := strconv.FormatInt(time.Now().UnixMilli(), 10)
if err := os.Setenv("GS_DYNC_REFRESH_TIME", refreshTime); err != nil {
	panic(err)
}
if err := r.AppConfig.RefreshProperties(); err != nil {
	panic(err)
}
```

然后再次请求列表：

```go
runStep("list after config refresh", http.MethodGet, "/books", "")
```

这里演示的是 Go-Spring 动态配置刷新能力。示例代码把它放在自测 Runner 里，读者运行一次就能看到刷新前后的返回差异。

## init.go 处理启动体验

`init.go` 做了两件和业务无关但影响交付体验的事。

第一，设置 Banner：

```go
gs.Banner(banner)
```

第二，把工作目录切到源码所在目录：

```go
_, filename, _, ok := runtime.Caller(0)
if ok {
	execDir = filepath.Dir(filename)
}
err := os.Chdir(execDir)
```

这样 `conf/`、`public/`、`logs/` 这类相对路径不会因为从不同目录启动而失效。

这个做法适合教学示例。生产服务通常会由部署系统明确工作目录和配置路径，不一定需要在代码里 `Chdir`。

## 交付前的检查项

这个版本交付前，至少要手动过一遍：

```bash
go test ./...
go run .
```

然后确认：

- README 里的目录和接口与代码一致。
- `go run .` 不需要外部基础设施。
- 自动 HTTP 自测能跑完。
- 动态配置刷新后返回数据有变化。
- 进程收到 `SIGTERM` 后能退出。
- 日志文件能按配置写到 `logs/app.log`。

这套检查不复杂，但能挡住很多示例项目最常见的问题：文档过期、默认配置跑不起来、相对路径失效、后台任务无法退出。

BookMan Pro 到这里已经不只是几段分散的功能代码。它有一条可运行的默认路径，也有足够清楚的目录、配置、日志和自测入口，后续接真实数据库、缓存或外部服务时可以沿着这些边界继续扩展。
