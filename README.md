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

Run one commit without the web panel:

```bash
go run ./cmd/mr-queue run --config mr-queue.yml
```

Print safe config without exposing token values:

```bash
go run ./cmd/mr-queue dry-run --config mr-queue.yml
```

## Build

```bash
go build -o dist/mr-queue ./cmd/mr-queue
```
