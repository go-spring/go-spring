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

The service keeps running. Press `Ctrl+C` to stop.

In another terminal, rewrite a key file and watch it print:

```bash
echo manual-1 > mount/db.user
# prints: db.user: "alice" -> "manual-1"
```

Without `-manual` the example rewrites the mount itself (the kubelet's atomic
`..data` swap), waits for the bound fields to hot-reload, prints
`hot-reload observed: db.user= bob-<hhmmss>`, and exits:

```bash
go run .
```

## Smoke Test

```bash
./check.sh
```

`check.sh` lays down a Secret-style mount, rewrites it and asserts the bound
fields hot-reload, exit code 0 means pass.
