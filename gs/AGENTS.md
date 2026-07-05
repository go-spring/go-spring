# gs Subproject Conventions

## Debuggability

- Every CLI entrypoint (`gs` itself and every `gs-<tool>` external command) must expose the shared `-v` count flag (`-v`, `-vv`, `--verbose=N`) and honor the level semantics below.
- **level 0**: prints only `[INFO] <label>` step lines.
- **`-v`**: additionally logs the full argv.
- **`-vv`**: additionally streams child stdout/stderr live and logs cwd + exit code.
- AI-driven refactors must not remove, hide, rename, collapse, or reduce these flags to no-ops when "simplifying" the interface — they are the primary way to debug `gs` and its subcommands, in development and in production alike.
