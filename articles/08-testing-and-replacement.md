# 每次都手工 curl 太累了，我开始给 BookMan 写测试

前面每做完一个功能，我都靠 `go run .` 加 `curl` 验证。

刚开始这很直观。但接口一多，我就发现自己不可能每次改代码都手工请求一遍。更麻烦的是，如果以后接 MySQL、Redis、价格服务，难道每次测试都要把这些服务全启动起来吗？

这一篇我开始给 BookMan Pro 写测试。目标不是追求覆盖率数字，而是让我敢继续改。

## 不是所有测试都要启动 Go-Spring

我一开始以为，既然项目用了 Go-Spring，测试也应该都用 Go-Spring 启动。

后来发现不对。

如果一个对象能手动构造，依赖也能手动传进去，那普通 Go 单元测试更合适。它快，也更容易定位问题。

比如内存 DAO：

```go
func TestBookDao_SaveAndFind(t *testing.T) {
	dao := book_dao.NewBookDao()
	book := book_dao.Book{ISBN: "1", Title: "Test"}

	dao.Save(book)
	got, ok := dao.Find("1")

	require.That(t, ok).True()
	assert.That(t, got.Title).Equal("Test")
}
```

这类测试完全不需要 HTTP Server。

## Service 测试靠 fake 依赖

Service 依赖 DAO 和价格 SDK。如果我想测试业务规则，最好传 fake：

```go
type fakeRepo struct {
	books map[string]Book
}

func (r *fakeRepo) Find(isbn string) (Book, bool) {
	book, ok := r.books[isbn]
	return book, ok
}
```

然后构造 Service：

```go
service := book_service.NewBookService(&fakeRepo{books: seed}, &fakePriceClient{price: 42})
```

这样我可以测：

- ISBN 为空是否报错。
- 保存成功后能不能查到。
- 价格 SDK 的结果有没有合进图书详情。

如果这里很难写，通常说明我前面分层没做好，比如 Service 依赖了具体 SDK，而不是接口。

## 什么时候用 gs.RunTest

Go-Spring 参与的行为，才需要容器测试。

比如我要确认：

- `autowire` 能不能注入。
- `value` 能不能绑定配置。
- 条件注册有没有按配置生效。
- 模块之间的 Bean 能不能装配起来。

这时可以用 `gs.RunTest`：

```go
func TestBookServiceWithContainer(t *testing.T) {
	gs.Configure(func(g gs.App) {
		g.Property("bookman.price.enabled", "false")
	}).RunTest(t, func(s *struct {
		Service *book_service.BookService `autowire:""`
	}) {
		require.That(t, s.Service).NotNil()
		books := s.Service.List(context.Background())
		assert.That(t, len(books) > 0).True()
	})
}
```

我把它理解成：这是在测试“容器能不能把东西装起来”，不是每个业务方法都必须这么测。

## 容器测试里也能替换依赖

如果 Service 依赖价格接口，可以在测试里提供 fake：

```go
gs.Configure(func(g gs.App) {
	g.Provide(func() book_service.PriceClient {
		return &fakePriceClient{price: 42}
	})
}).RunTest(t, func(s *struct {
	Service *book_service.BookService `autowire:""`
}) {
	book, err := s.Service.Get(context.Background(), "978-0134190440")
	require.That(t, err).Nil()
	assert.That(t, book.Price).Equal(42)
})
```

这让我更确信：前面把 DAO、SDK 抽成接口，不是为了形式，而是为了测试替换。

## 默认测试命令要简单

我希望默认验证永远是：

```bash
go test ./...
```

这条命令不应该依赖 MySQL、Redis 或外部价格服务。

如果以后有集成测试，可以单独放到 tag 下：

```bash
go test ./... -tags=integration
```

默认测试越轻，我越愿意经常跑。

## 我这次踩到的坑

所有测试都用 `gs.RunTest`。容器测试更重，普通业务逻辑优先用单元测试。

容器测试里用 `t.Parallel()`。基于全局注册信息的测试不建议并行。

测试依赖本机环境变量。测试需要的配置应该在 `gs.Configure()` 里写清楚。

## 给自己留个小练习

给 `BookService.Save` 写两个测试：

- ISBN 为空时报错。
- 注入 fake 价格客户端后，查询图书能得到预期价格。

写完这一篇，我终于不用每次都靠手工 curl 给自己壮胆了。接下来几篇会进入更偏组件化和生产化的内容。
