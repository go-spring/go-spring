# 写完一个 GET 接口以后，我才发现 CRUD 不只是加几个路由

前面我已经能查图书列表了。

刚开始我以为，接下来就是把路由补齐：`GET`、`POST`、`DELETE` 都写一下，CRUD 就完成了。

真正动手后才发现，问题不只是“多写几个 Handler”。如果我在 Handler 里直接操作 Map，保存和删除确实很快就能写出来，但业务规则、HTTP 状态码、数据存储会混在一起。

这一篇我想把 BookMan Pro 做成一个最小 CRUD API，同时尽量守住前面刚建立起来的分层边界。

## 先把 API 想清楚

我先定了几个接口：

```text
GET    /books
GET    /books/{isbn}
POST   /books
DELETE /books/{isbn}
GET    /
```

`/` 只是静态首页，用来浏览器里看一眼。真正的主线还是 `/books`。

这次我刻意提醒自己：不要先写 Handler。先看业务层需要哪些能力。

## Repository 先补能力

原来的 Repository 只能查询：

```go
type BookRepository interface {
	List() []Book
	Find(isbn string) (Book, bool)
}
```

现在要支持保存和删除：

```go
type BookRepository interface {
	List() []Book
	Find(isbn string) (Book, bool)
	Save(book Book)
	Delete(isbn string) bool
}
```

这样 Service 不需要知道底层是不是 Map。以后换 MySQL，这个接口也还能继续用。

## 校验规则放 Service

保存图书时，我至少要检查 ISBN 和标题：

```go
func (s *BookService) SaveBook(book Book) error {
	if book.ISBN == "" {
		return errors.New("isbn is required")
	}
	if book.Title == "" {
		return errors.New("title is required")
	}
	s.repo.Save(book)
	return nil
}
```

我以前很容易把这种校验直接写在 Handler 里。现在我更愿意放 Service，因为这属于业务规则，不属于 HTTP。

以后如果图书保存来自命令行、gRPC 或消息队列，这段规则仍然可以复用。

## Controller 只处理 HTTP

保存接口的 Controller：

```go
func (c *BookController) Save(w http.ResponseWriter, r *http.Request) {
	var book Book
	if err := json.NewDecoder(r.Body).Decode(&book); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := c.Service.SaveBook(book); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

这段代码里有 HTTP 状态码，这是 Controller 应该关心的事。

但它没有直接操作 DAO，也没有直接碰 Map。这样我就能分清：HTTP 层负责协议，Service 层负责规则。

## 标准库路由先够用了

Go 1.22 之后，`http.ServeMux` 能写方法和路径变量：

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

我之前以为做 REST API 一定要先选 Gin 或 chi。现在发现，对这个阶段来说，标准库已经够用了。更重要的是把业务层写稳。

以后如果换路由框架，Controller 可能要改，Service 不应该跟着改。

## 访问日志中间件

写 CRUD 时，我很快需要知道请求有没有进来、状态码是多少、耗时多少。

先包一个 `ResponseWriter`：

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

再写中间件：

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

注册时包到最外层：

```go
return &gs.HttpServeMux{Handler: accessLog(mux)}
```

这版先用 `log.Printf`，下一篇再改成结构化日志。

## 验证一下 CRUD

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

如果控制台能看到方法、路径、状态码和耗时，说明中间件也生效了。

## 我这次踩到的坑

中间件一直记录 `200`。原因是没有捕获 `WriteHeader`，错误响应也被当成成功。

Controller 偷偷访问 DAO。写的时候很顺手，但后面换数据库会很痛。

路径变量取不到。要确认路由是 `/books/{isbn}`，读取时用 `r.PathValue("isbn")`。

## 给自己留个小练习

给 `POST /books` 增加重复 ISBN 的处理策略：

```properties
bookman.book.overwrite=false
```

配置为 `false` 时，重复保存返回 `409 Conflict`；配置为 `true` 时允许覆盖。

写完这一篇，我发现 CRUD 不只是“把路由补齐”，更重要的是把 HTTP、业务和数据访问的边界守住。
