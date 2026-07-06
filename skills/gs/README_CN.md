# gs skill (/gs)

面向 Go-Spring 项目全生命周期的 Claude Code Skill。本文只讲 skill 的安装方法，其能力与用法见 [`SKILL.md`](SKILL.md)。

[English](README.md) | [中文](README_CN.md)

## 一键安装

```bash
curl -fsSL https://raw.githubusercontent.com/go-spring/go-spring/main/skills/gs/install.sh | bash
```

脚本会从远端仓库最新的 `skills/gs/vX.Y.Z` tag 拉取 skill 文件，安装到本地 Claude Code 的 skills 目录。安装成功后输出：

```
Installed gs skill (skills/gs/vX.Y.Z) to ~/.claude/skills/gs
```

## 安装分支

脚本默认安装最新的 `skills/gs/v*` tag。如果想在打 tag 之前先试装某个分支，可以通过 `GS_SKILL_REF` 指定 ref：

```bash
GS_SKILL_REF=main curl -fsSL https://raw.githubusercontent.com/go-spring/go-spring/main/skills/gs/install.sh | bash
```

`GS_SKILL_REF` 可以是分支名、tag 或者 commit。

## 更新与卸载

- **更新**：重跑上面的一键安装命令即可。
- **卸载**：删除安装目录即可，`rm -rf ~/.claude/skills/gs`。
