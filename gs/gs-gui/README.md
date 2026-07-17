# gs-gui

[English](README.md) | [中文](README_CN.md)

`gs-gui` is an external [gs](../gs) tool that provides a browser-based wizard
for creating Go-Spring projects. It is a thin front-end over `gs init`.

## Usage

Build and install it next to the `gs` binary, then run:

```bash
gs gui
```

This starts a local web server (default port `8639`, or an ephemeral port if
taken), prints the URL, and tries to open your default browser. Fill in the
module path and documentation language, then click **Create project** — the
wizard execs `gs init` and streams its output back to the page.

## How it works

- Follows the gs external-tool protocol: the binary is named `gs-gui` and lives
  next to the `gs` binary; `gs gui` dispatches to it.
- The single-page UI is embedded into the binary via `go:embed`.
- On submit, the server execs the sibling `gs init -m <module> --lang <lang>`
  and streams its combined stdout/stderr to the browser.
