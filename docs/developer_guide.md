# Developer Guide

Portage CD is a Go CLI that orchestrates a fixed, opinionated sequence of security-analysis tools (gitleaks, semgrep, syft, grype, clamav) around a container build, then bundles their artifacts via [gatecheck](https://github.com/easy-up/gatecheck) for validation and optional webhook delivery. The design goal is to give users a pre-assembled, hardened pipeline that runs identically from a local shell, a CI runner, or a container — without recreating shell-argument glue in every CI tool.

## [Getting Started](./getting_started.md)

## Project Layout

The code is organized around two layers:

- **`pkg/shell/`** — thin, opinionated wrappers around each external tool. Each tool gets its own file (e.g. `grype.go`, `syft.go`, `docker.go`) with functions that expose *only the flags portage actually uses*. This constrains the surface area — callers can't sneak arbitrary shell into a command.
- **`pkg/pipelines/`** — pipeline orchestration. Each pipeline file (`image-scan.go`, `code-scan.go`, etc.) composes shell functions into an ordered, concurrency-aware sequence. `AsyncTask` handles inter-stage dependencies (e.g. grype waits on syft).

`cmd/portage/` is the Cobra CLI entry point; it translates flags into the `pkg/pipelines/config.go` struct and dispatches to the appropriate pipeline.

### Shell

The Shell package (`pkg/shell`) is a library of commands and utilities used in portage.
The standard library way to execute shell commands is by using the `os/exec` package which has a lot of features and
flexibility.
In our case, we want to restrict the ability to arbitrarily execute shell commands by carefully selecting a sub-set of 
features for each command.

For example, if you look at the Syft CLI reference, you'll see dozens of commands and configuration options.
This is all controlled by flag parsing the string of the command.
This is an opinionated security pipeline, so we don't need all the features Syft provides.
The user shouldn't care that we're using Syft to generate an SBOM which is then scanned by Grype for vulnerabilities.
The idea of Portage CD is that it's all abstracted to the Security Analysis pipeline.

In the Shell package, all necessary commands will be abstracted into a native Go object.
Only the used features for the given command will be written into this package.

The shell.Executable wraps the exec.Cmd struct and adds some convenient methods for building a command.

```shell
syft version -o json
```

How to execute regular std lib commands with `exec.Cmd`

```go
cmd := exec.Command("syft", "version", "-o","json")
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr
// some other options
err := cmd.Run()
```

There's also additional logic with the `os.exec` standard library command.
Since portage is built around executing external binaries, there is an internal library called the `pkg/shell`
used to abstract a lot of the complexities involved with handling async patterns, possible interrupts, and parameters.

Commands can be represented as functions.

```go
func SyftVersion(options ...OptionFunc) error {
	o := newOptions(options...)
	cmd := exec.Command("syft", "version")
	return run(cmd, o)
}
```

The `OptionFunc` variadic parameter allows the caller to modify the behavior of the command with an arbitrary
number of `OptionFunc`(s).

`newOptions` generates the default `Options` structure and then applies all of passed in functions.
The `o` variable can now be used to apply parameters to the command before execution. 

Returning the `run` function handles off the execution phase of the command to another function which bootstraps
a lot of useful functionality without needing to write supported code for each new command.

For example, if you only want to output what the command would run but not actually run the command, 
```go
dryRun := false
SyftVersion(WithStdout(os.Stdout), WithDryRun(dryRun))
```

This would log the final output command without executing.

The motivation behind this architecture is to simply the Methods for all sub-commands on an executable.

Implementing a new sub command is trivial, just write a new function with the same pattern

```go
func SyftHelp(options ...OptionFunc) error {
	o := newOptions(options...)
	cmd := exec.Command("syft", "--help")
	return run(cmd, o)
}
```

If we wanted to build an optionFunc for version to optionally write JSON instead of plain text, it would go in the
`pkg/shell/shell.go` function.

Since there aren't many commands, they all share the same configuration object `Options`.

```go
func WithJSONOutput(enabled bool) OptionFunc {
	return func(o *Options) {
		o.JSONOutput = true
	}
}
```

Now, the version function can reference this field and change the shell command

```go
func SyftVersion(options ...OptionFunc) error {
	o := newOptions(options...)
	cmd := exec.Command("syft", "version")
  if o.JSONOutput {
    cmd = exec.Command("syft", "version", "-o", "json")
  }
	return run(cmd, o)
}
```

See `pkg/shell/docker.go` for a more complex example of a command with a lot of parameters.

### Pipelines

Pipelines live in `pkg/pipelines/` — one file per pipeline (e.g. `image-scan.go`, `code-scan.go`, `image-build.go`, `deploy.go`). Each pipeline constructs an ordered sequence of `AsyncTask`s (see Concurrency below), manages the artifact file handles, and reports exit status.

Pipelines are composed into higher-level commands in `cmd/portage/cli/v0/pipelines.go`. The top-level `portage run all` is just `code-scan` → `image-build` → `image-scan` → `image-publish` → `deploy` in sequence, gated by each stage's `enabled` flag.

## Concepts

### Concurrency

An AsyncTask is used to simplify concurrency by providing a few convenient methods.

`StreamTo`: allows the caller to block and read the stderr log while the command is running.
`Close`: Closes the internal pipe writer, signaling to the pipe reader that it is done writing data.
`Wait`: can be called multiple times, it blocks until `Close()` is called on the task. Under the hood it uses a ctx.

The general idea is that a "task" can be used in a goroutine in the background until the command is complete.
This strategy enables a bunch of jobs to be kicked off in goroutines and stream stderr output in any order.

The pattern used in the image-scan and code-scan pipelines uses methods with parameters than defines task to task
dependencies.
For example, grype cannot be run until syft runs and generates the SBOM, so the function for the grypeJob has a
syftTask parameter so it can wait for the syft command to finish.

For situations where the output of one command can be piped into another, there is a stdoutBuf field on AsyncTask
that can be used to temporarily store the output in memory until the command is complete.

Originally, io.Pipes were used here but it makes the logic of the command very complicated and not very readable even
though it would technically be more efficient than storing in memory.

The Async Task also wraps the stderr output with a label and timing capability, so the user can see how long each task
takes to complete.

## Release Process

### Creating a New Release

To create a new release of portage-cd and publish a new container image:

1. **Navigate to the portage-cd repository**
   ```bash
   cd portage-cd
   ```

2. **Ensure you're on the correct branch and up to date**
   ```bash
   git status
   git branch
   # If on main:
   git pull origin main
   # If on belay_main (current active branch):
   git pull origin belay_main
   ```

3. **Handle any uncommitted changes**
   If you have uncommitted changes:
   - **To include them in the release**: Commit the changes first
     ```bash
     git add .
     git commit -m "Description of changes"
     ```
   - **To exclude them**: Stash the changes
     ```bash
     git stash push -m "WIP changes before release"
     ```

4. **Determine the next version number**
   ```bash
   git tag --sort=-version:refname | head -5
   ```
   Follow semantic versioning. Bump patch for bug fixes, minor for features, major for breaking changes.

5. **Create and push the new tag**
   ```bash
   # Replace with the chosen version
   git tag vX.Y.Z
   git push origin vX.Y.Z
   ```

6. **Verify the release**
   - The GitHub Actions workflow will automatically trigger
   - Monitor the release at: `https://github.com/easy-up/portage-cd/actions`
   - The workflow will:
     - Build the Go binary using GoReleaser
     - Build the Docker container (including latest gatecheck changes)
     - Create a GitHub release with artifacts
     - Publish the container image

### Release Dependencies

Portage-cd includes gatecheck as a build-time dependency, **pinned to an exact commit** in the
`Dockerfile`:

```dockerfile
ARG GATECHECK_VERSION=belay_main            # source branch (documentation only)
ARG GATECHECK_VERSION_COMMIT=<40-char sha>  # what is actually built
```

- The build fetches that **commit**, not the branch tip. Pushing to gatecheck's `belay_main`
  does **not** reach a portage-cd image on its own.
- To pick up gatecheck changes you must **bump `GATECHECK_VERSION_COMMIT`** and commit that to
  portage-cd. The `Dockerfile` is in the delivery workflow's path filter, so that commit is what
  triggers the rebuild.
- Pinning is deliberate: image contents must be reproducible from a portage-cd commit. Tracking a
  moving branch would change what ships without any corresponding change here.

Unlike grype/syft/gitleaks/oras, which are cloned by immutable **tag**, gatecheck is pinned by
commit on a moving branch. The fetch is by commit precisely so the pin stays valid when that branch
advances — an earlier `git clone --depth=1 --single-branch` of the branch contained only the tip, so
`git checkout <pin>` failed with exit 128 as soon as `belay_main` moved ahead of the pin. That broke
four of five consecutive image builds in Sept 2026, and because a failed publish is silent, the
`belay-main-latest` tag simply stopped advancing without anyone noticing.

### Image Publishing Chain

Two images are published, from two repositories, and **only the first is automatic**:

| Image | Built by | Triggered by |
|---|---|---|
| `ghcr.io/easy-up/portage:belay-main-latest` | portage-cd `delivery.yaml` | push to `belay_main` touching `Dockerfile`, `pkg/**`, `cmd/**`, `go.*` |
| `ghcr.io/easy-up/portage-action:belay-main-latest` | portage-cd-actions `delivery.yml` | push touching **its own** `Dockerfile`/`entrypoint.sh`/`PORTAGE_VERSION`, `workflow_dispatch`, or a `portage-updated` repository dispatch |

The action image is built `FROM` the portage image, but **nothing notifies it when portage
publishes**. After shipping a portage change that must reach GitHub Actions users, run the
portage-cd-actions delivery workflow (`workflow_dispatch`) or the action image will keep wrapping an
older Portage binary. Verify what actually shipped with:

```shell
docker run --rm --entrypoint sh ghcr.io/easy-up/portage-action:belay-main-latest \
  -c 'portage version; portage config generate-table | grep -i buildgroupid'
```

### Branch Strategy

This repository follows a dual-branch strategy to support different use cases:

- **`main` branch**: For open source users who want to use Portage-CD without the Belay platform integration
- **`belay_main` branch**: For users integrating with Belay, a SaaS platform that provides additional security pipeline capabilities and deployment automation

The `belay_main` branch includes additional features and configurations specifically designed to work with the Belay platform at https://belay.holomuatech.com. Choose the appropriate branch based on your deployment needs.

### Documentation

## Too Long; Might Read (TL;MR)

A collection of thoughts around design decisions made in Portage CD, mostly ramblings that some people may or may 
not find useful.

### Why CI/CD Flexible Configuration is Painful

In a traditional CI/CD environment, you would have to parse strings to build the exact command you want to execute.

Local Shell:
```bash
syft version
```

GitLab CI/CD Configuration let's use declare the execution environment by providing an image name
```yaml
syft-version:
  stage: scan
  image: anchore/syft:latest
  script:
    - syft version
```

What typically happens is configuration creep.
If you need to print the version information in JSON, (one of the many command options), you would have to provide 
multiple options in GitLab, only changing the script block, hiding each on behind an env variable

```yaml
.syft:
  stage: scan
  image: anchore/syft:latest

syft-version:text:
  extends: .syft
  script:
    - syft version
  rules:
    - if: $SYFT_VERSION_JSON != "true"

syft-version:json:
  extends: .syft
  script:
    - syft version -o json
  rules:
    - if: $SYFT_VERSION_JSON == "true"

```

The complexity increase exponentially in a GitLab CI/CD file for each configuration option you wish to support.
