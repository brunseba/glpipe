# glpipe User Guide

Practical walkthroughs for common workflows. For the full command reference see the [README](../README.md).

---

## Table of contents

1. [First-time setup](#1-first-time-setup)
2. [Syncing projects from GitLab](#2-syncing-projects-from-gitlab)
3. [Monitoring your projects](#3-monitoring-your-projects)
4. [Triggering pipelines](#4-triggering-pipelines)
5. [Working with manual jobs](#5-working-with-manual-jobs)
6. [Working with groups](#6-working-with-groups)
7. [Reading logs](#7-reading-logs)
8. [CI/CD usage](#8-cicd-usage)
9. [Troubleshooting](#9-troubleshooting)

---

## 1. First-time setup

### Install the binary

```sh
# macOS ARM
curl -sSfL https://github.com/brunseba/glpipe/releases/latest/download/glpipe-darwin-arm64 \
  -o /usr/local/bin/glpipe && chmod +x /usr/local/bin/glpipe

# Linux x86-64
curl -sSfL https://github.com/brunseba/glpipe/releases/latest/download/glpipe-linux-amd64 \
  -o /usr/local/bin/glpipe && chmod +x /usr/local/bin/glpipe
```

### Create the config file

```sh
glpipe config init
```

Edit `~/.glpipe/config.yaml`:

```yaml
gitlab:
  url: https://gitlab.com        # or https://gitlab.mycompany.com
  token: glpat-xxxxxxxxxxxx      # personal access token, scope: api

projects:
  - id: mygroup/backend
    alias: backend
    default_branch: main
  - id: mygroup/frontend
    alias: frontend
    default_branch: main

groups:
  - name: apps
    projects: [backend, frontend]
```

> **Security** — `~/.glpipe/` is created with mode `0700` and `config.yaml` with mode `0600`. The token is only readable by your user.

### Verify connectivity

```sh
glpipe config validate
```

### Install shell completion

```sh
glpipe completion install    # auto-detects bash / zsh / fish
```

---

## 2. Syncing projects from GitLab

Instead of editing `config.yaml` by hand, `config sync-group` fetches all projects from a GitLab group and adds them automatically.

### Preview before applying

```sh
glpipe config sync-group myorg/platform --dry-run
```

```text
  + myorg/platform/api       alias: api       branch: main
  + myorg/platform/worker    alias: worker    branch: main
  skip  myorg/platform/docs (already in config)

(dry-run) would add 2 project(s)
```

### Add projects to config

```sh
glpipe config sync-group myorg/platform
```

New projects are appended to `projects:` in `~/.glpipe/config.yaml`. Projects already present (matched by ID) are skipped.

### Add projects and create a config group

```sh
glpipe config sync-group myorg/platform --create-group
```

This also appends a `groups:` entry named after the last segment of the GitLab group path (`platform`), listing the aliases of the newly added projects.

```yaml
groups:
  - name: platform
    projects: [api, worker]
```

### Custom group name

```sh
glpipe config sync-group myorg/platform --create-group --group-name infra
```

### Merge into an existing group

If a group with the same name already exists in the config, the new aliases are merged into it rather than creating a duplicate.

### Subgroups

Subgroup projects are included by default (`--include-subgroups` is on). To sync only the top-level group, use the exact top-level path — subgroup paths resolve to their own namespace.

### Flags

| Flag | Default | Description |
| ---- | ------- | ----------- |
| `--dry-run` | false | Print what would change without writing the file |
| `--create-group` | false | Also add a `groups:` entry for the synced projects |
| `--group-name` | last path segment | Name of the config group to create or merge into |

---

## 3. Monitoring your projects

### Global dashboard

```sh
glpipe status
```

```text
Project    Pipeline  Branch  Status   Jobs                       Duration  Created
backend    1234      main    running  running:2 manual:1         1m30s     2026-06-17 14:00
frontend   5678      main    success  success:6                  4m12s     2026-06-17 13:45
infra      910       main    failed   failed:1 success:3         2m05s     2026-06-17 13:30
```

### List recent pipelines

```sh
glpipe pipeline list                       # all projects, last 10 each
glpipe pipeline list -p backend -n 5       # one project, last 5
glpipe pipeline list -g apps               # a group
glpipe pipeline list -s failed             # only failed, all projects
glpipe pipeline list -g apps -s running    # running pipelines in a group
```

---

## 4. Triggering pipelines

### Single project

```sh
glpipe pipeline trigger -p backend
```

On a specific branch with pipeline variables:

```sh
glpipe pipeline trigger -p backend -r feature/my-feature \
  -v ENV=staging \
  -v DEBUG=true
```

### Trigger then watch

```sh
glpipe pipeline trigger -p backend --play-manual
```

This triggers the pipeline and immediately starts watching it, playing manual jobs automatically as they appear. The command exits when the pipeline finishes.

### Cancel a running pipeline

```sh
# Find the pipeline ID
glpipe pipeline list -p backend -s running

# Cancel it
glpipe pipeline cancel -p backend -i 1234
```

---

## 5. Working with manual jobs

Manual jobs are common for deploy gates — they block the pipeline until explicitly approved.

### Inspect manual jobs on a pipeline

```sh
glpipe job list -p backend -i 1234 -s manual
```

```text
ID     Name               Stage   Status  Duration  Runner
98701  deploy:staging     deploy  manual  -         -
98702  deploy:production  deploy  manual  -         -
```

### Play a manual job

```sh
glpipe job play -p backend -i 98701
```

### Watch and auto-play all manual jobs

```sh
glpipe pipeline watch -p backend -i 1234 --play-manual
```

```text
Watching pipeline #1234 on mygroup/backend (interval 5s, auto-play manual jobs)...
[auto-play] job #98701 (deploy:staging) triggered
[14:05:02] Pipeline #1234 — running   build:running deploy:staging:running
[auto-play] job #98702 (deploy:production) triggered
[14:09:18] Pipeline #1234 — running   deploy:production:running
Pipeline finished: success
```

### Retry a failed job

```sh
glpipe job retry -p backend -i 98701
```

---

## 6. Working with groups

Groups let you target multiple projects with a single command.

### Config

```yaml
groups:
  - name: apps
    projects: [backend, frontend]
  - name: all
    projects: [backend, frontend, infra]
```

### List pipelines for a group

```sh
glpipe pipeline list -g apps
glpipe pipeline list -g apps -s failed
```

### Trigger all projects in a group

```sh
glpipe pipeline trigger -g apps
```

```text
✓ Pipeline #1235 — backend @ main  https://gitlab.com/...
✓ Pipeline #5679 — frontend @ main  https://gitlab.com/...
```

### Trigger a group and auto-play all manual jobs

```sh
glpipe pipeline trigger -g apps --play-manual
```

All pipelines are watched concurrently. Each line is prefixed with the project alias:

```text
Watching 2 pipeline(s) with auto-play manual jobs...
[backend]  auto-play job #98701 (deploy:staging) triggered
[frontend] auto-play job #65401 (deploy:preview) triggered
[backend]  pipeline #1235 — running
[frontend] pipeline #5679 — success
[backend]  auto-play job #98702 (deploy:production) triggered
[backend]  finished: success
```

### Trigger a group on a specific branch

```sh
glpipe pipeline trigger -g apps -r release/2.0 --play-manual
```

---

## 7. Reading logs

### Print job logs

```sh
glpipe job list -p backend -i 1234
glpipe job logs -p backend -i 98701
```

### Stream logs in real time

```sh
glpipe job logs -p backend -i 98701 --follow
```

The command polls every 3 seconds and streams new output until the job reaches a terminal state.

---

## 8. CI/CD usage

In a CI/CD environment, inject the token via environment variable:

```sh
export GITLAB_TOKEN=$CI_JOB_TOKEN
export GITLAB_URL=https://gitlab.mycompany.com
```

Write the config file at runtime, then sync from the target group:

```sh
mkdir -p ~/.glpipe && chmod 700 ~/.glpipe
cat > ~/.glpipe/config.yaml <<EOF
gitlab:
  url: "${GITLAB_URL}"
  token: "${GITLAB_TOKEN}"
projects: []
groups: []
EOF
chmod 600 ~/.glpipe/config.yaml

glpipe config sync-group myorg/platform --create-group
glpipe pipeline trigger -g platform --play-manual
```

Or with a fully static config:

```sh
cat > ~/.glpipe/config.yaml <<EOF
gitlab:
  url: "${GITLAB_URL}"
  token: "${GITLAB_TOKEN}"
projects:
  - id: mygroup/backend
    alias: backend
    default_branch: main
  - id: mygroup/frontend
    alias: frontend
    default_branch: main
groups:
  - name: apps
    projects: [backend, frontend]
EOF
chmod 600 ~/.glpipe/config.yaml
```

See [`examples/gitlab-ci.yml`](../examples/gitlab-ci.yml) for a complete GitLab CI pipeline that drives glpipe.

---

## 9. Troubleshooting

### `project "x" not found in config`

The value passed to `-p` does not match any `alias` or `id` in the config. Run `glpipe config validate` to list configured projects, or use `glpipe config sync-group` to import them automatically.

### `group "x" not found in config`

The value passed to `-g` does not match any `name` under `groups:`. Check spelling in `~/.glpipe/config.yaml`, or re-run `sync-group --create-group` to regenerate the entry.

### `no projects found in group "x"`

The GitLab group path is wrong, the token lacks access to it, or the group is empty. Verify the path at `gitlab.com/<group-path>`.

### `GitLab token is required`

No token is set. Either add `token:` to `~/.glpipe/config.yaml` or export `GITLAB_TOKEN`.

### `Identity verification is required in order to run CI jobs`

Your GitLab.com namespace requires phone or credit card verification to use shared runners. Go to **[gitlab.com/-/identity_verification](https://gitlab.com/-/identity_verification)**.

### `401 Unauthorized`

The token has expired or lacks the `api` scope. Create a new personal access token at **GitLab → User Settings → Access Tokens** with the `api` scope.

### Pipeline fails immediately (0 jobs, 0s duration)

| Cause | Fix |
| ----- | --- |
| New account not verified | Complete identity verification |
| Shared runners disabled on project | Settings → CI/CD → Runners → enable shared runners |
| Namespace on free plan with expired trial | Verify account or upgrade plan |
| Project is private on unverified account | Make project public or verify account |
