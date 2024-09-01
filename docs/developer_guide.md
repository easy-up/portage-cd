# Developer Guide

TODO: Project info, goals, etc.

## [Getting Started](./getting_started.md)

## Project Layout

TODO: Add the philosophy behind the project layout

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
