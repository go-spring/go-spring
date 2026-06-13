# 把 Controller、Service 和 DAO 的依赖交给容器

`misc/course/03-ioc` 开始把 BookMan Pro 从 `/echo` 改成图书查询接口。

这一步还不做完整 CRUD，只先建立一条最小链路：

```text
BookController -> BookService -> BookRepository
```

如果把这些对象都写在 `main` 里手动创建，代码一开始会很直观：

```go
dao := NewMemoryBookDao()
service := NewBookService(dao)
controller := NewBookController(service)
```

对象数量少时，这样写没什么问题。问题出现在后续演进：DAO 要换实现，Controller 变多，Service 需要更多依赖，路由也要跟着组合。装配代码如果散在入口附近，业务对象会越来越难单独测试和替换。

IoC 容器在这里承担的角色很具体：让对象声明自己需要什么，由启动阶段统一完成装配。

## Repository 先定义替换边界

图书数据先用内存 Map：

```go
type BookRepository interface {
	List() []Book
	Find(isbn string) (Book, bool)
}

type MemoryBookDao struct {
	books map[string]Book
}
```

这个接口不追求抽象感。它对应一个明确的替换点：现在是内存 DAO，后面可能换成 MySQL，测试里也可能换成 fake。

DAO 注册时要显式导出接口：

```go
func init() {
	gs.Provide(NewMemoryBookDao).Export(gs.As[BookRepository]())
}
```

Go-Spring 不会因为 `MemoryBookDao` 刚好实现了 `BookRepository` 就自动把它当接口暴露。`Export(gs.As[BookRepository]())` 是应用作者主动声明的装配边界。

## Service 用构造函数拿依赖

Service 只依赖接口：

```go
type BookService struct {
	repo BookRepository
}

func NewBookService(repo BookRepository) *BookService {
	return &BookService{repo: repo}
}

func (s *BookService) ListBooks() []Book {
	return s.repo.List()
}

func (s *BookService) CountBooks() int {
	return len(s.repo.List())
}
```

注册 Service：

```go
gs.Provide(NewBookService)
```

`NewBookService(repo BookRepository)` 把依赖关系写在函数签名里。启动时如果没有任何 Bean 导出 `BookRepository`，应用会在装配阶段失败，而不是等第一个请求进来再 panic。

这种早失败对服务端项目很实用。配置缺失、依赖缺失、类型不匹配，都应该尽量暴露在启动阶段。

## Controller 用字段注入也可以

Controller 这一版用字段注入：

```go
type BookController struct {
	Service *BookService `autowire:""`
}
```

然后暴露两个接口：

```go
func (c *BookController) List(w http.ResponseWriter, r *http.Request) {
	_ = json.NewEncoder(w).Encode(c.Service.ListBooks())
}

func (c *BookController) Count(w http.ResponseWriter, r *http.Request) {
	_ = json.NewEncoder(w).Encode(map[string]int{"count": c.Service.CountBooks()})
}
```

Service 这种核心业务对象，用构造函数注入更容易看清必需依赖。Controller 完全由容器创建，字段注入也能接受。关键是不要让 Controller 自己 `new` Service，否则会绕过容器，后面配置注入和测试替换都会变麻烦。

## 路由组合也声明依赖

路由注册函数需要 `*BookController`：

```go
gs.Provide(func(c *BookController) *gs.HttpServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/books", c.List)
	mux.HandleFunc("/books/count", c.Count)
	return &gs.HttpServeMux{Handler: mux}
})
```

这段代码仍然使用标准库 `http.ServeMux`。Go-Spring 负责的是对象创建和依赖装配，不要求把路由写法换成另一套路由框架。

启动时的关系大致是：

```text
NewMemoryBookDao -> BookRepository
BookRepository -> NewBookService
BookService -> BookController
BookController -> HttpServeMux
```

依赖方向清楚以后，后面的 CRUD、日志、测试和 starter 才有地方落。

## 运行验证

进入目录后启动：

```bash
go run .
```

查询图书列表：

```bash
curl http://127.0.0.1:9090/books
```

查询数量：

```bash
curl http://127.0.0.1:9090/books/count
```

这篇最需要确认的是对象关系没有写死在 `main` 里。Controller 只调用 Service，Service 只依赖 Repository，DAO 的具体实现由启动阶段注入。
