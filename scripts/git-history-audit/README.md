# Git History Audit

用于扫描当前仓库所有 ref 可达的 Git 历史，统计曾经提交过的文件路径、最大 blob 大小和历史 blob 版本数量，并标记大文件、命中当前忽略规则的历史文件，以及通常不应提交的目录或文件。

## 使用方式

在仓库根目录执行：

```bash
./scripts/git-history-audit/audit-git-history.sh
```

可选参数：

```bash
./scripts/git-history-audit/audit-git-history.sh --large-threshold-mb 20
./scripts/git-history-audit/audit-git-history.sh --output-dir /tmp/git-history-audit
./scripts/git-history-audit/audit-git-history.sh --allowlist /path/to/allowlist.tsv
```

参数说明：

- `--large-threshold-mb N`：大文件阈值，单位 MiB，默认 `10`；`N` 必须是 `0` 到 `8796093022207` 之间的十进制整数。
- `--output-dir DIR`：报告输出目录，默认 `scripts/git-history-audit/reports/`。
- `--allowlist FILE`：可疑路径精确匹配豁免表，默认 `scripts/git-history-audit/allowlist.tsv`。

运行要求：

- 必须在非 bare Git 工作树中执行，脚本会自动定位仓库根目录。
- 依赖 `git`、`perl`、`awk`、`sort`、`cut`、`cp`、`mv` 和常规 POSIX shell 工具。

## 输出文件

默认输出到：

```text
scripts/git-history-audit/reports/
```

包含六份 TSV 报告：

- `all-paths.tsv`：历史中出现过的所有文件路径。
- `large-blobs.tsv`：达到或超过阈值的大 blob/path 记录。
- `ignored-history.tsv`：命中当前仓库忽略规则的历史路径。
- `suspicious-paths.tsv`：命中可疑目录或文件规则的路径。
- `allowlisted-paths.tsv`：命中可疑规则但被 allowlist 豁免的路径及理由。
- `suspicious-summary.tsv`：按严重程度和原因汇总可疑路径数量。

路径及规则字段使用 TSV 安全转义：

- `\` 写为 `\\`
- 制表符写为 `\t`
- 回车写为 `\r`
- 换行写为 `\n`

### all-paths.tsv 字段

| 字段 | 说明 |
| --- | --- |
| `path` | Git 历史中的文件路径 |
| `type` | 根据路径和扩展名推断的类型 |
| `extension` | 文件扩展名；无扩展名时为 `[none]` |
| `max_size_bytes` | 该路径历史上出现过的最大 blob 大小，单位字节 |
| `max_size_mib` | 该路径历史上出现过的最大 blob 大小，单位 MiB |
| `largest_blob` | 最大 blob 的对象 ID |
| `blob_versions` | 该路径在历史中对应过的 blob 版本数量 |

对于只作为 submodule 出现过的路径，`type` 为 `submodule`，`max_size_bytes` 和 `blob_versions` 为 `0`，`largest_blob` 为空；gitlink 指向的是 commit，不是 blob。

### large-blobs.tsv 字段

| 字段 | 说明 |
| --- | --- |
| `size_bytes` | blob 大小，单位字节 |
| `size_mib` | blob 大小，单位 MiB |
| `blob` | blob 对象 ID |
| `path` | 该 blob 对应的历史路径 |

### ignored-history.tsv 字段

| 字段 | 说明 |
| --- | --- |
| `ignore_source` | 命中的当前 `.gitignore` 或其他 Git 忽略规则来源 |
| `line` | 规则所在行 |
| `pattern` | 命中的忽略模式 |
| `path` | 历史中出现过且命中规则的路径 |

### suspicious-paths.tsv 字段

| 字段 | 说明 |
| --- | --- |
| `severity` | 严重程度：`critical`、`high`、`medium` 或 `low` |
| `reason` | 命中的可疑规则 |
| `path` | Git 历史中的文件路径 |

### suspicious-summary.tsv 字段

| 字段 | 说明 |
| --- | --- |
| `severity` | 严重程度 |
| `reason` | 可疑规则 |
| `count` | 命中路径数量 |

### allowlisted-paths.tsv 字段

| 字段 | 说明 |
| --- | --- |
| `severity` | 原始可疑规则的严重程度 |
| `reason` | 原始可疑规则 |
| `path` | 被豁免的精确历史路径 |
| `justification` | allowlist 中记录的豁免理由 |

## 实现说明

脚本只读取 Git 历史，不修改 Git 对象或工作区业务文件。

核心流程：

1. 使用 `git log --all --root -m --raw --no-renames --no-abbrev --full-index -z` 收集每次提交中出现的路径与新 blob ID。
2. 使用严格的元数据/路径状态机解析 raw diff，并对 `blob ID / path` 去重，确保冒号开头的路径、复制、重命名、合并提交和空文件都能正确统计。
3. 使用 `git cat-file --batch-check` 批量查询唯一 blob 的对象类型和大小；缺失对象或非 blob 对象会导致检查失败。
4. 使用 `git check-ignore --no-index -v -z --stdin` 将历史路径与当前工作树的忽略规则匹配。
5. 单独识别 Git submodule（gitlink），避免把它误报为空文件。
6. 将可疑路径与精确路径 allowlist 匹配，分别生成有效发现和已豁免发现。
7. 使用 `awk` 和 `perl` 根据路径、扩展名和大小生成 TSV 报告。

Git 原始输出和忽略规则匹配均使用 NUL 分隔，避免空格、制表符或换行文件名破坏解析。

## 检查规则

脚本当前包含三类规则：

1. `all-paths.tsv` 中的文件类型推断规则。
2. `ignored-history.tsv` 中由当前 Git 忽略配置提供的规则。
3. `suspicious-paths.tsv` 中的可疑路径命中规则。

文件类型和可疑路径规则都基于历史路径字符串和文件扩展名判断，不读取文件内容。除扩展名字段保留原始大小写外，规则匹配不区分大小写。

### 大文件规则

`large-blobs.tsv` 会输出所有大小大于等于阈值的历史 blob：

```text
blob_size >= large_threshold_mb * 1024 * 1024
```

默认阈值为 `10 MiB`，可通过 `--large-threshold-mb N` 调整。

### 文件类型推断规则

`all-paths.tsv` 的 `type` 字段按以下顺序匹配；先命中的规则优先生效。目录规则要求路径中存在对应的目录段，例如 `vendor/file.go` 会命中，但普通文件名 `vendor` 不会命中。

| 类型 | 命中规则 |
| --- | --- |
| `submodule` | 历史中该路径曾使用 gitlink 模式 |
| `vendored dependency` | 路径包含 `vendor/` |
| `frontend dependency` | 路径包含 `node_modules/` 或 `bower_components/` |
| `generated/build output` | 路径包含 `dist/`、`build/`、`target/`、`out/`、`bin/`、`coverage/`、`htmlcov/` |
| `cache` | 路径包含 `__pycache__/`、`.pytest_cache/`、`.mypy_cache/`、`.tox/`、`.cache/`、`.gocache/`、`.gradle/` |
| `editor config` | 路径包含 `.idea/` 或 `.vscode/` |
| `system file` | 路径为 `.DS_Store` |
| `image` | 扩展名为 `.png`、`.jpg`、`.jpeg`、`.gif`、`.webp`、`.ico`、`.svg` |
| `archive` | 扩展名为 `.zip`、`.tar`、`.tgz`、`.tar.gz`、`.tar.bz2`、`.tar.xz`、`.gz`、`.bz2`、`.xz`、`.7z`、`.rar` |
| `binary/build artifact` | 扩展名为 `.jar`、`.war`、`.ear`、`.class`、`.so`、`.dylib`、`.dll`、`.exe`、`.bin`、`.a`、`.o`、`.pyc` |
| `document` | 扩展名为 `.pdf`、`.doc`、`.docx`、`.xls`、`.xlsx`、`.ppt`、`.pptx` |
| `source` | 扩展名为 `.go`、`.java`、`.c`、`.cc`、`.cpp`、`.h`、`.hpp`、`.js`、`.jsx`、`.ts`、`.tsx`、`.py`、`.rb`、`.rs`、`.sh`、`.bash`、`.zsh`、`.sql` |
| `text/doc` | 扩展名为 `.md`、`.txt`、`.rst`、`.adoc` |
| `config/schema` | 扩展名为 `.json`、`.yaml`、`.yml`、`.toml`、`.ini`、`.properties`、`.xml`、`.proto`、`.idl` |
| `no extension` | 文件名没有扩展名，或文件名形如 `.gitignore` 这类单段隐藏文件 |
| `other` | 未命中以上任何规则 |

扩展名提取规则：

- 只取路径最后一段文件名。
- 文件名没有 `.` 时，扩展名为 `[none]`。
- 文件名形如 `.gitignore`、`.keep` 这类单段隐藏文件时，扩展名为 `[none]`。
- 其他情况取最后一个 `.` 之后的内容作为扩展名，例如 `foo.test.go` 的扩展名为 `.go`。

### 可疑路径规则

`suspicious-paths.tsv` 严格按下表顺序匹配，每个路径只输出第一个命中的规则。所有匹配均不区分大小写。

| severity | reason | 命中规则 |
| --- | --- | --- |
| `critical` | `credential or private key file` | 文件扩展名为 `.pem`、`.key`、`.p12`、`.pfx`、`.keystore`、`.jks` |
| `critical` | `credential or private key file` | 文件名为 `id_rsa`、`id_dsa`、`id_ecdsa`、`id_ed25519` |
| `critical` | `cloud credential file` | 文件名为 `credentials` 或 `config`，且路径位于 `.aws/`、`.azure/`、`.gcloud/`、`.kube/` 目录下 |
| `high` | `local environment file` | 文件名为 `.env` 或 `.env.*`，但精确的 `.env.example`、`.env.sample`、`.env.template` 除外 |
| `high` | `database dump` | 文件扩展名为 `.dump` 或 `.bak` |
| `high` | `database dump` | SQL 文件名为 `dump.sql`、`backup.sql`、`database.sql`、`db.sql`，或这些前缀后接 `.`、`_`、`-` 和其他文本 |
| `high` | `database dump` | `.sql` 文件位于 `dump/`、`dumps/`、`backup/` 或 `backups/` 目录下 |
| `medium` | `vendored dependency` | 路径包含 `vendor/` 目录段 |
| `medium` | `dependency directory` | 路径包含 `node_modules/`、`bower_components/`、`.venv/`、`venv/` 目录段 |
| `low` | `build or coverage output` | 路径包含 `dist/`、`build/`、`target/`、`out/`、`bin/`、`coverage/`、`htmlcov/`、`.next/`、`.nuxt/` 目录段 |
| `low` | `cache directory` | 路径包含 `__pycache__/`、`.pytest_cache/`、`.mypy_cache/`、`.tox/`、`.cache/`、`.gocache/`、`.gradle/`、`.parcel-cache/` 目录段 |
| `low` | `editor directory` | 路径包含 `.idea/` 或 `.vscode/` 目录段 |
| `low` | `operating system file` | 文件名为 `.DS_Store`、`Thumbs.db`、`desktop.ini` |
| `low` | `Go workspace file` | 文件名为 `go.work` 或 `go.work.sum` |
| `low` | `runtime or test output` | 文件扩展名为 `.log`、`.pid`、`.coverprofile`、`.prof`、`.pprof`、`.trace` |
| `medium` | `archive file` | 文件扩展名为 `.zip`、`.tar`、`.tgz`、`.tar.gz`、`.tar.bz2`、`.tar.xz`、`.gz`、`.bz2`、`.xz`、`.7z`、`.rar` |
| `medium` | `binary or build artifact` | 文件扩展名为 `.jar`、`.war`、`.ear`、`.class`、`.so`、`.dylib`、`.dll`、`.exe`、`.bin`、`.a`、`.o`、`.pyc`、`.test`、`.out` |
| `low` | `package manager cache` | 路径包含 `.npm/`、`.pnpm-store/`、`.yarn/cache/`、`.m2/repository/` 目录段 |

安全类规则优先于目录类规则，例如 `vendor/.env` 仍会按本地环境文件标为 `high`，不会被 vendor 规则降级。

由于采用首个命中规则，位于包管理器缓存中的归档或二进制文件会优先按 `archive file` 或 `binary or build artifact` 报告。

### Allowlist 规则

allowlist 是带表头的 TSV 文件，每行包含两个字段：

```text
path	justification
docs/example/runtime.log	已确认是历史文档夹具，当前分支已删除
```

- `path` 必须是仓库根目录相对路径，并与 `suspicious-paths.tsv` 中的转义形式完全一致。
- 匹配是精确路径匹配，不支持 glob 或正则表达式。
- `justification` 必填，用于记录接受风险的理由。
- 空行和以 `#` 开头的注释行会被忽略。
- allowlist 只过滤 `suspicious-paths.tsv` 和 `suspicious-summary.tsv`，不会从 `all-paths.tsv`、`large-blobs.tsv` 或 `ignored-history.tsv` 隐藏路径。
- 命中的豁免项写入 `allowlisted-paths.tsv`；未命中的 allowlist 项会在标准错误中产生警告。
- 路径包含反斜杠、制表符、回车或换行时，使用报告相同的 `\\`、`\t`、`\r`、`\n` 转义。

## 注意事项

- 报告目录 `scripts/git-history-audit/reports/` 已加入根 `.gitignore`，默认不会被提交。
- `ignored-history.tsv` 使用当前检出的 `.gitignore`、`.git/info/exclude` 和 Git 配置的全局忽略规则，不代表文件提交当时的忽略规则；被 `!pattern` 重新包含的路径不会列入报告。
- `suspicious-paths.tsv` 是启发式报告，`dist/`、`bin/`、证书等路径可能是项目有意提交的内容，需要人工确认。
- `suspicious-paths.tsv` 按 `critical`、`high`、`medium`、`low` 排序，同级再按原因和路径排序；汇总报告同样按严重程度排序，同一严重程度内按数量倒序。
- 同一个 blob 可能对应多个历史路径；`large-blobs.tsv` 因此可能为同一个 blob 输出多行。
- 脚本扫描 `--all` 可达对象，不包含已经无法通过任何 ref 到达的悬空对象。
- 六份报告全部生成成功后才会发布到输出目录；扫描失败时保留上一次成功生成的报告。
- 如果要彻底从 Git 历史中移除大文件，需要单独使用历史重写工具；本脚本只负责发现问题。
