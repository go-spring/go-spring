# contract

[English](README.md) | [中文](README_CN.md)

`contract` 是 Go-Spring 版的 Spring Cloud Contract。一份声明式契约——请求形态
加上它必须产生的响应——同时驱动一次服务间调用的**两端**:

- **Provider 端**:[`Verify`](verify.go) 用每条契约回放真实 handler,断言响应匹配。
  provider 一旦偏离约定,测试就会失败。
- **Consumer 端**:[`StubServer`](stub.go) 把同一批契约变成一个桩 HTTP 服务,按
  provider 承诺的样子应答;于是 consumer——即声明式 HTTP 客户端(其调用点
  只持有一个 `*http.Client`)——可以对着一个忠实替身被独立测试。

因为同一份产物同时喂给两端,consumer 的桩绝不可能编造出 provider 实际不会返回的响应。

## 为什么不做 Groovy DSL / 不引 YAML 依赖

契约就是普通 Go 结构体。落盘用 **JSON**,以此守住 `stdlib` 的零依赖约定
(YAML parser 会破坏它)。若偏好 YAML,请自行反序列化后把得到的
`[]contract.Contract` 交给 `Verify` / `StubServer`。

## 契约格式

```json
[
  {
    "name": "greet",
    "request":  { "method": "GET", "path": "/greet", "query": { "name": "Ada" } },
    "response": {
      "status": 200,
      "headers": { "Content-Type": "application/json" },
      "body": { "message": "Hello, Ada!" }
    }
  }
]
```

一个文件可放单个契约对象或其数组。只有你设置了的请求字段才参与匹配:空的
`query`/`headers` 不构成约束,`body` 为空则不检查其内容。合法 JSON 的 body 按
**结构等价**比较(忽略键顺序与空白)。

## 用法

```go
contracts, _ := contract.Load("testdata/greet.contract.json")

// Provider 端——真实 handler 一旦偏离任一契约即失败。
contract.Verify(t, greetHandler(), contracts)          // 进程内 http.Handler
contract.Verify(t, "http://127.0.0.1:8080", contracts) // 或一个运行中的 base URL

// Consumer 端——被测 consumer 调用的桩。
stub := contract.StubServer(t, contracts)
client := stub.Client()
resp, _ := client.Get(stub.URL + "/greet?name=Ada")
```

- `Verify` 用 `t.Errorf` 报告每处不匹配且继续执行(assert 风格,非 fail-fast),
  一次调用即暴露全部失败。
- 未命中任何契约的请求返回 `501 Not Implemented`,并附上已尝试契约清单,让越界
  调用大声失败。

双向示例见 [`contract_test.go`](contract_test.go)。

## 许可证

Apache License 2.0
