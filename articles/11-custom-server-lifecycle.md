# 应用里不只有 HTTP 时，我才真正理解自定义 Server

写到现在，BookMan Pro 的主要入口还是 HTTP。

但我开始想另一个问题：如果应用里还有 gRPC 服务、TCP 服务、消息消费者，或者一个内部管理端口，它们应该怎么启动？

以前我可能会直接在 `main` 里起 goroutine。可前面已经吃过后台任务的亏了：启动成功怎么算？出错怎么办？退出时谁来停？

这一篇我开始看 Go-Spring 的自定义 Server。

## 我先区分 Job 和 Server

刚开始我把后台任务和 Server 混在一起。

后来我这样理解：

Job 是应用内部的后台逻辑。它通常只要监听 Context 退出。

Server 是长期对外提供服务的东西。它需要告诉应用“我 ready 了”，也需要在应用关闭时执行 Stop。

比如 HTTP、gRPC、TCP 网关，都更像 Server。

## 当前 Server 接口

当前 `spring-core` 里的接口是：

```go
type Server interface {
	Run(ctx context.Context, sig gs.ReadySignal) error
	Stop() error
}
```

`Run` 启动并运行服务。

`ReadySignal` 用来参与应用整体 ready 判断。

`Stop` 在应用关闭时释放资源。

我以前自己写服务时，很少认真处理 ready 这件事。现在发现它很关键：端口监听成功，不等于整个应用已经准备好对外服务。

## 写一个最小 TCP Echo Server

先定义结构：

```go
type EchoServer struct {
	addr string
	ln   net.Listener
}
```

配置：

```go
type EchoServerConfig struct {
	Addr string `value:"${bookman.echo-server.addr:=:10090}"`
}
```

构造函数：

```go
func NewEchoServer(c EchoServerConfig) *EchoServer {
	return &EchoServer{addr: c.Addr}
}
```

Server 也是 Bean，只是它实现了生命周期接口。

## Run 里先监听，再触发 ready

```go
func (s *EchoServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.ln = ln

	<-sig.TriggerAndWait()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return err
			}
		}
		go handleConn(conn)
	}
}
```

这里我重点看两点。

第一，监听失败要返回 error。端口被占用时，应用应该启动失败。

第二，`sig.TriggerAndWait()` 不是装饰品。它让这个 Server 参与应用整体 ready 流程。

## Stop 要真的能停

```go
func (s *EchoServer) Stop() error {
	if s.ln != nil {
		return s.ln.Close()
	}
	return nil
}
```

关闭 listener 后，阻塞在 `Accept` 的循环才会退出。

我以前写 Stop 很容易只改一个变量，结果 goroutine 还卡在阻塞调用里。这种 Stop 看起来有，实际上没停。

## 注册为 Server

```go
gs.Provide(NewEchoServer).Export(gs.As[gs.Server]())
```

这样应用启动后，Go-Spring 会同时管理内置 HTTP Server 和这个 EchoServer。

比起自己在 `main` 里起 goroutine，这种方式把启动失败、ready、运行错误和关闭都放进统一生命周期里。

## 验证一下

启动：

```bash
go run . -Dbookman.echo-server.addr=:10090
```

连接：

```bash
nc 127.0.0.1 10090
```

再试几个故障：

端口占用时，应用应该启动失败。

运行期错误时，应用应该能感知。

按 `Ctrl+C` 时，HTTP Server 和 EchoServer 都应该进入关闭流程。

## 我这次踩到的坑

启动后不触发 Ready。应用无法准确知道整体是否启动完成。

Stop 不关闭 listener。阻塞在 `Accept` 的 goroutine 不会退出。

运行期错误被吞掉。Server 静默停止，会让应用进入不完整状态。

把普通 Job 做成 Server。如果不需要 ready 和 stop 语义，监听 Context 的 Job 更简单。

## 给自己留个小练习

让 EchoServer 支持：

```properties
bookman.echo-server.enabled=false
```

关闭后只启动 HTTP Server。

写完这一篇，我对 Server 的理解终于不只停留在 HTTP 上了。只要是长期对外服务，都应该认真考虑它的 ready、error 和 stop。
