# glpipe

CLI to manage GitLab pipelines across multiple repositories from a single command.

## Features

- **Dashboard** — one-line status per project (`glpipe status`)
- **Pipeline control** — list, trigger, cancel, watch
- **Job control** — list, play manual jobs, retry, stream logs
- **Project groups** — define groups in config, target them with `-g`
- **Auto-play** — trigger manual jobs automatically with `--play-manual`
- **Concurrent group watch** — `--play-manual` on a group watches all pipelines in parallel
- **Config file + env override** — `~/.glpipe/config.yaml` with `GITLAB_TOKEN` / `GITLAB_URL` override

---

## Installation

### From source

```sh
git clone https://github.com/brunseba/glpipe
cd glpipe
task install          # requires Task (taskfile.dev) and Go ≥ 1.21
```

Or without Task:

```sh
go install github.com/brunseba/glpipe@latest
```

### Pre-built binaries

Download from the [releases page](https://github.com/brunseba/glpipe/releases):

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

Creates `~/.glpipe/` (mode `0700`) and `~/.glpipe/config.yaml` (mode `0600`):

```yaml
gitlab:
  url: https://gitlab.com   # or your self-hosted instance
  token: ""                 # personal access token with api scope

projects:
  - id: mygroup/backend
    alias: backend
    default_branch: main
  - id: mygroup/frontend
    alias: frontend
    default_branch: develop
  - id: mygroup/infra
    alias: infra
    default_branch: main

groups:
  - name: apps
    projects: [backend, frontend]
  - name: all
    projects: [backend, frontend, infra]
```

### 2. Set your token

In the file or via environment variable (env takes precedence):

```sh
export GITLAB_TOKEN=glpat-xxxxxxxxxxxxxxxxxxxx
export GITLAB_URL=https://gitlab.mycompany.com   # optional
```

### 3. Validate

```sh
glpipe config validate
```

### Custom config path

```sh
glpipe --config /path/to/config.yaml status
```

---

## Commands

### `glpipe status`

One-line summary per project: last pipeline, branch, status, job counts, duration.

```
Project    Pipeline  Branch  Status   Jobs                   Duration  Created
backend    1234      main    running  running:2 manual:1     1m30s     2026-06-17 14:00
frontend   5678      main    success  success:8              4m12s     2026-06-17 13:45
```

---

### `glpipe pipeline list`

```sh
glpipe pipeline list                      # all projects
glpipe pipeline list -p backend           # one project
glpipe pipeline list -g apps              # a group
glpipe pipeline list -g apps -s running   # group, filtered by status
glpipe pipeline list -p backend -n 5      # last 5
```

| Flag | Default | Description |
| ---- | ------- | ----------- |
| `-p`, `--project` | — | Project alias or ID |
| `-g`, `--group` | — | Group name (defined in config) |
| `-n`, `--limit` | 10 | Pipelines per project |
| `-s`, `--status` | — | `running` `success` `failed` `pending` `canceled` |

---

### `glpipe pipeline trigger`

```sh
glpipe pipeline trigger -p backend                        # single project
glpipe pipeline trigger -g apps                           # all projects in group
glpipe pipeline trigger -g apps -r feature/x              # specific branch
glpipe pipeline trigger -g apps --play-manual             # trigger + auto-play manual jobs
glpipe pipeline trigger -p backend -v ENV=staging         # with variable
```

With `--play-manual` on a group, all pipelines are watched concurrently — each manual job is played as it appears, logs prefixed by project alias.

| Flag | Default | Description |
| ---- | ------- | ----------- |
| `-p`, `--project` | — | Project alias or ID |
| `-g`, `--group` | — | Group name |
| `-r`, `--ref` | `default_branch` | Branch or tag |
| `-v`, `--var` | — | `KEY=VALUE` variable (repeatable) |
| `--play-manual` | false | Watch and auto-play manual jobs until all pipelines finish |
| `--interval` | 5 | Polling interval in seconds |

---

### `glpipe pipeline watch`

```sh
glpipe pipeline watch -p backend -i 1234
glpipe pipeline watch -p backend -i 1234 --play-manual
glpipe pipeline watch -p backend -i 1234 --interval 10
```

| Flag | Default | Description |
| ---- | ------- | ----------- |
| `-p`, `--project` | required | Project alias or ID |
| `-i`, `--id` | required | Pipeline ID |
| `--interval` | 5 | Polling interval in seconds |
| `--play-manual` | false | Auto-play manual jobs |

---

### `glpipe pipeline cancel`

```sh
glpipe pipeline cancel -p backend -i 1234
```

---

### `glpipe job list`

```sh
glpipe job list -p backend -i 1234
glpipe job list -p backend -i 1234 -s manual
```

| Flag | Default | Description |
| ---- | ------- | ----------- |
| `-p`, `--project` | required | Project alias or ID |
| `-i`, `--pipeline` | required | Pipeline ID |
| `-s`, `--scope` | — | `running` `pending` `success` `failed` `canceled` `manual` (repeatable) |

---

### `glpipe job play`

```sh
glpipe job list -p backend -i 1234 -s manual   # find job ID
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
glpipe job logs -p backend -i 987        # one-shot
glpipe job logs -p backend -i 987 -f     # stream until finished
```

---

## Shell completion

```sh
glpipe completion install                                     # auto-detect shell
glpipe completion zsh  > ~/.zsh/completions/_glpipe
glpipe completion bash > ~/.bash_completion.d/glpipe
glpipe completion fish > ~/.config/fish/completions/glpipe.fish
```

For zsh add to `~/.zshrc`:

```sh
fpath=(~/.zsh/completions $fpath) && autoload -Uz compinit && compinit
```

---

## Version

```sh
glpipe version         # v0.3.0 (commit abc1234, built 2026-06-17T11:00:00Z, darwin/arm64, go1.21.0)
glpipe --version       # glpipe version v0.3.0
```

---

## Building from source

Requires [Task](https://taskfile.dev) and Go ≥ 1.21.

```sh
task build             # ./glpipe
task install           # → $GOPATH/bin
task release           # cross-compile → dist/
task sign              # keyless cosign signing (requires cosign)
task verify            # verify cosign signatures
task sbom              # generate SBOM → dist/ (requires syft)
task tag V=v1.2.3      # tag + rebuild
task test
task lint              # requires golangci-lint
task clean
```

See the [Taskfile Reference](docs/taskfile.md) for full documentation of every task.

---

## GitLab token permissions

The token needs the **`api`** scope (read + write pipelines and jobs).

> See the [User Guide](docs/user-guide.md) for step-by-step walkthroughs and real-world workflows.
