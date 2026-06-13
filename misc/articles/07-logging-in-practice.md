# 日志先按语义打标签

`misc/course/07-logging` 在分层项目的基础上整理日志。

前面几篇已经出现了 `log.Printf` 和 `fmt.Println`。这种临时打印适合调试，但不适合作为服务日志长期留下：访问日志、业务错误、后台任务和 SDK 调用混在一起，排查时很难过滤。

这一篇先做两件事：

- 用 Go-Spring 日志标签区分日志语义。
- 把根日志配置放到 `conf/app.properties`。

## 根日志配置

示例配置是：

```properties
logging.logger.root.type=FileLogger
logging.logger.root.level=INFO
logging.logger.root.dir=./logs
logging.logger.root.file=app.log
logging.logger.root.layout.type=JSONLayout
logging.logger.root.layout.fileLineMaxLength=20
```

这表示根日志写到 `./logs/app.log`，级别为 `INFO`，布局用 JSON。

开发阶段也可以改成控制台输出。关键是不要让日志配置写死在业务代码里，否则不同环境下切换输出位置和格式会很别扭。

## 访问日志是应用层语义

HTTP 访问日志放在 `internal/app/common/httpsvr`：

```go
var TagHttpAccess = log.RegisterAppTag("http", "access")
```

中间件里使用这个标签：

```go
func Access() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Infof(r.Context(), TagHttpAccess, "access %s %s", r.Method, r.URL.Path)
			next.ServeHTTP(w, r)
		})
	}
}
```

这条日志关心的是协议入口：方法、路径、请求上下文。

如果要继续增强，可以把第 05 篇的 `statusWriter` 合进来，补状态码和耗时。那仍然应该留在 HTTP 中间件里，而不是写到 Service。

## 业务日志放在 Service

业务层注册自己的标签：

```go
var TagBookService = log.RegisterBizTag("book", "service")
```

查询图书失败时记录业务错误：

```go
func (s *BookService) GetBook(ctx context.Context, isbn string) (proto.Book, error) {
	book, err := s.BookDao.GetBook(isbn)
	if err != nil {
		log.Errorf(ctx, TagBookService, "GetBook return err: %s", err.Error())
		return proto.Book{}, err
	}
	// ...
}
```

这里不记录 HTTP 状态码，因为 Service 不知道自己是不是被 HTTP 调用。它只记录业务事实：哪个业务动作失败了，错误是什么。

这种分层也适用于成功日志。保存图书可以记录 ISBN 和操作名；删除不存在的图书可以用 WARN。协议结果仍然交给 Controller 或中间件。

## SDK 日志应该单独留出口

当前 `book_sdk` 只是固定返回价格：

```go
func (s *BookSDK) GetPrice(isbn string) string {
	return "￥10"
}
```

所以这一版没有必要给它加复杂日志。

如果下一步接真实价格服务，SDK 包应该注册单独标签，比如：

```go
var TagBookPrice = log.RegisterRPCTag("book", "price")
```

外部调用的超时、状态码、目标地址、错误原因都应该在 SDK 层记录。这样价格服务不稳定时，可以直接过滤 RPC/SDK 类日志，不用在业务日志里猜。

## 运行后看什么

启动应用：

```bash
go run .
```

请求列表：

```bash
curl http://127.0.0.1:8080/books
```

默认端口来自第 07 篇的配置：

```properties
spring.http.server.addr=0.0.0.0:8080
```

然后查看日志文件：

```bash
tail -f logs/app.log
```

重点看三件事：

- HTTP 请求是否带 `http/access` 语义标签。
- 业务错误是否走 `book/service` 标签。
- 日志输出位置、级别和格式是否由配置控制。

## 这篇的边界

日志不是“到处打印一下”。每层应该记录自己能解释的信息。

HTTP 层记录协议入口和响应结果。

Service 层记录业务动作和业务错误。

SDK 层记录外部依赖调用。

后台任务记录任务状态和退出原因。

这样设计后，日志量增加时仍然能过滤、路由和排障，而不是只剩下一堆看起来很热闹的文本。
