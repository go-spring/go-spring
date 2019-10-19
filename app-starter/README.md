# app-starter

通用的 Go 程序启动器。

```go
//
// 实现了一个通用的启动框架。
//
package AppStarter

import (
	"os"
	"os/signal"
	"syscall"
)

//
// 应用执行器。
//
type AppRunner interface {
	Start()    // 启动执行器
	ShutDown() // 关闭执行器
}

var exitChan chan struct{}

//
// 启动应用执行器。
//
func Run(runner AppRunner) {

	exitChan = make(chan struct{})

	go func() {
		// 响应控制台的 Ctrl+C 及 kill 命令。

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

		<-sig

		SafeCloseChan(exitChan)
	}()

	runner.Start()

	<-exitChan

	runner.ShutDown()
}

//
// 退出当前的应用程序。
//
func Exit() {
	SafeCloseChan(exitChan)
}

func SafeCloseChan(ch chan struct{}) {
	select {
	case <-ch:
		// Already closed. Don't close again.
	default:
		// Safe to close here.
		close(ch)
	}
}
```