# glpipe

CLI to manage GitLab pipelines across multiple repositories from a single command.

## Features

- **Dashboard** — one-line status per project (`glpipe status`)
- **Pipeline control** — list, trigger, cancel, watch
- **Job control** — list, play manual jobs, retry, stream logs
- **Auto-play** — watch a pipeline and trigger manual jobs automatically (`--play-manual`)
- **Multi-project** — all commands work across every configured repository at once
- **Config file + env override** — `~/.glpipe.yaml` with `GITLAB_TOKEN` / `GITLAB_URL` override

---

## Installation

### From source

```sh
git clone https://github.com/sebrun/glpipe
cd glpipe
task install          # requires Task (taskfile.dev) and Go ≥ 1.21
```

Or without Task:

```sh
go install github.com/sebrun/glpipe@latest
```

### Pre-built binaries

Download from the [releases page](https://github.com/sebrun/glpipe/releases), then:

```sh
chmod +x glpipe-linux-amd64
mv glpipe-linux-amd64 /usr/local/bin/glpipe
```

---

## Configuration

### 1. Create the config file

```sh
glpipe config init
```

This creates `~/.glpipe/` (mode `0700`) and writes `~/.glpipe/config.yaml` (mode `0600`):

```yaml
gitlab:
  url: https://gitlab.com   # or your self-hosted instance
  token: ""                 # personal access token with api scope

projects:
  - id: mygroup/myrepo
    alias: backend
    default_branch: main
  - id: mygroup/frontend
    alias: frontend
    default_branch: develop
```

The config directory and file are readable only by the current user, so the token is not exposed to other users on the same machine.

### 2. Set your token

Either in the file:

```yaml
gitlab:
  token: glpat-xxxxxxxxxxxxxxxxxxxx
```

Or via environment variable (takes precedence, useful in CI):

```sh
export GITLAB_TOKEN=glpat-xxxxxxxxxxxxxxxxxxxx
```

`GITLAB_URL` overrides `gitlab.url` the same way.

### 3. Validate

```sh
glpipe config validate
```

### Using a custom config file

```sh
glpipe --config /path/to/config.yaml status
```

---

## Commands

### `glpipe status`

One-line summary per project: last pipeline ID, branch, status, job counts, duration.

```sh
glpipe status
```

```
Project    Pipeline  Branch   Status   Jobs                   Duration  Created
backend    1234      main     running  running:2 manual:1     1m30s     2026-06-17 14:00
frontend   5678      develop  success  success:8              4m12s     2026-06-17 13:45
```

---

### `glpipe pipeline list`

```sh
# All projects (last 10 pipelines each)
glpipe pipeline list

# One project
glpipe pipeline list -p backend

# Only running pipelines, up to 5
glpipe pipeline list -p backend -s running -n 5
```

| Flag | Default | Description |
|------|---------|-------------|
| `-p`, `--project` | all | Project alias or ID |
| `-n`, `--limit` | 10 | Pipelines per project |
| `-s`, `--status` | — | Filter: `running` `success` `failed` `pending` `canceled` |

---

### `glpipe pipeline trigger`

```sh
# Trigger on default branch
glpipe pipeline trigger -p backend

# Specific branch with variables
glpipe pipeline trigger -p backend -r feature/my-branch -v ENV=staging -v DEBUG=true
```

| Flag | Default | Description |
|------|---------|-------------|
| `-p`, `--project` | required | Project alias or ID |
| `-r`, `--ref` | `default_branch` | Branch or tag |
| `-v`, `--var` | — | `KEY=VALUE` variable (repeatable) |

---

### `glpipe pipeline watch`

Polls the pipeline every N seconds and prints a live status line. Exits automatically when the pipeline reaches a terminal state.

```sh
glpipe pipeline watch -p backend -i 1234

# Automatically play manual jobs as they appear
glpipe pipeline watch -p backend -i 1234 --play-manual

# Faster polling
glpipe pipeline watch -p backend -i 1234 --interval 3
```

```
Watching pipeline #1234 on mygroup/myrepo (interval 5s, auto-play manual jobs)...
[auto-play] job #987 (deploy-staging) triggered
[14:05:02] Pipeline #1234 — running   build:running  deploy-staging:running
Pipeline finished: success
```

| Flag | Default | Description |
|------|---------|-------------|
| `-p`, `--project` | required | Project alias or ID |
| `-i`, `--id` | required | Pipeline ID |
| `--interval` | 5 | Polling interval in seconds |
| `--play-manual` | false | Trigger manual jobs automatically |

---

### `glpipe pipeline cancel`

```sh
glpipe pipeline cancel -p backend -i 1234
```

---

### `glpipe job list`

```sh
# All jobs for a pipeline
glpipe job list -p backend -i 1234

# Only manual jobs
glpipe job list -p backend -i 1234 -s manual
```

| Flag | Default | Description |
|------|---------|-------------|
| `-p`, `--project` | required | Project alias or ID |
| `-i`, `--pipeline` | required | Pipeline ID |
| `-s`, `--scope` | — | Filter: `running` `pending` `success` `failed` `canceled` `manual` (repeatable) |

---

### `glpipe job play`

Trigger a job that is in `manual` state.

```sh
# Find the job ID first
glpipe job list -p backend -i 1234 -s manual

# Then play it
glpipe job play -p backend -i 987
```

---

### `glpipe job retry`

```sh
glpipe job retry -p backend -i 987
```

---

### `glpipe job logs`

```sh
# Print all logs (job must be finished or running)
glpipe job logs -p backend -i 987

# Stream until job finishes
glpipe job logs -p backend -i 987 -f
```

| Flag | Default | Description |
|------|---------|-------------|
| `-p`, `--project` | required | Project alias or ID |
| `-i`, `--id` | required | Job ID |
| `-f`, `--follow` | false | Stream logs until job finishes |

---

## Shell completion

```sh
# Auto-detect shell and install
glpipe completion install

# Or manually
glpipe completion zsh > ~/.zsh/completions/_glpipe
glpipe completion bash > ~/.bash_completion.d/glpipe
glpipe completion fish > ~/.config/fish/completions/glpipe.fish
```

For zsh, add to `~/.zshrc` if not already present:

```sh
fpath=(~/.zsh/completions $fpath) && autoload -Uz compinit && compinit
```

---

## Version

```sh
glpipe version
# dev (commit abc1234, built 2026-06-17T11:00:00Z, darwin/arm64, go1.21.0)

glpipe --version
# glpipe version v1.2.0
```

Version, commit, and build date are injected at compile time via `-ldflags`. When built with `go run .` or without tags the values fall back to `dev` / `none` / `unknown`.

---

## Building from source

Requires [Task](https://taskfile.dev) and Go ≥ 1.21.

```sh
task build           # local binary → ./glpipe
task install         # install to $GOPATH/bin
task release         # cross-compile → dist/
task test
task lint            # requires golangci-lint
task clean
```

Version metadata injected automatically by Task:

| Variable | Source |
| -------- | ------ |
| `Version` | `git describe --tags --always --dirty` |
| `Commit` | `git rev-parse --short HEAD` |
| `BuildDate` | `date -u +%Y-%m-%dT%H:%M:%SZ` |

---

## GitLab token permissions

The token needs the **api** scope (read + write pipelines and jobs).  
A project-scoped token with `api` is sufficient if all projects are in the same group.
