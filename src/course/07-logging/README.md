# BookMan

[English](README.md) | [中文](README_CN.md)

BookMan is a small book-management example that shows how Go-Spring fits into a layered application: configuration loading, bean injection, HTTP routing, business services, DAO access, SDK wrapping, dynamic configuration refresh, and graceful shutdown of background jobs.

## 1. Directory Structure

```text
conf/                     Configuration files
logs/                     Log files
public/                   Static files
internal/
  app/                    Application layer
    common/httpsvr/       HTTP server and middleware
    controller/           HTTP controllers
  biz/                    Business layer
    job/                  Background jobs
    service/book_service/ Book business service
  dao/book_dao/           In-memory data access layer
  idl/http/proto/         HTTP interface and route registration
  sdk/book_sdk/           External SDK wrapper example
main.go                   Application entry and self-check runner
init.go                   Banner and working directory setup
```

## 2. What This Example Shows

- `main.go` registers a `gs.Runner` that sends HTTP requests after startup, demonstrates the full CRUD flow, and sends `SIGTERM` when the run is done.
- `internal/app/common/httpsvr` customizes `http.ServeMux`, registers generated-style routes, and adds access logging middleware.
- `internal/app/controller` keeps HTTP decoding/encoding in controllers and delegates business behavior to services.
- `internal/biz/service/book_service` composes the DAO and SDK, then returns books enriched with price and dynamic configuration fields.
- `internal/dao/book_dao` uses an in-memory map so the data access layer stays easy to read and test.
- `internal/biz/job` demonstrates how a background task listens to the application context and exits gracefully.

## 3. HTTP API

```text
GET    /books          List books
GET    /books/{isbn}   Get one book
POST   /books          Create or update a book
DELETE /books/{isbn}   Delete a book
GET    /               Static home page
```

Example `POST /books` body:

```json
{
  "title": "Clean Architecture",
  "author": "Robert C. Martin",
  "isbn": "978-0134494166",
  "publisher": "Prentice Hall"
}
```

## 4. Run

```bash
go run .
```

After startup, the runner prints each HTTP step with the status code and response body. It also refreshes the dynamic `dync.refresh.time` property, lists books again, and finally sends a shutdown signal so the background job can stop gracefully.
