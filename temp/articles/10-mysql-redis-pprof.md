# 默认路径保持轻量，集成能力通过开关打开

`misc/course/10-integrations` 处理三个生产项目里常见的开关：

- DAO 实现切换：默认内存实现，集成 Profile 切到 MySQL 路径。
- 图书详情缓存：默认关闭，需要时打开并设置 TTL。
- pprof 入口：默认关闭，需要诊断时打开。

当前示例没有真的连接 MySQL 和 Redis。代码里用 `FakeMysqlBookDao` 模拟 MySQL 路径，用进程内 Map 模拟缓存，用一个 pprof 占位路由模拟诊断入口。

这样写是有意的。第 10 篇先把条件注册、Profile 和默认启动体验讲清楚。真实 MySQL、Redis 客户端可以在这个边界上替换进去，不影响前面的学习路径。

## 默认配置不要求外部依赖

基础配置在 `conf/app.properties`：

```properties
spring.http.server.addr=:9090

bookman.dao.type=memory
bookman.cache.enabled=false
bookman.cache.ttl=30s
pprof.enable=false
```

默认运行：

```bash
go run .
curl http://127.0.0.1:9090/books
```

这个命令应该直接可用，不需要先装数据库、启动 Redis 或准备 pprof 端口。

学习示例尤其要守住这个默认路径。第一次运行就卡在外部依赖上，读者很难判断是代码问题、环境问题还是配置问题。

## DAO 通过条件互斥

内存 DAO 是默认实现：

```go
gs.Provide(NewMemoryBookDao).
	Condition(gs.OnProperty("bookman.dao.type").HavingValue("memory").MatchIfMissing()).
	Export(gs.As[BookRepository]())
```

`MatchIfMissing()` 表示没有配置 `bookman.dao.type` 时，也走内存实现。

模拟 MySQL 路径：

```go
gs.Provide(NewFakeMysqlBookDao).
	Condition(gs.OnProperty("bookman.dao.type").HavingValue("mysql")).
	Export(gs.As[BookRepository]())
```

这两个 Bean 都导出成 `BookRepository`，所以条件必须互斥。否则容器里会出现多个同类型候选，Service 不知道该注入哪个。

以后换成真实 MySQL DAO 时，Service 应尽量不动；主要工作是把 `NewFakeMysqlBookDao` 换成真实构造函数。条件和接口边界可以保留。

## 缓存先只覆盖单本查询

缓存结构：

```go
type BookCache struct {
	Enabled bool          `value:"${bookman.cache.enabled:=false}"`
	TTL     time.Duration `value:"${bookman.cache.ttl:=30s}"`
	mu      sync.Mutex
	items   map[string]cacheItem
}
```

Service 查询单本图书时先读缓存：

```go
func (s *BookService) Find(isbn string) (Book, bool) {
	if book, ok := s.cache.Get(isbn); ok {
		return book, true
	}
	book, ok := s.repo.Find(isbn)
	if ok {
		s.cache.Set(book)
	}
	return book, ok
}
```

这版只缓存 `GET /books/{isbn}` 这种详情查询。列表接口继续读 Repository。

真实 Redis 接入时，也建议先从一个小范围缓存开始。缓存一旦覆盖写入和删除路径，就要处理失效策略。当前示例没有 `Save` 和 `Delete`，所以还不展开缓存删除；前面的 CRUD 版本如果加缓存，保存和删除时必须清理对应 key。

## pprof 入口默认关闭

Controller 里有一个开关：

```go
type BookController struct {
	Service *BookService `autowire:""`
	PProf   bool         `value:"${pprof.enable:=false}"`
}
```

路由：

```go
mux.HandleFunc("GET /debug/pprof/", c.PProfIndex)
```

处理函数里先检查配置：

```go
func (c *BookController) PProfIndex(w http.ResponseWriter, r *http.Request) {
	if !c.PProf {
		http.NotFound(w, r)
		return
	}
	_, _ = w.Write([]byte("pprof is enabled in this course example"))
}
```

当前只是占位文本。真实项目通常会接入标准 pprof handler 或 starter，并且限制访问来源。pprof 是诊断入口，不应该裸露在公网。

## 用 Profile 打开集成路径

`conf/app-integration.properties`：

```properties
bookman.dao.type=mysql
bookman.cache.enabled=true
pprof.enable=true
```

启动集成 Profile：

```bash
go run . -Dspring.profiles.active=integration
```

验证 pprof 占位入口：

```bash
curl http://127.0.0.1:9090/debug/pprof/
```

这条路径会切到 `FakeMysqlBookDao`、打开进程内缓存、暴露诊断占位入口。

如果后续接真实 MySQL 和 Redis，可以把配置扩展成：

```properties
bookman.dao.type=mysql
bookman.cache.enabled=true
bookman.cache.ttl=30s

gorm.mysql.dsn=...
redis.addr=localhost:6379
```

但这些不应该进入默认配置。默认配置继续保证 `go run .` 能直接启动。

## 这篇真正固定的是策略

第 10 篇的核心不是“示例代码已经完整接入 MySQL 和 Redis”。当前代码还没有。

它固定的是生产化接入策略：

- 默认路径轻量。
- 外部依赖通过配置和 Profile 打开。
- 同一接口下的多个实现必须条件互斥。
- 缓存从小范围开始，并明确失效策略。
- pprof 这类诊断能力默认关闭，开启时考虑访问控制。

这些策略确定后，把 fake 实现换成真实组件会更稳。
