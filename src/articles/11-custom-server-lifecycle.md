# 自定义 Server 要进入统一生命周期

`course/11-custom-server` 在 HTTP 接口旁边加了一个 TCP echo server。

HTTP 服务由 Go-Spring 内置 Server 管理。应用里如果还要启动 gRPC、TCP 网关、管理端口或其他长期监听组件，也应该进入同一套生命周期。直接在 `main` 里起 goroutine，启动失败、ready、运行错误和关闭都会变成散落的逻辑。

这一篇用一个最小 TCP Server 看清楚 `gs.Server` 的边界。

## Job 和 Server 的区别

第 04 篇里的后台统计任务是 Job。它不对外监听端口，只要跟随应用 Context 退出。

TCP echo server 不一样。它长期监听端口，对外接收连接，还需要在应用关闭时释放 listener。它更适合实现 `gs.Server`。

当前接口是：

```go
type Server interface {
	Run(ctx context.Context, sig gs.ReadySignal) error
	Stop() error
}
```

`Run` 负责启动和运行服务。

`ReadySignal` 让这个 Server 参与应用整体 ready 流程。

`Stop` 在应用关闭时释放资源。

## 配置和构造函数

Echo Server 的地址来自配置：

```go
type EchoServerConfig struct {
	Addr string `value:"${bookman.echo-server.addr:=:10090}"`
}
```

结构体保存监听地址和 listener：

```go
type EchoServer struct {
	addr string
	ln   net.Listener
}
```

构造函数保持简单：

```go
func NewEchoServer(c EchoServerConfig) *EchoServer {
	return &EchoServer{addr: c.Addr}
}
```

这和前面 Service、DAO、starter 的思路一致：配置绑定发生在启动阶段，对象只拿到已经绑定好的值。

## Run 先监听，再进入 ready 流程

核心代码：

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

监听失败直接返回 error。端口被占用、权限不足、地址不合法，都应该让应用启动失败。

`sig.TriggerAndWait()` 放在 `net.Listen` 成功之后。这个 Server 只有在端口已经监听成功后，才参与应用 ready。

`Accept` 返回错误时要区分两类情况：应用正在关闭时返回 nil；其他运行期错误返回 error，让框架感知到 Server 异常。

## Stop 要关闭阻塞点

Stop 里关闭 listener：

```go
func (s *EchoServer) Stop() error {
	if s.ln == nil {
		return nil
	}
	err := s.ln.Close()
	if errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}
```

只设置一个标志位不够。`Accept` 正阻塞在 listener 上，必须关闭 listener 才能让循环醒过来并退出。

连接处理本身也保持简单：

```go
func handleConn(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "quit" {
			return
		}
		_, _ = conn.Write([]byte("echo: " + line + "\n"))
	}
}
```

这个示例没有做连接级别的超时和并发限制。真实 Server 需要继续补这些保护。

## 条件注册

自定义 Server 默认启用，也可以通过配置关闭：

```go
gs.Provide(NewEchoServer).
	Condition(gs.OnProperty("bookman.echo-server.enabled").HavingValue("true").MatchIfMissing()).
	Export(gs.As[gs.Server]())
```

关闭：

```bash
go run . -Dbookman.echo-server.enabled=false
```

这时应用只启动 HTTP Server。

## 运行验证

启动：

```bash
go run .
```

HTTP 仍然可用：

```bash
curl http://127.0.0.1:9090/books
```

连接 TCP echo server：

```bash
nc 127.0.0.1 10090
```

输入一行文本，应该收到：

```text
echo: 你的输入
```

输入 `quit` 关闭当前连接。

还应该试两个故障路径：

- 端口被占用时，应用启动失败。
- 按 `Ctrl+C` 时，HTTP Server 和 EchoServer 都进入关闭流程。

## 适合做成 Server 的组件

判断标准很直接：只要组件长期对外提供服务，并且需要 ready、运行错误和 stop 语义，就应该考虑 `gs.Server`。

HTTP、gRPC、TCP 网关、独立管理端口都符合这个条件。

单纯的周期性任务、队列轮询或内部清理逻辑，如果不需要 ready 和 listener 关闭，使用 Runner 拉起 Job 并监听 Context 会更轻。
