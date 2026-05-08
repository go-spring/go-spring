# 手动 new 对象写烦了以后，我才理解 IoC 容器在解决什么

写到这里，我的 BookMan Pro 已经能启动，也能读配置。

但只要我想把它从 `/echo` 变成真正的图书服务，问题马上来了：我需要 Controller、Service、DAO。Controller 调 Service，Service 调 DAO。

如果是刚学 Go 的我，大概率会直接在一个地方手动创建：

```go
dao := NewBookDao()
service := NewBookService(dao)
controller := NewBookController(service)
```

对象少的时候这样很清楚。但我稍微往后想了一下：以后 DAO 要换 MySQL，SDK 要换 mock，Controller 要越来越多，这些手动组装代码会散到哪里？

这时我才开始理解 IoC 容器的意义：不是为了显得高级，而是为了让对象只声明依赖，不自己到处创建依赖。

## 我先拆出三层

我给 BookMan Pro 画了一条最小链路：

```text
BookController -> BookService -> BookRepository
```

Controller 只管 HTTP。

Service 只管业务规则。

Repository/DAO 只管数据。

这条线看起来简单，但对我这种刚开始写服务的人很重要。因为我以前很容易在 Handler 里直接操作 Map，写着写着就分不清哪里是业务，哪里是 HTTP。

## DAO 先用内存版

为了不被数据库打断，我先写内存 DAO：

```go
type Book struct {
	ISBN      string `json:"isbn"`
	Title     string `json:"title"`
	Author    string `json:"author"`
	Publisher string `json:"publisher"`
}

type BookRepository interface {
	List() []Book
	Find(isbn string) (Book, bool)
}

type MemoryBookDao struct {
	books map[string]Book
}

func NewMemoryBookDao() *MemoryBookDao {
	return &MemoryBookDao{books: map[string]Book{
		"978-0134190440": {
			ISBN: "978-0134190440", Title: "The Go Programming Language",
			Author: "Alan A. A. Donovan", Publisher: "Addison-Wesley",
		},
	}}
}
```

这里我第一次认真想“接口该放在哪里”。

不是所有东西都要抽接口。但 DAO 以后确实可能被替换成 MySQL，也可能在测试里换成 fake，所以这里抽一个 `BookRepository` 是有价值的。

注册 DAO：

```go
func init() {
	gs.Provide(NewMemoryBookDao).Export(gs.As[BookRepository]())
}
```

我一开始以为 Go-Spring 会自动发现 `MemoryBookDao` 实现了 `BookRepository`。后来发现不会，必须显式 `Export`。

这个设计反而让我觉得安心：接口导出是我主动声明的，不是框架偷偷猜出来的。

## Service 通过构造函数拿依赖

Service 写成这样：

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
```

注册：

```go
gs.Provide(NewBookService)
```

这里最关键的是 `NewBookService(repo BookRepository)`。它等于告诉容器：我要一个 `BookRepository`，但我不负责创建它。

容器启动时会找到刚才导出的 `MemoryBookDao`，再把它传给 Service。

## Controller 可以用字段注入

Controller 里我用了字段注入：

```go
type BookController struct {
	Service *BookService `autowire:""`
}

func (c *BookController) List(w http.ResponseWriter, r *http.Request) {
	_ = json.NewEncoder(w).Encode(c.Service.ListBooks())
}
```

注册：

```go
gs.Provide(&BookController{})
```

一开始我纠结：到底该用构造函数注入，还是字段注入？

现在我的理解是：Service 这种核心对象，构造函数更清楚；Controller 这种完全交给容器管理的对象，字段注入也可以接受。重要的是依赖关系别藏起来，测试时也能替换。

## 路由也交给容器组合

最后注册路由：

```go
gs.Provide(func(c *BookController) *gs.HttpServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/books", c.List)
	return &gs.HttpServeMux{Handler: mux}
})
```

这个函数也声明了依赖：它需要 `*BookController`。

所以容器大概会按这样的关系创建对象：

```text
MemoryBookDao -> BookRepository
BookRepository -> BookService
BookService -> BookController
BookController -> HttpServeMux
```

这比我自己在 `main` 里一层层 new，要更容易扩展。

## 跑一下看看

启动：

```bash
go run .
```

请求：

```bash
curl http://127.0.0.1:9090/books
```

应该能看到一个 JSON 数组。

我还故意注释掉 DAO 注册试了一下。应用启动时就报依赖缺失，而不是等请求来了才 panic。这一点让我对“启动期装配”有了感觉：能早失败，就不要晚失败。

## 我这次学到的坑

注册具体类型不等于注册接口。如果 Service 要的是 `BookRepository`，DAO 注册时必须 `Export(gs.As[BookRepository]())`。

Controller 不要自己 `new` Service。这样会绕过容器，后面配置注入和测试替换都会变麻烦。

不要为了“解耦”给所有类型都抽接口。接口应该对应真实替换需求。

## 给自己留个小练习

给 `BookService` 增加 `CountBooks()`，再加一个 `/books/count` 接口。

要求 Controller 只能调用 Service，不能直接访问 DAO。

做到这里，我终于开始理解 IoC 容器不是“魔法 new 对象”，而是让依赖关系在启动阶段变清楚。下一篇我会继续看启动自检和后台任务该放在哪里。
