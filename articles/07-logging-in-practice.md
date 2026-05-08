# fmt.Println 越写越多以后，我开始补日志系统

BookMan Pro 能保存、查询、删除图书以后，我又遇到一个很朴素的问题：出问题时我怎么看？

一开始我到处写 `fmt.Println`。请求进来了打印一下，保存图书打印一下，调用 SDK 打印一下。

这种方式在刚调试时很快，但很快就乱了。我分不清哪些是访问日志，哪些是业务日志；也不知道以后要按 ISBN、状态码、耗时去查时该怎么办。

这一篇我开始把临时打印换成结构化日志。

## 我不再只想知道日志来自哪个文件

以前我理解日志，主要看它来自哪个包、哪个文件。

但排查问题时，我更关心这条日志是什么性质：

- 这是一次 HTTP 访问吗？
- 这是一次保存图书的业务操作吗？
- 这是价格 SDK 的外部调用吗？

Go-Spring 日志系统里的标签，刚好用来表达这个语义。

## 先注册几个标签

我先注册三类：

```go
var (
	TagHTTPAccess = log.RegisterAppTag("http", "access")
	TagBookBiz    = log.RegisterBizTag("book", "operation")
	TagBookSDK    = log.RegisterRPCTag("book", "price")
)
```

访问日志、业务日志、外部依赖日志就分开了。

我没有一口气设计很多标签，因为我现在还不确定全部排障场景。先围绕当前能用到的地方拆开就好。

## 先让日志输出到控制台

`conf/app.properties` 里先写：

```properties
logger.root.type=ConsoleLogger
logger.root.level=INFO
```

我现在不急着搞文件、JSON 或异步日志。先让日志能稳定输出，能看懂，再继续增强。

## 访问日志不要再拼字符串

第 05 篇里，中间件还是这样：

```go
log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start))
```

现在改成结构化字段：

```go
log.Info(r.Context(), TagHTTPAccess,
	log.String("method", r.Method),
	log.String("path", r.URL.Path),
	log.Int("status", sw.status),
	log.String("duration", time.Since(start).String()),
	log.Msg("http access"),
)
```

我理解结构化日志的方式很简单：重要信息不要塞在一句话里，要变成字段。

以后我要查 `status=500`，或者查某个 `path`，就不用从一整行文本里再抠。

## 业务日志写在 Service

保存图书时，我在 Service 层记录：

```go
log.Info(ctx, TagBookBiz,
	log.String("isbn", book.ISBN),
	log.String("title", book.Title),
	log.Msg("save book"),
)
```

这里不写 HTTP 状态码。Service 不应该知道自己是被 HTTP 调用的。

如果删除一本不存在的书，可以用 `WARN`：

```go
log.Warn(ctx, TagBookBiz,
	log.String("isbn", isbn),
	log.Msg("delete missing book"),
)
```

这让我开始明白：日志也应该遵守分层。Controller 记录协议结果，Service 记录业务事实，SDK 记录外部调用。

## SDK 日志单独打

价格 SDK 调用成功：

```go
log.Info(ctx, TagBookSDK,
	log.String("isbn", isbn),
	log.Float("price", price),
	log.Msg("load book price"),
)
```

失败：

```go
log.Error(ctx, TagBookSDK,
	log.String("isbn", isbn),
	log.String("error", err.Error()),
	log.Msg("load book price failed"),
)
```

这样如果以后价格服务不稳定，我可以直接盯着 SDK 相关日志看，不用在业务日志里混着找。

## 试一下

启动应用：

```bash
go run .
```

请求列表：

```bash
curl http://127.0.0.1:9090/books
```

保存一本书：

```bash
curl -X POST http://127.0.0.1:9090/books \
  -H 'Content-Type: application/json' \
  -d '{"isbn":"978-0134494166","title":"Clean Architecture","author":"Robert C. Martin","publisher":"Prentice Hall"}'
```

我主要看两点：日志有没有清晰标签，关键字段是不是单独输出。

## 我这次踩到的坑

只写 `msg`，不写字段。这样人能看，但机器不好查。

所有日志共用一个标签。短期省事，长期会让日志路由和检索变粗。

业务层记录 HTTP 信息。这样会让 Service 和 HTTP 绑定在一起。

## 给自己留个小练习

给删除图书操作加业务日志：

- 删除成功记录 `isbn`。
- 删除不存在时用 `WARN`。
- 不要在 Controller 里直接写业务日志。

写到这里，我对日志的理解从“打印一下看看”变成了“为以后排障留下结构化线索”。
