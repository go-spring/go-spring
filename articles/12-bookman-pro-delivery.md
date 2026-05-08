# 写完功能以后，我才发现交付一个服务还差最后一公里

BookMan Pro 写到这里，功能已经不少了。

它有配置、IoC、生命周期、HTTP CRUD、分层结构、日志、测试、starter、MySQL、Redis、pprof，还有自定义 Server。

如果只是自己学习，到这里已经很满足了。但如果要把这个项目交给别人，问题就变了：别人能不能启动？能不能看懂配置？能不能跑测试？出错时能不能知道从哪里查？

这一篇我不再加新功能，而是把 BookMan Pro 当成一个要交付的小服务，做最后整理。

## README 不是摆设

我以前写 README，经常只写几句简介。

现在我觉得，一个服务的 README 至少要回答：

```text
这个项目做什么？
目录结构怎么组织？
本地怎么启动？
关键配置有哪些？
API 怎么调用？
测试怎么跑？
哪些外部基础设施是可选的？
常见错误怎么排查？
```

如果别人不看这 12 篇文章，只看 README，也应该能把服务跑起来。

## 默认启动必须简单

默认配置应该保持轻量：

```properties
spring.http.server.addr=:9090
bookman.dao.type=memory
bookman.cache.enabled=false
pprof.enable=false
```

启动：

```bash
go run .
```

验证：

```bash
curl http://127.0.0.1:9090/books
```

我不希望第一次启动就要求 MySQL、Redis 或价格服务。那些都是可选增强，不应该成为入门门槛。

## 配置要写给未来的自己看

README 里应该列出常用配置：

```properties
spring.http.server.addr=:9090
bookman.dao.type=memory
bookman.cache.enabled=false
bookman.cache.ttl=30s
bookman.price.base-url=
pprof.enable=false
pprof.addr=:6060
```

也要写清楚覆盖方式：

```bash
go run . -Dspring.http.server.addr=:9091
GS_BOOKMAN_CACHE_ENABLED=true go run .
go run . -Dspring.profiles.active=integration
```

我现在很怕那种“配置靠口口相传”的项目。今天我记得，三个月后我自己也会忘。

## API 示例要能复制

README 里的 curl 命令应该直接能跑：

```bash
curl http://127.0.0.1:9090/books
```

```bash
curl http://127.0.0.1:9090/books/978-0134190440
```

```bash
curl -X POST http://127.0.0.1:9090/books \
  -H 'Content-Type: application/json' \
  -d '{"isbn":"978-0134494166","title":"Clean Architecture","author":"Robert C. Martin","publisher":"Prentice Hall"}'
```

```bash
curl -X DELETE http://127.0.0.1:9090/books/978-0134494166
```

每条命令最好写明预期状态码或关键输出。

文档里的命令如果没人跑，很快就会失效。

## 测试命令也要简单

默认测试命令：

```bash
go test ./...
```

它不应该依赖 MySQL、Redis 或外部价格服务。

集成测试可以单独说明：

```bash
go test ./... -tags=integration
```

默认测试越轻，越容易成为日常习惯。

## 主动演练失败场景

我以前只验证“成功路径”。现在觉得失败路径也要跑一下。

端口占用：把 `spring.http.server.addr` 配到已占用端口，应用应该启动失败。

缺失依赖：临时去掉 DAO 注册，应用应该在启动阶段报告 `BookRepository` 缺失。

错误配置：把 `bookman.cache.ttl=abc`，配置绑定应该失败。

外部资源不可用：启用 integration Profile，但 MySQL DSN 不可达，应用应该明确报错。

这些演练让我确认一件事：错误要在合适的阶段暴露，而不是藏到线上请求里。

## 最后检查一次边界

我给自己列了一个清单：

```text
Controller 不包含数据存储细节。
Service 不依赖 http.Request 或 http.ResponseWriter。
DAO 和 SDK 可以在测试中替换。
Runner 不执行长期阻塞任务。
后台 Job 监听根 Context。
资源型 starter 有 Destroy 逻辑。
默认配置能本地直接启动。
go test ./... 通过。
```

这比目录看起来漂不漂亮更重要。

## 我最后怎么理解 Go-Spring

写完整个 BookMan Pro 后，我对 Go-Spring 的理解也变了。

它不是为了替我写业务，也不是为了替代标准库 HTTP。

它更像是在帮我处理服务端项目里那些绕不开的工程问题：配置、依赖、生命周期、日志、测试替换、组件集成。

如果只是一个很小的一次性脚本，我可能不会用它。但如果我要认真写一个能配置、能测试、能扩展、能交付的服务，它就开始有价值了。

## 最后给自己的作业

把 README 里的启动、配置、API、测试命令全部复制执行一遍。

任何一个命令跑不通，都不要怪读者“应该知道怎么改”，而是回到代码或文档里修。

做到这里，BookMan Pro 才不只是我自己写过的一组练习，而是一个别人也能接手的小型 Go-Spring 服务。
