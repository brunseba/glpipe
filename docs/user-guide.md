# glpipe User Guide

Practical walkthroughs for common workflows. For the full command reference see the [README](../README.md).

---

## Table of contents

1. [First-time setup](#1-first-time-setup)
2. [Monitoring your projects](#2-monitoring-your-projects)
3. [Triggering pipelines](#3-triggering-pipelines)
4. [Working with manual jobs](#4-working-with-manual-jobs)
5. [Working with groups](#5-working-with-groups)
6. [Reading logs](#6-reading-logs)
7. [CI/CD usage](#7-cicd-usage)
8. [Troubleshooting](#8-troubleshooting)

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
  - id: mygroup/infra
    alias: infra
    default_branch: main

groups:
  - name: apps
    projects: [backend, frontend]
  - name: all
    projects: [backend, frontend, infra]
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

## 2. Monitoring your projects

### Global dashboard

```sh
glpipe status
```

```
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

## 3. Triggering pipelines

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

## 4. Working with manual jobs

Manual jobs are common for deploy gates — they block the pipeline until explicitly approved.

### Inspect manual jobs on a pipeline

```sh
glpipe job list -p backend -i 1234 -s manual
```

```
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
# Start watching an existing pipeline, play manual jobs as they appear
glpipe pipeline watch -p backend -i 1234 --play-manual
```

```
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

## 5. Working with groups

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
# Trigger backend + frontend simultaneously
glpipe pipeline trigger -g apps
```

```
✓ Pipeline #1235 — backend @ main  https://gitlab.com/...
✓ Pipeline #5679 — frontend @ main  https://gitlab.com/...
```

### Trigger a group and auto-play all manual jobs

```sh
glpipe pipeline trigger -g apps --play-manual
```

All pipelines are watched concurrently. Each line is prefixed with the project alias so you can follow multiple pipelines at once:

```
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

## 6. Reading logs

### Print job logs

```sh
# Get the job ID
glpipe job list -p backend -i 1234

# Print logs
glpipe job logs -p backend -i 98701
```

### Stream logs in real time

Useful to follow a long-running job (build, test suite, deploy):

```sh
glpipe job logs -p backend -i 98701 --follow
```

The command polls every 3 seconds and streams new output until the job reaches a terminal state.

---

## 7. CI/CD usage

In a CI/CD environment, inject the token via environment variable instead of storing it in a config file:

```sh
export GITLAB_TOKEN=$CI_JOB_TOKEN   # or a dedicated service account token
export GITLAB_URL=https://gitlab.mycompany.com
```

Write the config file at runtime:

```sh
mkdir -p ~/.glpipe && chmod 700 ~/.glpipe
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

Then use glpipe as usual:

```sh
glpipe pipeline trigger -g apps --play-manual
```

See [`examples/gitlab-ci.yml`](../examples/gitlab-ci.yml) for a complete GitLab CI pipeline that drives glpipe.

---

## 8. Troubleshooting

### `project "x" not found in config`

The value passed to `-p` does not match any `alias` or `id` in the config. Run `glpipe config validate` to list configured projects.

### `group "x" not found in config`

The value passed to `-g` does not match any `name` under `groups:`. Check spelling in `~/.glpipe/config.yaml`.

### `GitLab token is required`

No token is set. Either add `token:` to `~/.glpipe/config.yaml` or export `GITLAB_TOKEN`.

### `Identity verification is required in order to run CI jobs`

Your GitLab.com namespace requires phone or credit card verification to use shared runners. Go to **[gitlab.com/-/identity_verification](https://gitlab.com/-/identity_verification)**.

### `401 Unauthorized`

The token has expired or lacks the `api` scope. Create a new personal access token at **GitLab → User Settings → Access Tokens** with the `api` scope.

### Pipeline fails immediately (0 jobs, 0s duration)

Common causes on GitLab.com:

| Cause | Fix |
| ----- | --- |
| New account not verified | Complete identity verification |
| Shared runners disabled on project | Settings → CI/CD → Runners → enable shared runners |
| Namespace on free plan with expired trial | Verify account or upgrade plan |
| Project is private on unverified account | Make project public or verify account |
