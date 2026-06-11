# 02. Configuration

Run:

```bash
go run .
curl http://127.0.0.1:9090/echo
```

Override with command line:

```bash
go run . -Dspring.http.server.addr=:9091 -Dbookman.echo.prefix=cli
curl http://127.0.0.1:9091/echo
```

Use profile:

```bash
go run . -Dspring.profiles.active=test
```
