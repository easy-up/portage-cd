package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
)

var (
	ErrInterupt      = errors.New("command interupted (SIGINT)")
	ErrInteruptFail  = errors.New("command interupted (SIGINT) requested but failed")
	ErrNotStarted    = errors.New("command canceled before run")
	ErrBadParameters = errors.New("command has invalid parameters")
)

const exitCodeOther = 300

type ErrCommand struct {
	Err         error
	ExitCode    int
	CommandName string
}

func (e *ErrCommand) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[shell:%s] %v", e.CommandName, e.Err)
	}
	return ""
}

func NewCommandError(err error, cmdName string, exitCode int) *ErrCommand {
	return &ErrCommand{
		Err:         err,
		ExitCode:    exitCode,
		CommandName: cmdName,
	}
}

// Command is any function that accepts optionFuncs and returns an exit code
//
// Most commands can take advantage of the run function which automatically
// parses the options to configure the exec.Cmd
//
// It also handles early termination of the command with a context and logging
type Command func(...OptionFunc) error

// Options are flexible parameters for any command
type Options struct {
	dryRunEnabled      bool
	stdin              io.Reader
	stdout             io.Writer
	stderr             io.Writer
	errorOnlyStderr    io.Writer
	errorOnlyStderrBuf *bytes.Buffer
	errorOnly          bool
	ctx                context.Context
	failTriggerFunc    func()
	waitFunc           func() error
	tarFilename        string
	dockerAlias        DockerAlias
	imageTag           string
	reportType         string
	bundleTag          string
	targetFilename     string
	configFilename     string
	metadata           struct {
		commandName string
	}

	logger *slog.Logger

	imageBuildOptions ImageBuildOptions

	gatecheck struct {
		bundleFilename string
		targetFile     string
		label          string
		tags           []string
	}

	semgrep struct {
		rules           string
		experimental    bool
		targetDirectory string
	}

	gitleaks struct {
		targetDirectory string
		reportPath      string
	}

	listTargetFilename string
}

// apply should be called before the exec.Cmd is run
func (o *Options) apply(options ...OptionFunc) {
	for _, optionFunc := range options {
		optionFunc(o)
	}
}

// newOptions is used to generate an Options struct and automatically apply optionFuncs
func newOptions(options ...OptionFunc) *Options {
	o := new(Options)
	o.failTriggerFunc = func() {}
	o.waitFunc = func() error {
		return nil
	}
	o.logger = slog.Default()
	o.apply(options...)
	return o
}

// OptionFunc are used to set option values in a flexible way
type OptionFunc func(o *Options)

// WithDryRun sets the dryRunEnabled option which will print the command that would run and exitOK
func WithDryRun(enabled bool) OptionFunc {
	return func(o *Options) {
		o.dryRunEnabled = enabled
	}
}

// WithErrorOnly buffers stderr unless there is a non-zero exit code.
//
// If there is a non-zero exit, the error buffer will dump to stderr
func WithErrorOnly(stderr io.Writer) OptionFunc {
	return func(o *Options) {
		o.errorOnly = true
		o.errorOnlyStderr = stderr
	}
}

// WithIO sets input and output for a command
func WithIO(stdin io.Reader, stdout io.Writer, stderr io.Writer) OptionFunc {
	return func(o *Options) {
		o.stdin = stdin
		o.stdout = stdout
		o.stderr = stderr
	}
}

// WithStdin only sets STDIN for the command
func WithStdin(r io.Reader) OptionFunc {
	return func(o *Options) {
		o.stdin = r
	}
}

// WithStdout only sets STDOUT for the command
func WithStdout(w io.Writer) OptionFunc {
	return func(o *Options) {
		o.stdout = w
	}
}

// WithStderr only sets STDERR for the command
func WithStderr(w io.Writer) OptionFunc {
	return func(o *Options) {
		o.stderr = w
	}
}

// WithCtx enables a command to be interruptable
func WithCtx(ctx context.Context) OptionFunc {
	return func(o *Options) {
		o.ctx = ctx
	}
}

// WithGitleaks specific parameters
func WithGitleaks(targetDirectory string, reportPath string) OptionFunc {
	return func(o *Options) {
		o.gitleaks.targetDirectory = targetDirectory
		o.gitleaks.reportPath = reportPath
	}
}

// WithFailTrigger will call the provided function for non-zero exit
//
// This can be useful if running multiple commands async and you want
// to early termination with a context cancel should either command fail
func WithFailTrigger(f func()) OptionFunc {
	return func(o *Options) {
		o.failTriggerFunc = f
	}
}

// WithDockerAlias can be used to configure an alternative docker compatible CLI
//
// For example, `docker build` and `podman build` can be used interchangably
func WithDockerAlias(a DockerAlias) OptionFunc {
	return func(o *Options) {
		o.dockerAlias = a
	}
}

// WithImageTag can be used for multiple commands to define a target image as a parameter
//
// This will include the full image and tag for example `alpine:latest`
func WithImageTag(imageTag string) OptionFunc {
	return func(o *Options) {
		o.imageTag = imageTag
	}
}

// WithImage can be used for multiple commands to define a archive/tar filename
//
// should include the full filename including extension
func WithTarFilename(filename string) OptionFunc {
	return func(o *Options) {
		o.tarFilename = filename
	}
}

// WithReportType used in Gatecheck List to define the input type for piped content
func WithReportType(reportType string) OptionFunc {
	return func(o *Options) {
		o.reportType = reportType
	}
}

// WithBundleImage gatecheck bundle specific parameters
func WithBundleImage(bundleTag string, bundleFilename string) OptionFunc {
	return func(o *Options) {
		o.bundleTag = bundleTag
		o.gatecheck.bundleFilename = bundleFilename
	}
}

// WithBundleFile gatecheck bundle specific parameters
func WithBundleFile(bundleFilename string, targetFilename string) OptionFunc {
	return func(o *Options) {
		o.gatecheck.bundleFilename = bundleFilename
		o.gatecheck.targetFile = targetFilename
	}
}

// WithBundleTags sets the label and optional tags for bundle operations
func WithBundleTags(tags ...string) OptionFunc {
	return func(o *Options) {
		o.gatecheck.tags = tags
	}
}

// WithTargetFile generic parameter that needs a specific filename
func WithTargetFile(filename string) OptionFunc {
	return func(o *Options) {
		o.targetFilename = filename
	}
}

func WithConfigFile(filename string) OptionFunc {
	return func(o *Options) {
		o.configFilename = filename
	}
}

// WithSemgrep specific parameters
func WithSemgrep(rules string, experimental bool, targetDirectory string) OptionFunc {
	return func(o *Options) {
		o.semgrep.rules = rules
		o.semgrep.experimental = experimental
		o.semgrep.targetDirectory = targetDirectory
	}
}

// WithListTarget gatecheck list a specific filen by name
func WithListTarget(filename string) OptionFunc {
	return func(o *Options) {
		o.listTargetFilename = filename
	}
}

// WithBuildImageOptions apply docker build options before command execution
func WithBuildImageOptions(options ImageBuildOptions) OptionFunc {
	return func(o *Options) {
		o.imageBuildOptions = options
	}
}

// WithLogger use a specific logger for command debugging
func WithLogger(logger *slog.Logger) OptionFunc {
	return func(o *Options) {
		o.logger = logger
	}
}

// WithWaitFunc at runtime, this function will be called, if error, it will auto fail the command
//
// The default behavoir is to return nil immediately
func WithWaitFunc(f func() error) OptionFunc {
	return func(o *Options) {
		o.waitFunc = f
	}
}

// gracefulExit handles errors from run
//
// 1. If commandError is nil, just return ExitOK
// 2. Trigger the failTrigger function
// 3. Dump stderr from stderrBuf if the command was configured to only send stderr on command fail
// 4. Try to get the command native error code
// 5. If nothing else works, log debugging information and return exitUnknown
func gracefulExit(commandError error, o *Options) error {
	if commandError == nil {
		return nil
	}

	o.failTriggerFunc()

	var exitCodeError *exec.ExitError
	var execError *exec.Error

	switch {
	case o.errorOnly:
		o.logger.Warn("a error occurred while running a command. Dumping logs to stderr")
		_, err := io.Copy(o.errorOnlyStderr, o.errorOnlyStderrBuf)
		if err != nil {
			commandError = errors.Join(commandError, fmt.Errorf("cannot dump logs. reason: %v", err))
		}
	// this happens on a regular exit from a command that failed with something other than 0
	case errors.As(commandError, &exitCodeError):
		return NewCommandError(exitCodeError, o.metadata.commandName, exitCodeError.ExitCode())
	case errors.As(commandError, &execError):
		return NewCommandError(execError, o.metadata.commandName, exitCodeOther)
	}

	return NewCommandError(commandError, o.metadata.commandName, exitCodeOther)
}

// run handles the execution of the command
//
// context will be set to background if not provided in the o.ctx
// this enables the command to be terminated before completion
// if ctx fires done.
//
// Setting the dry run option will always return ExitOK
func run(cmd *exec.Cmd, o *Options) error {
	o.logger.Info("shell exec", "dry_run", o.dryRunEnabled, "command", cmd.String(), "errors_only", o.errorOnly)

	o.metadata.commandName = cmd.Args[0]

	o.errorOnlyStderrBuf = new(bytes.Buffer)
	if o.errorOnly {
		cmd.Stderr = o.errorOnlyStderrBuf
	}

	err := o.waitFunc() // will block if defined by caller
	if err != nil {
		return gracefulExit(fmt.Errorf("%w: %v", ErrNotStarted, err), o)
	}

	cmd.Stdin = o.stdin
	cmd.Stdout = o.stdout
	if !o.errorOnly {
		cmd.Stderr = o.stderr
	}

	if o.dryRunEnabled {
		return gracefulExit(nil, o)
	}

	if err := cmd.Start(); err != nil {
		return gracefulExit(err, o)
	}

	if o.ctx == nil {
		o.ctx = context.Background()
	}

	var runError error
	doneChan := make(chan struct{}, 1)
	go func() {
		runError = cmd.Wait()
		doneChan <- struct{}{}
	}()

	// Either context will cancel or the command will finish before
	// capture the exit code
	select {
	case <-o.ctx.Done():
		o.logger.Warn("command canceled", "command", cmd.String())
		if err := cmd.Process.Kill(); err != nil {
			err = fmt.Errorf("%w: %w", ErrInteruptFail, err)
			return gracefulExit(err, o)
		}
		return gracefulExit(ErrInterupt, o)
	case <-doneChan:
		return gracefulExit(runError, o)
	}
}
