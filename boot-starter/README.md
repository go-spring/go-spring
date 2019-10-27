# boot-starter

通用的 Go 程序启动器框架。

```
type MyApp struct {
}

func (app *MyApp) Start() {
	fmt.Println("app start")
}

func (app *MyApp) ShutDown() {
	fmt.Println("app shutdown")
}

func TestBootStarter(t *testing.T) {

	go func() {
		defer fmt.Println("go stop")
		fmt.Println("go start")

		time.Sleep(200 * time.Millisecond)
		BootStarter.Exit()
	}()

	BootStarter.Run(new(MyApp))
}
```