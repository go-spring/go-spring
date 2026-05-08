# 测试要分清业务行为和容器装配

`course/08-testing` 给 BookMan Pro 补测试。

前面每做完一节，都可以用 `go run .` 加几条 `curl` 验证。但接口和依赖一多，手工验证很快就不可靠。更麻烦的是，后面如果接 MySQL、Redis、外部价格服务，默认测试不能要求这些环境全部就绪。

这篇的目标很朴素：让 `go test ./...` 成为日常可跑的验证命令。

## DAO 用普通 Go 测试

`internal/dao/book_dao` 是内存实现，不需要启动 Go-Spring。

测试可以直接构造对象：

```go
func TestBookDao(t *testing.T) {
	dao := &BookDao{Store: map[string]Book{
		"978-0134190440": {
			Title:     "The Go Programming Language",
			Author:    "Alan A. A. Donovan, Brian W. Kernighan",
			ISBN:      "978-0134190440",
			Publisher: "Addison-Wesley",
		},
	}}

	books, err := dao.ListBooks()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(books) != 1 {
		t.Fatalf("expected 1 book, got %d", len(books))
	}
}
```

保存、查询、删除、缺失图书、缺 ISBN 这些分支都可以在普通单元测试里覆盖。

这类测试快，失败位置也清楚。只要对象能直接构造，就没有必要为了“项目用了 Go-Spring”而强行启动容器。

## Service 测试需要容器时再用 RunTest

当前 Service 里有 Go-Spring 注入字段：

```go
type BookService struct {
	BookDao     *book_dao.BookDao `autowire:""`
	BookSDK     *book_sdk.BookSDK `autowire:""`
	RefreshTime gs.Dync[int64]    `value:"${dync.refresh.time:=0}"`
}
```

测试这类装配行为时，用 `RunTest` 更合适：

```go
func TestBookService(t *testing.T) {
	gs.Web(false).RunTest(t, func(s *struct {
		Service *BookService `autowire:""`
	}) {
		// ...
	})
}
```

`gs.Web(false)` 关闭 Web Server。这个测试只关心容器能不能装配 `BookService`，不需要真的监听 HTTP 端口。

测试里还手动替换了 `BookDao`：

```go
s.Service.BookDao = &book_dao.BookDao{Store: map[string]book_dao.Book{
	"978-0134190440": {
		Title:     "The Go Programming Language",
		Author:    "Alan A. A. Donovan, Brian W. Kernighan",
		ISBN:      "978-0134190440",
		Publisher: "Addison-Wesley",
	},
}}
```

注释里写得很直接：`BookDao` 不应包含状态，因为它是全局共享对象。

这个点值得单独记住。容器创建的 Bean 如果带可变状态，测试之间很容易互相污染。当前示例通过测试内替换 DAO 状态来隔离数据；长期更稳的做法是让 DAO 构造函数每次创建独立存储，或者让 Service 依赖接口后注入 fake。

## 容器测试测什么

`RunTest` 适合覆盖这些行为：

- `autowire` 是否能注入对象。
- `value` 是否能绑定配置。
- 条件注册是否按配置生效。
- 模块之间的 Bean 是否能装起来。

普通业务规则优先用单元测试。比如 ISBN 不能为空、删除不存在的图书返回错误，这些逻辑不一定需要完整容器。

容器测试更接近应用启动过程，成本也更高。把所有测试都写成 `RunTest`，失败时反而不容易判断是业务错了，还是配置和装配错了。

## 不要让默认测试依赖外部环境

默认验证命令保持：

```bash
go test ./...
```

这条命令应该不依赖 MySQL、Redis 或真实价格服务。

以后如果确实要测外部基础设施，可以单独用 tag 或单独目录管理：

```bash
go test ./... -tags=integration
```

日常测试越轻，越容易在每次修改后运行。一个需要准备五个外部服务的默认测试套件，最后往往没人跑。

## 当前测试覆盖了什么

DAO 测试覆盖：

- 列表查询。
- 保存图书。
- 按 ISBN 查询。
- 查询缺失图书。
- 删除图书。
- 保存缺 ISBN 的错误。

Service 测试覆盖：

- 容器能装配 `BookService`。
- 列表查询能返回数据。
- 保存后数量变化。
- 按 ISBN 查询能拿到图书。
- 删除后列表变化。

这些测试不追求覆盖率好看，先覆盖最容易被后续改动破坏的路径。第 09、10 篇加入 starter 和集成开关后，这些测试能继续守住默认路径。
