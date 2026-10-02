# singleton

[English](README.md) | [中文](README_CN.md)

## 简介

`singleton` 持有一个首次使用时才构造的值，连同构造函数返回的错误一起保存。

## 使用方式

```go
import "go-spring.org/stdlib/singleton"

var db singleton.Singleton[*sql.DB]

func Init(ctx context.Context) (*sql.DB, error) {
    return db.Init(func() (*sql.DB, error) {
        return sql.Open("mysql", dsn)
    })
}
```

构造函数在首次调用时执行；它的值和错误会被缓存，因此后续调用返回同一对结果，不会再次执行构造函数。

## 并发

`Init` 可安全并发调用，构造函数至多执行一次。构造函数执行期间锁一直被持有，因此构造函数不能（直接或间接）对同一个
`Singleton` 调用 `Init`——锁不可重入，那样会死锁。

## 许可证

`singleton` 基于 Apache License 2.0 发布，详见 [LICENSE](../../LICENSE)。
