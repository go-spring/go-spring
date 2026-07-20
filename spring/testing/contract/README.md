# contract

[English](README.md) | [中文](README_CN.md)

`contract` is the Go-Spring equivalent of Spring Cloud Contract. A single
declarative contract — a request shape paired with the response it must produce —
drives **both** ends of a service-to-service call:

- **Provider side**: [`Verify`](verify.go) replays every contract against the real
  handler and asserts the response matches. The provider cannot drift from the
  agreement without a test failure.
- **Consumer side**: [`StubServer`](stub.go) turns the same contracts into a stub
  HTTP server that answers exactly as the provider promised, so a consumer — a
  [Task 01 declarative HTTP client](../../httpx) whose generated call site only
  holds an `*http.Client` — can be tested in isolation against a faithful double.

Because one artifact feeds both directions, a consumer stub can never encode a
response the provider does not actually return.

## Why no Groovy DSL / no YAML dependency

Contracts are plain Go structs. On disk they are **JSON**, so the package keeps
`stdlib`'s zero-dependency rule (the same reason `spring/i18n` declines a YAML
dependency and takes already-parsed input). If you prefer YAML, unmarshal it
yourself and hand the resulting `[]contract.Contract` to `Verify` / `StubServer`.

## Contract format

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

A file holds one contract object or an array of them. Only the request fields you
set take part in matching: an empty `query`/`headers` imposes no constraint, and a
nil `body` is not inspected. Bodies that are valid JSON compare by **structural
equality** (key order and whitespace ignored).

## Usage

```go
contracts, _ := contract.Load("testdata/greet.contract.json")

// Provider side — fails if the real handler drifts from any contract.
contract.Verify(t, greetHandler(), contracts)          // in-process http.Handler
contract.Verify(t, "http://127.0.0.1:8080", contracts) // or a running base URL

// Consumer side — a stub the consumer under test calls.
stub := contract.StubServer(t, contracts)
client := stub.Client()
resp, _ := client.Get(stub.URL + "/greet?name=Ada")
```

- `Verify` reports every mismatch with `t.Errorf` and keeps going (assert-style,
  not fail-fast), so one call surfaces all failures at once.
- A request matching no contract gets `501 Not Implemented` with a diagnostic
  listing what was tried, so out-of-contract calls fail loudly.

See [`contract_test.go`](contract_test.go) for the both-directions example.

## License

Apache License 2.0
