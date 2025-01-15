package main

import (
	"fmt"
	"log/slog"
	"os"
	"portage/cmd/portage/cli/v0"
	"runtime"
	"time"

	"github.com/lmittmann/tint"
)

const (
	exitOK             = 0
	exitCommandFailure = 1
)

var (
	cliVersion     = "[Not Provided]"
	buildDate      = "[Not Provided]"
	gitCommit      = "[Not Provided]"
	gitDescription = "[Not Provided]"
)

func main() {
	os.Exit(runCLIv0())
}

func runCLIv0() int {
	lvler := &slog.LevelVar{}
	lvler.Set(slog.LevelInfo)
	// Set up custom structured logging with colorized output
	slog.SetDefault(slog.New(tint.NewHandler(os.Stderr, &tint.Options{
		Level:      lvler,
		TimeFormat: time.TimeOnly,
	})))
	cli.AppLogLever = lvler
	cli.AppMetadata = cli.ApplicationMetadata{
		CLIVersion:     cliVersion,
		GitCommit:      gitCommit,
		BuildDate:      buildDate,
		GitDescription: gitDescription,
		Platform:       fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		GoVersion:      runtime.Version(),
		Compiler:       runtime.Compiler,
	}

	cmd := cli.NewPortageCommand()
	start := time.Now()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "------------")
		slog.Error(fmt.Sprintf("%v", err), "elapsed", time.Since(start))
		return exitCommandFailure
	}
	fmt.Fprintln(os.Stderr, "------------")
	slog.Info("done", "elapsed", time.Since(start))
	return exitOK
}
