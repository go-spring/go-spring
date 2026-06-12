#!/usr/bin/env bash
set -euo pipefail

# Audit every path reachable from the repository's refs and flag large blobs,
# paths matched by current ignore rules, and commonly unwanted committed files.

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
large_threshold_mb=10
output_dir="$script_dir/reports"
allowlist_file="$script_dir/allowlist.tsv"

show_help() {
    cat <<'USAGE'
Usage: ./scripts/git-history-audit/audit-git-history.sh [options]

Scans all reachable git history and reports:
  - every historical path, inferred type, largest blob size, and blob version count
  - large blob/path pairs
  - paths matched by the repository's current ignore rules
  - paths matching commonly unwanted directory/file patterns

Options:
  --large-threshold-mb N   Large blob threshold in MiB, decimal integer (default: 10)
  --output-dir DIR         Report output directory (default: this tool's reports directory)
  --allowlist FILE         Exact-path suspicious finding allowlist
                           (default: this tool's allowlist.tsv)
  -h, --help               Show this help message

Outputs:
  DIR/all-paths.tsv        one row per historical path
  DIR/large-blobs.tsv      blob/path rows at or above the threshold
  DIR/ignored-history.tsv  historical paths matched by current ignore rules
  DIR/suspicious-paths.tsv paths matching unwanted directory/file patterns
  DIR/allowlisted-paths.tsv suspicious paths suppressed by the allowlist
  DIR/suspicious-summary.tsv suspicious path counts grouped by severity/reason

Path fields use TSV-safe escaping: \\, \t, \r, and \n.
USAGE
}

while [ $# -gt 0 ]; do
    case "$1" in
        --large-threshold-mb)
            large_threshold_mb="${2:?missing value for --large-threshold-mb}"
            shift 2
            ;;
        --output-dir)
            output_dir="${2:?missing value for --output-dir}"
            shift 2
            ;;
        --allowlist)
            allowlist_file="${2:?missing value for --allowlist}"
            shift 2
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            echo "Error: unknown option: $1" >&2
            show_help >&2
            exit 1
            ;;
    esac
done

if ! command -v perl >/dev/null 2>&1; then
    echo "Error: perl is required for NUL-safe git path parsing" >&2
    exit 1
fi

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    echo "Error: this script must run inside a non-bare git work tree" >&2
    exit 1
fi

if ! [[ "$large_threshold_mb" =~ ^[0-9]+$ ]]; then
    echo "Error: --large-threshold-mb must be a non-negative integer" >&2
    exit 1
fi

if [ ! -f "$allowlist_file" ]; then
    echo "Error: allowlist file does not exist: $allowlist_file" >&2
    exit 1
fi

repo_root="$(git rev-parse --show-toplevel)"
mkdir -p "$output_dir"

tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/git-history-audit.XXXXXX")"
publish_dir=""
cleanup() {
    rm -rf "$tmp_dir"
    if [ -n "$publish_dir" ]; then
        rm -rf "$publish_dir"
    fi
}
trap cleanup EXIT

report_dir="$tmp_dir/reports"
mkdir -p "$report_dir"
tmp_history="$tmp_dir/history.raw"
tmp_pairs_unsorted="$tmp_dir/blob-path-pairs.unsorted.tsv"
tmp_pairs="$tmp_dir/blob-path-pairs.tsv"
tmp_paths="$tmp_dir/paths.tsv"
tmp_paths_raw="$tmp_dir/paths.raw"
tmp_gitlinks="$tmp_dir/gitlinks.tsv"
tmp_oids="$tmp_dir/blob-oids.txt"
tmp_object_info="$tmp_dir/object-info.txt"
tmp_sizes="$tmp_dir/blob-sizes.tsv"
tmp_objects="$tmp_dir/blob-path-sizes.tsv"
tmp_all_paths_body="$tmp_dir/all-paths.body.tsv"
tmp_large_body="$tmp_dir/large-blobs.body.tsv"
tmp_suspicious_body="$tmp_dir/suspicious-paths.body.tsv"
tmp_suspicious_filtered="$tmp_dir/suspicious-paths.filtered.tsv"
tmp_suspicious_body_sorted="$tmp_dir/suspicious-paths.sorted.tsv"
tmp_allowlist="$tmp_dir/allowlist.tsv"
tmp_allowlisted_body="$tmp_dir/allowlisted-paths.body.tsv"
tmp_allowlisted_body_sorted="$tmp_dir/allowlisted-paths.sorted.tsv"
tmp_allowlist_unused="$tmp_dir/allowlist-unused.tsv"
tmp_suspicious_summary="$tmp_dir/suspicious-summary.body.tsv"
tmp_suspicious_summary_sorted="$tmp_dir/suspicious-summary.sorted.tsv"
tmp_ignored_raw="$tmp_dir/ignored.raw"
tmp_ignored_body="$tmp_dir/ignored-history.body.tsv"

if ! large_threshold_bytes="$(
    perl -MMath::BigInt -e '
        my $mb = Math::BigInt->new(shift);
        my $max_mb = Math::BigInt->new("8796093022207");
        exit 1 if $mb->bcmp($max_mb) > 0;
        print $mb->bmul(1024 * 1024);
    ' "$large_threshold_mb"
)"; then
    echo "Error: --large-threshold-mb is too large" >&2
    exit 1
fi

printf 'Collecting historical paths and blob versions...\n'
git -C "$repo_root" log \
    --all \
    --root \
    -m \
    --raw \
    --no-renames \
    --no-abbrev \
    --full-index \
    --format= \
    -z \
    > "$tmp_history"

PAIRS_FILE="$tmp_pairs_unsorted" \
PATHS_FILE="$tmp_paths" \
RAW_PATHS_FILE="$tmp_paths_raw" \
GITLINKS_FILE="$tmp_gitlinks" \
perl -0ne '
sub escape_tsv {
    my ($value) = @_;
    $value =~ s/\\/\\\\/g;
    $value =~ s/\t/\\t/g;
    $value =~ s/\r/\\r/g;
    $value =~ s/\n/\\n/g;
    return $value;
}

BEGIN {
    open PAIRS, ">", $ENV{PAIRS_FILE} or die "open pairs: $!\n";
}

s/\0\z//;

if (!defined $new_mode) {
    next unless length $_;
    if (/^:[0-7]{6} ([0-7]{6}) [0-9a-f]+ ([0-9a-f]+) [A-Z][0-9]*$/) {
        $new_mode = $1;
        $new_oid = $2;
        next;
    }
    die "unexpected git raw record: ", escape_tsv($_), "\n";
}

my $path = $_;
die "empty path after git raw metadata\n" unless length $path;
$paths{$path} = 1;

if ($new_oid !~ /^0+$/) {
    if ($new_mode eq "160000") {
        $gitlinks{$path} = 1;
    } else {
        print PAIRS $new_oid, "\t", escape_tsv($path), "\n";
    }
}

undef $new_mode;
undef $new_oid;

END {
    die "git raw metadata without a following path\n" if defined $new_mode;
    close PAIRS or die "close pairs: $!\n";
    open PATHS, ">", $ENV{PATHS_FILE} or die "open paths: $!\n";
    open RAW_PATHS, ">", $ENV{RAW_PATHS_FILE} or die "open raw paths: $!\n";
    open GITLINKS, ">", $ENV{GITLINKS_FILE} or die "open gitlinks: $!\n";
    for my $path (sort keys %paths) {
        print PATHS escape_tsv($path), "\n";
        print RAW_PATHS $path, "\0";
    }
    for my $path (sort keys %gitlinks) {
        print GITLINKS escape_tsv($path), "\n";
    }
    close PATHS or die "close paths: $!\n";
    close RAW_PATHS or die "close raw paths: $!\n";
    close GITLINKS or die "close gitlinks: $!\n";
}
' "$tmp_history"

LC_ALL=C sort -u "$tmp_pairs_unsorted" > "$tmp_pairs"
cut -f1 "$tmp_pairs" | LC_ALL=C sort -u > "$tmp_oids"

printf 'Querying historical blob sizes...\n'
git -C "$repo_root" cat-file \
    --batch-check='%(objecttype) %(objectname) %(objectsize)' \
    < "$tmp_oids" \
    > "$tmp_object_info"

awk -v OFS='\t' '
$1 == "blob" && NF == 3 {
    print $2, $3
    next;
}
{
    print "Error: expected historical blob object, got: " $0 > "/dev/stderr"
    invalid=1
}
END {
    exit invalid
}
' "$tmp_object_info" > "$tmp_sizes"

awk -F '\t' -v OFS='\t' '
FILENAME == ARGV[1] {
    size[$1]=$2
    next
}
($1 in size) {
    print $1, size[$1], $2
}
' "$tmp_sizes" "$tmp_pairs" > "$tmp_objects"

awk -F '\t' -v OFS='\t' '
function ext_of(path, base, parts, n) {
    base = path
    sub(/^.*\//, "", base)
    if (base !~ /\./ || base ~ /^\.[^.]+$/) return "[none]"
    n = split(base, parts, ".")
    return "." parts[n]
}
function class_of(path, ext, is_gitlink, lower) {
    if (is_gitlink) return "submodule"
    lower=tolower(path)
    ext=tolower(ext)
    if (lower ~ /(^|\/)vendor\//) return "vendored dependency"
    if (lower ~ /(^|\/)(node_modules|bower_components)\//) return "frontend dependency"
    if (lower ~ /(^|\/)(dist|build|target|out|bin|coverage|htmlcov)\//) return "generated/build output"
    if (lower ~ /(^|\/)(__pycache__|\.pytest_cache|\.mypy_cache|\.tox|\.cache|\.gocache|\.gradle)\//) return "cache"
    if (lower ~ /(^|\/)(\.idea|\.vscode)\//) return "editor config"
    if (lower ~ /(^|\/)\.ds_store$/) return "system file"
    if (lower ~ /\.(png|jpe?g|gif|webp|ico|svg)$/) return "image"
    if (lower ~ /\.(zip|tar|tgz|tar\.gz|tar\.bz2|tar\.xz|gz|bz2|xz|7z|rar)$/) return "archive"
    if (lower ~ /\.(jar|war|ear|class|so|dylib|dll|exe|bin|a|o|pyc)$/) return "binary/build artifact"
    if (lower ~ /\.(pdf|docx?|xlsx?|pptx?)$/) return "document"
    if (lower ~ /\.(go|java|c|cc|cpp|h|hpp|js|jsx|ts|tsx|py|rb|rs|sh|bash|zsh|sql)$/) return "source"
    if (lower ~ /\.(md|txt|rst|adoc)$/) return "text/doc"
    if (lower ~ /\.(json|ya?ml|toml|ini|properties|xml|proto|idl)$/) return "config/schema"
    if (ext == "[none]") return "no extension"
    return "other"
}
FILENAME == ARGV[1] {
    seen[$0]=1
    next
}
FILENAME == ARGV[2] {
    gitlink[$0]=1
    seen[$0]=1
    next
}
{
    blob=$1
    size=$2
    path=$3
    seen[path]=1
    blob_count[path]++
    if (!(path in max_size) || size > max_size[path]) {
        max_size[path]=size
        max_blob[path]=blob
    }
}
END {
    for (path in seen) {
        ext=ext_of(path)
        type=class_of(path, ext, path in gitlink)
        size=(path in max_size ? max_size[path] : 0)
        blob=(path in max_blob ? max_blob[path] : "")
        versions=(path in blob_count ? blob_count[path] : 0)
        printf "%s\t%s\t%s\t%d\t%.2f\t%s\t%d\n", path, type, ext, size, size / 1024 / 1024, blob, versions
    }
}
' "$tmp_paths" "$tmp_gitlinks" "$tmp_objects" > "$tmp_all_paths_body"

{
    printf 'path\ttype\textension\tmax_size_bytes\tmax_size_mib\tlargest_blob\tblob_versions\n'
    LC_ALL=C sort -t $'\t' -k4,4nr -k1,1 "$tmp_all_paths_body"
} > "$report_dir/all-paths.tsv"

awk -F '\t' -v threshold="$large_threshold_bytes" '
$2 >= threshold {
    printf "%d\t%.2f\t%s\t%s\n", $2, $2 / 1024 / 1024, $1, $3
}
' "$tmp_objects" > "$tmp_large_body"

{
    printf 'size_bytes\tsize_mib\tblob\tpath\n'
    LC_ALL=C sort -t $'\t' -k1,1nr -k4,4 "$tmp_large_body"
} > "$report_dir/large-blobs.tsv"

printf 'Checking historical paths against current ignore rules...\n'
ignore_status=0
git -C "$repo_root" check-ignore \
    --no-index \
    -v \
    -z \
    --stdin \
    < "$tmp_paths_raw" \
    > "$tmp_ignored_raw" || ignore_status=$?

if [ "$ignore_status" -ne 0 ] && [ "$ignore_status" -ne 1 ]; then
    echo "Error: git check-ignore failed with status $ignore_status" >&2
    exit "$ignore_status"
fi

perl -0ne '
sub escape_tsv {
    my ($value) = @_;
    $value =~ s/\\/\\\\/g;
    $value =~ s/\t/\\t/g;
    $value =~ s/\r/\\r/g;
    $value =~ s/\n/\\n/g;
    return $value;
}

s/\0\z//;
push @fields, $_;
if (@fields == 4) {
    my ($source, $line, $pattern, $path) = @fields;
    if ($pattern !~ /^!/) {
        print join("\t", map { escape_tsv($_) } @fields), "\n";
    }
    @fields = ();
}

END {
    die "incomplete git check-ignore record\n" if @fields;
}
' "$tmp_ignored_raw" > "$tmp_ignored_body"

{
    printf 'ignore_source\tline\tpattern\tpath\n'
    LC_ALL=C sort -t $'\t' -k4,4 -k1,1 -k2,2n "$tmp_ignored_body"
} > "$report_dir/ignored-history.tsv"

perl -0ne '
sub escape_tsv {
    my ($value) = @_;
    $value =~ s/\\/\\\\/g;
    $value =~ s/\t/\\t/g;
    $value =~ s/\r/\\r/g;
    $value =~ s/\n/\\n/g;
    return $value;
}

sub print_record {
    my ($severity, $reason, $path) = @_;
    print escape_tsv($severity), "\t", escape_tsv($reason), "\t", escape_tsv($path), "\n";
}

sub finding_for {
    my ($path) = @_;
    my $base = $path;
    $base =~ s{^.*/}{};

    return ("critical", "credential or private key file") if $base =~ /\.(pem|key|p12|pfx|keystore|jks)$/i;
    return ("critical", "credential or private key file") if $base =~ /^id_(rsa|dsa|ecdsa|ed25519)$/i;
    return ("critical", "cloud credential file") if $base =~ /^(credentials|config)$/i && $path =~ m{(^|/)\.(aws|azure|gcloud|kube)/}i;
    return ("high", "local environment file") if $base =~ /^\.env(\..+)?$/i && $base !~ /^\.env\.(example|sample|template)$/i;
    return ("high", "database dump") if $base =~ /\.(dump|bak)$/i;
    return ("high", "database dump") if $base =~ /^(dump|backup|database|db)(?:[._-].*)?\.sql$/i;
    return ("high", "database dump") if $path =~ m{(^|/)(dumps?|backups?)/.*\.sql$}i;
    return ("medium", "vendored dependency") if $path =~ m{(^|/)vendor/}i;
    return ("medium", "dependency directory") if $path =~ m{(^|/)(node_modules|bower_components|\.venv|venv)/}i;
    return ("low", "build or coverage output") if $path =~ m{(^|/)(dist|build|target|out|bin|coverage|htmlcov|\.next|\.nuxt)/}i;
    return ("low", "cache directory") if $path =~ m{(^|/)(__pycache__|\.pytest_cache|\.mypy_cache|\.tox|\.cache|\.gocache|\.gradle|\.parcel-cache)/}i;
    return ("low", "editor directory") if $path =~ m{(^|/)(\.idea|\.vscode)/}i;
    return ("low", "operating system file") if $base =~ /^(\.DS_Store|Thumbs\.db|desktop\.ini)$/i;
    return ("low", "Go workspace file") if $base =~ /^go\.work(\.sum)?$/i;
    return ("low", "runtime or test output") if $base =~ /\.(log|pid|coverprofile|prof|pprof|trace)$/i;
    return ("medium", "archive file") if $base =~ /\.(zip|tar|tgz|tar\.gz|tar\.bz2|tar\.xz|gz|bz2|xz|7z|rar)$/i;
    return ("medium", "binary or build artifact") if $base =~ /\.(jar|war|ear|class|so|dylib|dll|exe|bin|a|o|pyc|test|out)$/i;
    return ("low", "package manager cache") if $path =~ m{(^|/)(\.npm|\.pnpm-store|\.yarn/cache|\.m2/repository)/}i;
    return;
}

s/\0\z//;
next unless length $_;
my ($severity, $reason) = finding_for($_);
print_record($severity, $reason, $_) if defined $reason;
' "$tmp_paths_raw" > "$tmp_suspicious_body"

perl -ne '
BEGIN {
    $line_number = 0;
    $data_row_number = 0;
}

$line_number++;
s/\r?\n\z//;
next if /^\s*$/ || /^#/;

$data_row_number++;
my @fields = split(/\t/, $_, -1);
if ($data_row_number == 1) {
    die "invalid allowlist header: expected path<TAB>justification\n"
        unless @fields == 2 && $fields[0] eq "path" && $fields[1] eq "justification";
    next;
}
die "invalid allowlist row $line_number: expected path<TAB>justification\n" unless @fields == 2;

my ($path, $justification) = @fields;
die "invalid allowlist row $line_number: path is empty\n" unless length $path;
die "invalid allowlist row $line_number: justification is empty\n" unless length $justification;
my $unescaped = $path;
$unescaped =~ s/\\[\\trn]//g;
die "invalid allowlist row $line_number: unsupported path escape\n" if $unescaped =~ /\\/;
die "duplicate allowlist path at row $line_number: $path\n" if $seen{$path}++;

print $path, "\t", $justification, "\n";

END {
    die "invalid allowlist: missing path<TAB>justification header\n" unless $data_row_number;
}
' "$allowlist_file" > "$tmp_allowlist"

: > "$tmp_allowlisted_body"
: > "$tmp_suspicious_filtered"
: > "$tmp_allowlist_unused"
awk -F '\t' -v OFS='\t' \
    -v allowlisted_file="$tmp_allowlisted_body" \
    -v filtered_file="$tmp_suspicious_filtered" \
    -v unused_file="$tmp_allowlist_unused" '
FILENAME == ARGV[1] {
    justification[$1]=$2
    order[++allowlist_count]=$1
    next
}
{
    path=$3
    if (path in justification) {
        print $0, justification[path] > allowlisted_file
        used[path]=1
    } else {
        print $0 > filtered_file
    }
}
END {
    for (i=1; i<=allowlist_count; i++) {
        path=order[i]
        if (!(path in used)) print path, justification[path] > unused_file
    }
}
' "$tmp_allowlist" "$tmp_suspicious_body"

awk -F '\t' -v OFS='\t' '
BEGIN {
    rank["critical"] = 1
    rank["high"] = 2
    rank["medium"] = 3
    rank["low"] = 4
}
{
    print rank[$1], $0
}
' "$tmp_suspicious_filtered" | LC_ALL=C sort -t $'\t' -k1,1n -k3,3 -k4,4 | cut -f2- > "$tmp_suspicious_body_sorted"
{
    printf 'severity\treason\tpath\n'
    cat "$tmp_suspicious_body_sorted"
} > "$report_dir/suspicious-paths.tsv"

awk -F '\t' -v OFS='\t' '
BEGIN {
    rank["critical"] = 1
    rank["high"] = 2
    rank["medium"] = 3
    rank["low"] = 4
}
{
    print rank[$1], $0
}
' "$tmp_allowlisted_body" | LC_ALL=C sort -t $'\t' -k1,1n -k3,3 -k4,4 | cut -f2- > "$tmp_allowlisted_body_sorted"
{
    printf 'severity\treason\tpath\tjustification\n'
    cat "$tmp_allowlisted_body_sorted"
} > "$report_dir/allowlisted-paths.tsv"

awk -F '\t' -v OFS='\t' '
NR > 1 { count[$1 FS $2]++ }
END {
    print "severity", "reason", "count"
    for (finding in count) print finding, count[finding]
}
' "$report_dir/suspicious-paths.tsv" > "$tmp_suspicious_summary"
awk -F '\t' -v OFS='\t' '
BEGIN {
    rank["critical"] = 1
    rank["high"] = 2
    rank["medium"] = 3
    rank["low"] = 4
}
NR > 1 {
    print rank[$1], $0
}
' "$tmp_suspicious_summary" | LC_ALL=C sort -t $'\t' -k1,1n -k4,4nr -k3,3 | cut -f2- > "$tmp_suspicious_summary_sorted"
{
    printf 'severity\treason\tcount\n'
    cat "$tmp_suspicious_summary_sorted"
} > "$report_dir/suspicious-summary.tsv"

report_names=(
    all-paths.tsv
    large-blobs.tsv
    ignored-history.tsv
    suspicious-paths.tsv
    allowlisted-paths.tsv
    suspicious-summary.tsv
)
publish_dir="$(mktemp -d "$output_dir/.git-history-audit.publish.XXXXXX")"
for report_name in "${report_names[@]}"; do
    cp "$report_dir/$report_name" "$publish_dir/$report_name"
done
for report_name in "${report_names[@]}"; do
    mv "$publish_dir/$report_name" "$output_dir/$report_name"
done
rmdir "$publish_dir"
publish_dir=""

large_row_count=$(awk 'NR > 1 { count++ } END { print count + 0 }' "$output_dir/large-blobs.tsv")
large_blob_count=$(awk -F '\t' 'NR > 1 && !seen[$3]++ { count++ } END { print count + 0 }' "$output_dir/large-blobs.tsv")
ignored_count=$(awk 'NR > 1 { count++ } END { print count + 0 }' "$output_dir/ignored-history.tsv")
suspicious_count=$(awk 'NR > 1 { count++ } END { print count + 0 }' "$output_dir/suspicious-paths.tsv")
allowlisted_count=$(awk 'NR > 1 { count++ } END { print count + 0 }' "$output_dir/allowlisted-paths.tsv")
unused_allowlist_count=$(awk 'END { print NR + 0 }' "$tmp_allowlist_unused")
path_count=$(awk 'NR > 1 { count++ } END { print count + 0 }' "$output_dir/all-paths.tsv")

cat <<REPORT

Git history audit complete.

Reports written to: $output_dir
  all paths:          $output_dir/all-paths.tsv ($path_count paths)
  large blobs:        $output_dir/large-blobs.tsv ($large_blob_count blobs, $large_row_count blob/path rows >= ${large_threshold_mb} MiB)
  ignored history:    $output_dir/ignored-history.tsv ($ignored_count paths matched by current ignore rules)
  suspicious paths:   $output_dir/suspicious-paths.tsv ($suspicious_count paths)
  allowlisted paths:  $output_dir/allowlisted-paths.tsv ($allowlisted_count suppressed findings)
  suspicious summary: $output_dir/suspicious-summary.tsv

Top large blob/path rows:
REPORT
awk -F '\t' 'NR == 1 { next } NR <= 11 { printf "  %8.2f MiB  %s\n", $2, $4 }' "$output_dir/large-blobs.tsv"

cat <<REPORT

Top paths matched by current ignore rules:
REPORT
awk -F '\t' 'NR == 1 { next } NR <= 11 { printf "  %-24s %s\n", $3, $4 }' "$output_dir/ignored-history.tsv"

cat <<REPORT

Suspicious path summary:
REPORT
awk -F '\t' 'NR == 1 { next } NR <= 11 { printf "  %-8s %-28s %s\n", $1, $2, $3 }' "$output_dir/suspicious-summary.tsv"

cat <<REPORT

Top suspicious paths:
REPORT
awk -F '\t' 'NR == 1 { next } NR <= 11 { printf "  %-8s %-28s %s\n", $1, $2, $3 }' "$output_dir/suspicious-paths.tsv"

cat <<REPORT

Allowlisted suspicious paths:
REPORT
awk -F '\t' 'NR == 1 { next } NR <= 11 { printf "  %-8s %-28s %s  (%s)\n", $1, $2, $3, $4 }' "$output_dir/allowlisted-paths.tsv"

if [ "$unused_allowlist_count" -gt 0 ]; then
    printf '\nWarning: %d allowlist entries did not match a suspicious historical path:\n' "$unused_allowlist_count" >&2
    awk -F '\t' '{ printf "  %s  (%s)\n", $1, $2 }' "$tmp_allowlist_unused" >&2
fi
