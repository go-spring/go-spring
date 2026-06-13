# 11. Custom Server

Run HTTP and TCP echo server:

```bash
go run .
curl http://127.0.0.1:9090/books
nc 127.0.0.1 10090
```

Disable the custom server:

```bash
go run . -Dbookman.echo-server.enabled=false
```
