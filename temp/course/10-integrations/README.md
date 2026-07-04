# 10. Integrations

Default run uses memory DAO and no cache:

```bash
go run .
curl http://127.0.0.1:9090/books
```

Integration profile switches to fake MySQL DAO, enables cache, and exposes a pprof placeholder:

```bash
go run . -Dspring.profiles.active=integration
curl http://127.0.0.1:9090/debug/pprof/
```
