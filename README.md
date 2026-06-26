# mr-queue

`mr-queue` is a Go CLI with a local web panel for serial merge request queue automation.
The first provider adapter targets GitCode.

Use a private queue branch as the source of prepared commits. It processes one
commit at a time:

1. fetch the private queue branch and community target branch
2. read the next unmerged commit from the queue branch
3. create a temporary worktree at the latest community target branch
4. cherry-pick exactly one queue commit onto a per-commit MR branch
5. push that MR branch to the private repository
6. create an MR from the private MR branch to the community target branch
7. add the configured review comment and approval with the reviewer account
8. merge with the maintainer account
9. move to the next commit only after the current MR is merged

Tokens are read from `.env` through names configured in `mr-queue.yml`. Real token
values are not stored in YAML, logs, or the state file.

## Quick Start

```bash
cp mr-queue.yml.example mr-queue.yml
cp .env.example .env
```

Edit `mr-queue.yml` for repositories, branches, and workflow settings. Edit `.env`
with fresh provider tokens.

Start the local web panel:

```bash
go run ./cmd/mr-queue serve --config mr-queue.yml
```

Open:

```text
http://127.0.0.1:8787/
```

Preload the queue without pushing branches or creating MRs. Syncing replaces the
visible queue with the current configured `commit_range`, so old range entries do
not remain mixed into the panel:

```bash
go run ./cmd/mr-queue sync-queue --config mr-queue.yml
```

When local refs are already up to date and the environment cannot write to the
target repository's `.git/FETCH_HEAD`, add `--skip-fetch` for preview-only local
debugging:

```bash
go run ./cmd/mr-queue sync-queue --config mr-queue.yml --skip-fetch
```

Run one commit without the web panel:

```bash
go run ./cmd/mr-queue run --config mr-queue.yml
```

Print safe config without exposing token values:

```bash
go run ./cmd/mr-queue dry-run --config mr-queue.yml
```

## MR Branch Names

MR branch names are configurable under `private`. The default template keeps the
old behavior:

```yaml
private:
  branch_prefix: "mr-queue"
  branch_template: "{prefix}-{sha12}"
```

For more readable per-commit branches, include the commit title:

```yaml
private:
  branch_prefix: "feat"
  branch_template: "{prefix}-{title_or_sha12}"
```

Supported placeholders are `{prefix}`, `{title}`, `{title_or_sha12}`, `{sha12}`,
and `{sha}`. The title is converted to a safe branch slug and capped in length.
`{title}` falls back to `commit` if the title cannot produce a safe slug.
`{title_or_sha12}` falls back to the 12-character commit SHA instead.

## External Bot Merge Mode

For repositories where a bot merges after reviewer commands, set
`merge_method: "external"`. In this mode the tool waits for the configured CLA
comment, posts the reviewer command comment, and then polls the MR until the
platform reports it merged. It does not call the maintainer merge API.

```yaml
workflow:
  merge_method: "external"
  required_comment_text: "CLA Signature Pass"
  review_comment: |
    /lgtm
    /approve
  approve: false
  wait_check_delay_min: "10s"
  wait_check_delay_max: "30s"
  next_pr_delay_min: "1m"
  next_pr_delay_max: "5m"
```

`wait_check_delay_min` and `wait_check_delay_max` control how often the tool
checks waiting MRs for required comments or external merge completion.
`next_pr_delay_min` and `next_pr_delay_max` control the random delay after a
commit reaches `merged` before the next MR is created. The web panel lets you
override both delay ranges, the working time window, and maximum merged commits
for each automatic run. The merged limit counts only commits that reach `merged`
during that run.

## Build

```bash
go build -o dist/mr-queue ./cmd/mr-queue
```

Print version information:

```bash
go run ./cmd/mr-queue version
```

## Versioning And Releases

The project version lives in `VERSION`. Release binaries receive version metadata
through Go linker flags, so release builds report the tag, git commit, and build
time:

```bash
mr-queue version
```

GitHub Releases are built by `.github/workflows/release.yml` whenever a `v*` tag
is pushed. The workflow runs tests, builds Linux/macOS/Windows artifacts, writes
`checksums.txt`, and creates the GitHub Release.

Create and publish a GitHub release:

```bash
version="$(cat VERSION)"
git tag "v${version}"
git push github main
git push github "v${version}"
```

## Install From GitHub

Install the latest GitHub Release on Linux or macOS:

```bash
curl -fsSL https://raw.githubusercontent.com/smileQiny/mr-queue/main/scripts/install.sh | sh
```

Install a specific version:

```bash
curl -fsSL https://raw.githubusercontent.com/smileQiny/mr-queue/main/scripts/install.sh | \
  MR_QUEUE_VERSION=v0.1.2 sh
```

Install to a custom directory:

```bash
curl -fsSL https://raw.githubusercontent.com/smileQiny/mr-queue/main/scripts/install.sh | \
  INSTALL_DIR="$HOME/.local/bin" sh
```

Download a release artifact manually:

```bash
version="v0.1.2"
os="darwin"   # linux or darwin
arch="arm64"  # amd64 or arm64
curl -LO "https://github.com/smileQiny/mr-queue/releases/download/${version}/mr-queue_${os}_${arch}.tar.gz"
tar -xzf "mr-queue_${os}_${arch}.tar.gz"
sudo install -m 0755 "mr-queue_${os}_${arch}/mr-queue" /usr/local/bin/mr-queue
mr-queue version
```

Deploy the local web panel:

```bash
mkdir -p "$HOME/mr-queue"
cd "$HOME/mr-queue"
cp /path/to/mr-queue.yml .
cp /path/to/.env .
mr-queue serve --config mr-queue.yml --env .env --addr 127.0.0.1:8787
```

Run it in the background on macOS with `launchctl`:

```bash
home_dir="$HOME"
cat > "$HOME/Library/LaunchAgents/com.mr-queue.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>com.mr-queue</string>
  <key>ProgramArguments</key>
  <array>
    <string>/usr/local/bin/mr-queue</string>
    <string>serve</string>
    <string>--config</string>
    <string>${home_dir}/mr-queue/mr-queue.yml</string>
    <string>--env</string>
    <string>${home_dir}/mr-queue/.env</string>
    <string>--addr</string>
    <string>127.0.0.1:8787</string>
  </array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>/tmp/mr-queue.log</string>
  <key>StandardErrorPath</key><string>/tmp/mr-queue.err.log</string>
</dict>
</plist>
PLIST

launchctl bootstrap "gui/$(id -u)" "$HOME/Library/LaunchAgents/com.mr-queue.plist"
launchctl kickstart -k "gui/$(id -u)/com.mr-queue"
```

Stop the macOS background service:

```bash
launchctl bootout "gui/$(id -u)" "$HOME/Library/LaunchAgents/com.mr-queue.plist"
```

## Same-Repository Test Loop

For a closed-loop test in your own fork, point `community` to the same repository
and set `queue.base_ref` to the target test branch, for example:

```yaml
queue:
  remote: "private"
  branch: "new-features"
  base_ref: "private/master-test"

community:
  remote: "private"
  owner: "smileQiny"
  repo: "syskits"
  branch: "master-test"
```

Click `同步队列` in the web panel first. That only loads commit metadata into the
state file. `运行下一条` and `自动运行` are the actions that push per-commit
branches, create MRs, review, and merge.

Before pushing an MR branch, `mr-queue` checks whether the exact commit patch is
already present on the target base branch. If it is already present, or if Git
reports an empty cherry-pick, the task is marked `skipped` and automatic runs
continue to the next commit without waiting for the next-PR delay.
