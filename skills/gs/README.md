# gs skill (/gs)

A Claude Code Skill for the full Go-Spring project lifecycle. This document only covers how to install the skill; for its capabilities and usage see [`SKILL.md`](SKILL.md).

[English](README.md) | [中文](README_CN.md)

## One-line install

```bash
curl -fsSL https://raw.githubusercontent.com/go-spring/go-spring/main/skills/gs/install.sh | bash
```

The script fetches the skill files from the latest `skills/gs/vX.Y.Z` tag on the remote repo and installs them into your local Claude Code skills directory. On success it prints:

```
Installed gs skill (skills/gs/vX.Y.Z) to ~/.claude/skills/gs
```

## Install branch

The script installs the latest `skills/gs/v*` tag by default. To try a branch before a tag is cut, set the ref via `GS_SKILL_REF`:

```bash
GS_SKILL_REF=main curl -fsSL https://raw.githubusercontent.com/go-spring/go-spring/main/skills/gs/install.sh | bash
```

`GS_SKILL_REF` can be a branch name, tag, or commit.

## Update & uninstall

- **Update**: just re-run the one-line install command above.
- **Uninstall**: remove the install directory, `rm -rf ~/.claude/skills/gs`.
