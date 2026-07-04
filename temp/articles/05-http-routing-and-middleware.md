# CRUD 的重点是边界，不只是路由数量

`misc/course/05-http-routing` 把图书服务扩展成一个最小 CRUD API：

```text
GET    /books
GET    /books/{isbn}
POST   /books
DELETE /books/{isbn}
GET    /
```

从代码量看，这一步像是“多加几个 Handler”。实际更需要处理的是边界：哪些逻辑属于 HTTP，哪些逻辑属于业务规则，哪些逻辑属于数据访问。

如果保存和删除都直接写在 Handler 里，最开始会很快。等到要加重复 ISBN 策略、换存储、补测试时，混在一起的代码会拖慢每一次修改。

## 先补 Repository 能力

第 03 篇里的 Repository 只能查：

```go
type BookRepository interface {
	List() []Book
	Find(isbn string) (Book, bool)
}
```

CRUD 需要保存和删除：

```go
type BookRepository interface {
	List() []Book
	Find(isbn string) (Book, bool)
	Save(book Book)
	Delete(isbn string) bool
}
```

内存 DAO 使用 `sync.RWMutex` 保护 Map：

```go
type MemoryBookDao struct {
	mu    sync.RWMutex
	books map[string]Book
}
```

这里的目标很具体：只要 HTTP 请求可能并发进来，共享 Map 就不应该裸写。

## 业务规则放在 Service

保存图书时至少要校验 ISBN 和标题：

```go
func (s *BookService) SaveBook(book Book) error {
	if book.ISBN == "" {
		return errors.New("isbn is required")
	}
	if book.Title == "" {
		return errors.New("title is required")
	}
	if !s.Overwrite {
		if _, ok := s.repo.Find(book.ISBN); ok {
			return errors.New("book already exists")
		}
	}
	s.repo.Save(book)
	return nil
}
```

这里还加了重复 ISBN 的策略：

```go
type BookService struct {
	repo      BookRepository
	Overwrite bool `value:"${bookman.book.overwrite:=true}"`
}
```

`bookman.book.overwrite=false` 时，重复保存返回错误；Controller 再把这个错误翻译成 `409 Conflict`。

这类规则放 Service 更合适。HTTP 入口以后可能变成命令行、消息队列或 gRPC，ISBN 不能为空、重复保存怎么处理这些规则仍然应该复用。

## Controller 只处理协议细节

保存接口的 Controller 代码主要做三件事：解 JSON，调用 Service，写 HTTP 状态码。

```go
func (c *BookController) Save(w http.ResponseWriter, r *http.Request) {
	var book Book
	if err := json.NewDecoder(r.Body).Decode(&book); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := c.Service.SaveBook(book); err != nil {
		if err.Error() == "book already exists" {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

Controller 可以知道 `400`、`404`、`409`、`204`。它不应该知道底层是 Map、MySQL 还是别的存储。

同样，Service 不应该依赖 `http.Request` 或 `http.ResponseWriter`。这一点是后续测试和替换的基础。

## 标准库路由已经够用

Go 1.22 之后，`http.ServeMux` 支持方法和路径变量：

```go
mux.HandleFunc("GET /books", c.List)
mux.HandleFunc("GET /books/{isbn}", c.Get)
mux.HandleFunc("POST /books", c.Save)
mux.HandleFunc("DELETE /books/{isbn}", c.Delete)
```

读取路径变量：

```go
isbn := r.PathValue("isbn")
```

对这个阶段的 BookMan Pro 来说，标准库路由足够清楚。现在更值得投入的地方是分层和错误处理，而不是过早换路由框架。

## 访问日志中间件

CRUD 一多，需要看到请求方法、路径、状态码和耗时。示例先用一个简单中间件：

```go
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
```

包装 Handler：

```go
func accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start))
	})
}
```

`statusWriter` 的作用是捕获 `WriteHeader`。否则错误响应已经写了 `404`，日志里还可能以为是 `200`。

第 07 篇会把这里的临时 `log.Printf` 换成 Go-Spring 日志标签。

## 验证 CRUD

启动：

```bash
go run .
```

查询列表：

```bash
curl http://127.0.0.1:9090/books
```

查询单本：

```bash
curl http://127.0.0.1:9090/books/978-0134190440
```

保存一本书：

```bash
curl -X POST http://127.0.0.1:9090/books \
  -H 'Content-Type: application/json' \
  -d '{"isbn":"978-0134494166","title":"Clean Architecture","author":"Robert C. Martin","publisher":"Prentice Hall"}'
```

删除：

```bash
curl -X DELETE http://127.0.0.1:9090/books/978-0134494166
```

重复保存策略也可以直接试：

```bash
go run . -Dbookman.book.overwrite=false
```

这篇完成后，BookMan Pro 已经有了可用 API。更重要的是，HTTP、业务和数据访问没有重新搅在一起。
