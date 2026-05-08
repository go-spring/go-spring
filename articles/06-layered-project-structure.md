# 文件越来越多以后，我才开始认真整理项目结构

BookMan Pro 写到 CRUD 以后，文件开始变多了。

一开始我还觉得没关系，反正 Go 项目就是几个包。但当 Controller、Service、DAO、SDK、配置、路由、中间件都出现以后，我开始找不到东西。

更麻烦的是，我有时候会不小心让 Service import HTTP 相关包，或者让 Controller 直接碰 DAO。代码还能跑，但我知道这会让后面越来越难改。

所以这一篇不是为了“看起来像大项目”，而是为了把依赖方向整理清楚。

## 我想要的目录结构

我参考了仓库里的 `docs/3.examples/bookman`，把结构整理成这样：

```text
conf/
public/
internal/
  app/
    common/httpsvr/
    controller/
  biz/
    job/
    service/book_service/
  dao/book_dao/
  sdk/book_sdk/
main.go
init.go
```

第一次看这个结构，我有点担心是不是太复杂。后来我发现关键不是目录数量，而是依赖方向：

```text
app -> biz -> dao/sdk
```

`app` 负责应用入口，比如 HTTP、路由、中间件和 Controller。

`biz` 负责业务，比如图书保存、查询、校验、后台任务。

`dao` 和 `sdk` 负责外部细节，一个访问数据，一个访问外部服务。

只要方向不乱，目录就不会只是摆设。

## Controller 放到 app 层

`internal/app/controller` 里只放 HTTP 相关代码：

- 解析路径参数。
- 解析 JSON。
- 调用 Service。
- 写状态码和响应。

例如：

```go
type BookController struct {
	Service *book_service.BookService `autowire:""`
}
```

Controller 依赖 Service 是正常的。它是应用入口，需要调用业务能力。

但 Controller 不应该知道图书怎么存，也不应该直接调用 SDK。

## Service 放到 biz 层

`internal/biz/service/book_service` 里放业务规则。

这里的方法应该尽量像业务语言：

```go
func (s *BookService) Save(ctx context.Context, book Book) error
func (s *BookService) Get(ctx context.Context, isbn string) (BookDetail, error)
```

我给自己定了一个检查标准：Service 方法里不要出现 `http.Request` 或 `http.ResponseWriter`。

如果出现了，说明 HTTP 细节漏进业务层了。

## DAO 放到 dao 层

`internal/dao/book_dao` 里先放内存实现：

```go
type BookDao struct {
	mu    sync.RWMutex
	books map[string]Book
}
```

Service 通过接口依赖它：

```go
type BookRepository interface {
	List(ctx context.Context) ([]Book, error)
	Find(ctx context.Context, isbn string) (Book, error)
	Save(ctx context.Context, book Book) error
	Delete(ctx context.Context, isbn string) error
}
```

我现在抽这个接口，不是为了写得“优雅”，而是因为第 10 篇确实要换 MySQL。提前把边界留出来，后面会轻松很多。

## SDK 单独放

`internal/sdk/book_sdk` 可以模拟一个价格服务：

```go
type BookPriceSDK struct {
	BaseURL string `value:"${bookman.price.base-url:=}"`
}
```

哪怕现在只是返回一个固定价格，我也愿意先把它放到 SDK 层。

因为外部 HTTP、RPC、第三方 API 这类东西，一旦散在 Service 里，测试和替换都会很麻烦。

## main.go 不要变成装配中心

入口文件尽量简单：

```go
import (
	"github.com/go-spring/spring-core/gs"

	_ "bookman-pro/internal/app"
	_ "bookman-pro/internal/biz"
)

func main() {
	gs.Run()
}
```

各模块在自己的 `init` 里注册 Bean：

```go
func init() {
	gs.Provide(book_service.NewBookService)
	gs.Provide(book_dao.NewBookDao).Export(gs.As[book_service.BookRepository]())
}
```

我以前会把所有注册都写在 `main` 附近，觉得这样一眼能看到。后来发现模块自己声明自己提供什么，入口只负责启动，反而更清楚。

## 怎么确认改对了

分层改造最怕改完以后行为变了。

所以我还是用原来的命令验收：

```bash
go run .
curl http://127.0.0.1:9090/books
go test ./...
```

如果接口行为没变，但代码位置和依赖方向更清楚，这次整理就是值得的。

## 我这次踩到的坑

只移动文件，不整理依赖。如果 `biz` 还 import `internal/app/controller`，那不叫分层。

为了分层给所有东西都抽接口。接口应该对应替换需求，不是格式要求。

包名起得太长。目录已经有上下文，包名保持清楚就行。

## 给自己留个小练习

新增一个 `internal/sdk/isbn_sdk`，提供：

```go
ValidateISBN(isbn string) bool
```

在 Service 保存图书前调用它，但不要让 Controller 直接依赖 SDK。

做到这里，我对项目结构的理解变了：分层不是摆目录，而是让代码变化时影响范围更小。
