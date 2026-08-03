# starter-config-file configtree Example

Directory-of-scalar-keys config hot reload with the `configtree` provider.

## Features

- **Tree import**: each flat key file becomes one property (`db.user`, `db.password`, `server.port`)
- **K8s-style mount**: reproduces a Secret/ConfigMap volume with the atomic `..data` symlink swap
- **Hot reload**: bound `gs.Dync[T]` fields update after the swap, no restart

## Manual Testing

```bash
cd starter-config-file/example-configtree
go run . -manual
```

The program keeps running. Press Ctrl+C to exit. Without `-manual`, `runTest()` runs automatically and exits.

## Smoke Test

```bash
cd starter-config-file/example-configtree
bash check.sh
```

Lays down a Secret-style mount, binds `gs.Dync[string]` fields to `db.user` /
`db.password` / `server.port`, atomically swaps `..data`, and asserts the bound
fields hot-reload. Exits non-zero on failure.

## When to use configtree vs file-watch

- `configtree:<dir>` — a directory of scalar key files (path → key, content → value). Typical of a Kubernetes Secret / env-style ConfigMap mount.
- `file-watch:<file>` — a single whole configuration document. Typical of a ConfigMap key holding `application.yaml`.
