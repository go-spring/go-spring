# 终于要接 MySQL 和 Redis 了，但我不想破坏默认启动体验

BookMan Pro 一直用内存 DAO。

这对学习很友好，因为我不需要先装 MySQL、建库、建表，也不需要启动 Redis。只要 `go run .`，服务就能跑。

但我也知道，真实服务不能一直靠内存。图书数据要持久化，热点详情可以加缓存，线上排查性能问题时还需要 pprof。

所以这一篇我想做的不是“强行上全家桶”，而是在默认路径不变的前提下，把 MySQL、Redis 和 pprof 作为可选能力接进来。

## 默认路径继续轻量

基础配置保持这样：

```properties
bookman.dao.type=memory
bookman.cache.enabled=false
pprof.enable=false
```

内存 DAO 作为默认实现：

```go
gs.Provide(NewMemoryBookDao).
	Condition(gs.OnProperty("bookman.dao.type").HavingValue("memory").MatchIfMissing()).
	Export(gs.As[book_service.BookRepository]())
```

我很在意 `MatchIfMissing()`。它表示没配置时也走内存实现。

这样读者第一次跑项目时，不会被外部依赖挡住。

## 用 MySQL 替换 DAO

需要持久化时，引入 GORM MySQL starter：

```go
import _ "github.com/go-spring/starter-gorm-mysql"
```

配置：

```properties
bookman.dao.type=mysql
gorm.mysql.dsn=user:password@tcp(127.0.0.1:3306)/bookman?charset=utf8mb4&parseTime=True&loc=Local
gorm.mysql.debug=true
```

MySQL DAO 注入 `*gorm.DB`：

```go
type GormBookDao struct {
	db *gorm.DB `autowire:""`
}
```

注册时和内存 DAO 互斥：

```go
gs.Provide(NewGormBookDao).
	Condition(gs.OnProperty("bookman.dao.type").HavingValue("mysql")).
	Export(gs.As[book_service.BookRepository]())
```

Service 还是只依赖 `BookRepository`。这一点让我觉得前面抽接口没有白做。

## Redis 先缓存图书详情

引入 go-redis starter：

```go
import _ "github.com/go-spring/starter-go-redis"
```

配置：

```properties
bookman.cache.enabled=true
bookman.cache.ttl=30s

redis.addr=localhost:6379
redis.password=
redis.db=0
```

缓存我不想一开始铺得太大。先缓存 `GET /books/{isbn}` 这种图书详情接口就够了。

写入和删除时要清理缓存 key。否则最容易出现的问题就是：明明数据库改了，接口却读到旧数据。

实现上可以有两种方式：

- 在 Service 里显式读写缓存，教学更直观。
- 用 Repository 装饰器包一层，长期维护更干净。

## pprof 也通过 starter 打开

引入：

```go
import _ "github.com/go-spring/starter-pprof"
```

配置：

```properties
pprof.enable=true
pprof.addr=:6060
```

启动后访问：

```bash
curl http://127.0.0.1:6060/debug/pprof/
```

或者采样：

```bash
go tool pprof http://127.0.0.1:6060/debug/pprof/profile
```

我给自己记一条：pprof 不能直接暴露公网。它是诊断入口，也可能泄露敏感信息。

## 用 Profile 管理集成环境

基础配置继续保持轻量。

新增 `conf/app-integration.properties`：

```properties
bookman.dao.type=mysql
bookman.cache.enabled=true
pprof.enable=true

gorm.mysql.dsn=user:password@tcp(127.0.0.1:3306)/bookman?charset=utf8mb4&parseTime=True&loc=Local
redis.addr=localhost:6379
pprof.addr=:6060
```

启动：

```bash
go run . -Dspring.profiles.active=integration
```

这样本地学习、集成验证、生产部署就能走不同配置。

## 我这次踩到的坑

默认启动就要求 MySQL 和 Redis。这样会让学习路径很重。

内存 DAO 和 MySQL DAO 同时注册。它们都导出 `BookRepository`，必须通过条件互斥。

缓存只读不删。保存和删除图书时要清理缓存。

pprof 直接暴露公网。诊断能力也要有访问控制。

## 给自己留个小练习

给图书详情缓存增加 TTL：

```properties
bookman.cache.ttl=30s
```

保存或删除图书时删除对应缓存 key。

写完这一篇，我对“生产化”的理解更现实了：不是把所有基础设施强塞进来，而是让它们可以按环境、按配置启用。
