# 05. HTTP Routing

Run:

```bash
go run .
curl http://127.0.0.1:9090/books
curl http://127.0.0.1:9090/books/978-0134190440
curl -X POST http://127.0.0.1:9090/books -H 'Content-Type: application/json' -d '{"isbn":"978-0134494166","title":"Clean Architecture","author":"Robert C. Martin","publisher":"Prentice Hall"}'
curl -X DELETE http://127.0.0.1:9090/books/978-0134494166
```
