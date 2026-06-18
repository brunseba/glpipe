# Taskfile Reference

All build, release, and supply-chain tasks are driven by [Task](https://taskfile.dev) (`Taskfile.yml`).

## Prerequisites

| Tool | Install | Required for |
| ---- | ------- | ------------ |
| [Go ≥ 1.21](https://go.dev/dl/) | `brew install go` | all tasks |
| [Task](https://taskfile.dev/#/installation) | `brew install go-task` | running tasks |
| [golangci-lint](https://golangci-lint.run) | `brew install golangci-lint` | `lint` |
| [cosign](https://github.com/sigstore/cosign) | `brew install cosign` | `sign`, `verify` |
| [syft](https://github.com/anchore/syft) | `brew install syft` | `sbom` |

```sh
brew install go-task cosign syft golangci-lint
```

---

## Quick reference

```sh
task          # list all available tasks
task build    # build for current platform
task test     # run tests
task release  # cross-compile all platforms → dist/
task sign     # sign dist/ binaries with cosign
task verify   # verify cosign signatures
task sbom     # generate SBOM → dist/
task tag V=v1.2.3   # tag + rebuild
task clean    # remove binary and dist/
```

---

## Tasks

### `task build`

Compiles a static binary for the current OS and architecture. Uses fingerprint caching — re-runs only if `.go` files, `go.mod`, or `go.sum` changed.

```sh
task build
./glpipe version
```

Version metadata is injected at compile time via `-ldflags`:

| Variable | Source |
| -------- | ------ |
| `Version` | `git describe --tags --always --dirty` |
| `Commit` | `git rev-parse --short HEAD` |
| `BuildDate` | `date -u +%Y-%m-%dT%H:%M:%SZ` |

---

### `task install`

Builds and copies the binary to `$GOPATH/bin` (must be in `$PATH`).

```sh
task install
glpipe version
```

---

### `task test`

```sh
task test
```

Runs `go test ./...`.

---

### `task lint`

```sh
task lint
```

Runs `golangci-lint run ./...`. Requires `golangci-lint` to be installed.

---

### `task fmt`

```sh
task fmt
```

Formats all `.go` files in-place with `gofmt`.

---

### `task tidy`

```sh
task tidy
```

Runs `go mod tidy` then `go mod verify` to ensure the module graph is clean and the module cache is consistent.

---

### `task tag`

Creates an annotated git tag and immediately rebuilds the binary with that version baked in.

```sh
task tag V=v1.2.3
```

**Preconditions checked before tagging:**

- `V` is provided
- `V` matches semver (`vMAJOR.MINOR.PATCH`)
- Tag does not already exist
- Working tree is clean (no uncommitted changes)

After tagging, push the tag to trigger the GitHub Actions release pipeline:

```sh
git push origin v1.2.3
```

---

### `task release`

Cross-compiles for all supported platforms and writes binaries to `dist/`.

```sh
task release
ls dist/
# glpipe-linux-amd64
# glpipe-linux-arm64
# glpipe-darwin-amd64
# glpipe-darwin-arm64
# glpipe-windows-amd64.exe
```

All binaries are statically linked (`CGO_ENABLED=0`) and stripped (`-s -w`).

---

### `task sign`

Signs every binary in `dist/` using **cosign keyless signing** (Sigstore). No private key required — your identity is proved via OIDC (opens a browser on first run to authenticate with your GitHub account).

```sh
task release   # build first
task sign      # sign all binaries
```

Produces a `.bundle` file alongside each binary:

```
dist/glpipe-linux-amd64
dist/glpipe-linux-amd64.bundle   ← certificate + signature + timestamp
dist/glpipe-darwin-arm64
dist/glpipe-darwin-arm64.bundle
...
```

The bundle contains everything needed for offline verification (no network call required at verify time).

---

### `task verify`

Verifies the cosign signatures of all signed binaries in `dist/`.

```sh
task verify
```

```
  OK    dist/glpipe-darwin-amd64
  OK    dist/glpipe-darwin-arm64
  OK    dist/glpipe-linux-amd64
  OK    dist/glpipe-linux-arm64
  OK    dist/glpipe-windows-amd64.exe

Result: 5 verified, 0 failed
```

By default verifies against your personal GitHub identity (local bundles from `task sign`). Override for CI-produced bundles:

```sh
# Verify bundles signed by GitHub Actions
task verify \
  IDENTITY="https://github.com/brunseba/glpipe/.github/workflows/build.yml@refs/tags/v0.4.0" \
  ISSUER="https://token.actions.githubusercontent.com"
```

| Variable | Default | Description |
| -------- | ------- | ----------- |
| `IDENTITY` | `veille.informatique@lepervier.org` | Expected certificate identity |
| `ISSUER` | `https://github.com/login/oauth` | Expected OIDC issuer |

---

### `task sbom`

Generates a Software Bill of Materials (SBOM) in two industry-standard formats.

```sh
task sbom
```

```
dist/sbom.cyclonedx.json   ← CycloneDX 1.6
dist/sbom.spdx.json        ← SPDX
```

Both files include the binary name, version (from `git describe`), and all transitive Go module dependencies. Requires `syft`.

---

### `task clean`

Removes the local binary and the `dist/` folder.

```sh
task clean
```

---

## Release workflow

Full local release (build → sign → SBOM → publish):

```sh
task tag V=v1.2.3          # tag + local binary
task release               # cross-compile all platforms
task sign                  # keyless sign all binaries
task verify                # confirm signatures
task sbom                  # generate SBOM
git push origin v1.2.3     # triggers GitHub Actions (CI build + keyless sign + gh release)
```

GitHub Actions then independently builds, signs (with the workflow identity), and publishes the release with binaries and `.bundle` files attached.
