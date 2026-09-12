# starter-config-apollo example

Self-contained cold-load: the example starts a mock Apollo config service
(meta + configfiles endpoints), imports the starter, and asserts the remote
property cold-loads into a Dync field. No docker, no real Apollo stack.

## Run

```bash
go run .
./check.sh   # the smoke test
```
