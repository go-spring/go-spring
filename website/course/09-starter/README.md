# 09. Starter

Run with the built-in fallback price client:

```bash
go run .
curl http://127.0.0.1:9090/books
```

Enable the starter-provided client:

```bash
go run . -Dbookman.price.base-url=http://127.0.0.1:18080
```
