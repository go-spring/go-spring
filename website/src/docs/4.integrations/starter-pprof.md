# starter-pprof - pprof 性能分析集成

> pprof 是 Go 内置的性能分析工具，starter 提供便捷集成。

## 安装

```go
import _ "github.com/go-spring/starter-pprof"
```

## 配置

```properties
# 是否启用 (默认 true)
pprof.enable=true

# 监听地址 (默认 :6060)
pprof.addr=:6060
```

## 使用

启动应用后，可以通过以下方式进行性能分析：

### 命令行

```bash
# 查看 CPU 概览
go tool pprof http://localhost:6060/debug/pprof/profile

# 查看堆内存
go tool pprof http://localhost:6060/debug/pprof/heap

# 查看 goroutine
go tool pprof http://localhost:6060/debug/pprof/goroutine
```

### 网页界面

访问 `http://localhost:6060/debug/pprof/` 在浏览器中查看交互式火焰图。
