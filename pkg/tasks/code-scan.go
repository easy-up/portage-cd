package tasks

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type CodeScanOptions struct {
	SemgrepFilename     string
	SemgrepRules        string
	SemgrepExperimental bool
	SemgrepSrcDir       string
	GitleaksFilename    string
	GitleaksSrc         string
	SnykCodeFilename    string
	SnykSrcDir          string
}

type CodeScanTask interface {
	SetDisplayWriter(io.Writer)
	SetOptions(CodeScanOptions)
	Run(context.Context, io.Writer) error
}

type CombinedCodeScanTask struct {
	tasks         []CodeScanTask
	displayWriter io.Writer
	opts          CodeScanOptions
}

func NewCombinedCodeScanTask(displayWriter io.Writer, opts CodeScanOptions, tasks ...CodeScanTask) *CombinedCodeScanTask {
	return &CombinedCodeScanTask{
		displayWriter: displayWriter,
		tasks:         tasks,
	}
}

func (t *CombinedCodeScanTask) Run(ctx context.Context, dstStderr io.Writer) error {
	var errs error
	displayBuf := new(bytes.Buffer)

	for _, task := range t.tasks {
		task.SetOptions(t.opts)
		task.SetDisplayWriter(displayBuf)
		err := task.Run(ctx, dstStderr)
		errs = errors.Join(errs, err)
	}

	_, err := io.Copy(t.displayWriter, displayBuf)

	return errors.Join(errs, err)
}

type SemgrepCodeScanTask struct {
	opts          CodeScanOptions
	semgrepFile   *os.File
	displayWriter io.Writer
}

func (t *SemgrepCodeScanTask) SetOptions(opts CodeScanOptions) {
	t.opts = opts
}

func (t *SemgrepCodeScanTask) SetDisplayWriter(w io.Writer) {
	t.displayWriter = w
}

func (t *SemgrepCodeScanTask) preRun() error {
	var err error
	if strings.EqualFold(t.opts.SemgrepRules, "") {
		return errors.New("Semgrep rules are required")
	}
	t.semgrepFile, err = os.Create(t.opts.SemgrepFilename)
	if err != nil {
		return err
	}

	return nil
}

func (t *SemgrepCodeScanTask) Run(ctx context.Context, dstStderr io.Writer) error {
	if err := t.preRun(); err != nil {
		return err
	}

	semgrepCmd := exec.CommandContext(ctx, "semgrep", "scan", "--json", "--config", t.opts.SemgrepRules, t.opts.SemgrepSrcDir)
	if t.opts.SemgrepExperimental {
		semgrepCmd = exec.CommandContext(ctx, "osemgrep", "scan", "--json", "--experimental", "--config", t.opts.SemgrepRules, t.opts.SemgrepSrcDir)
	}
	semgrepCmd.Stdout = t.semgrepFile
	err := StreamStderr(semgrepCmd, dstStderr, "semgrep code scan")
	if err != nil {
		// close and remove filename
		err = errors.Join(err, t.semgrepFile.Close(), os.Remove(t.opts.SemgrepFilename))
		return err
	}
	_ = t.semgrepFile.Close()

	gatecheckCmd := exec.CommandContext(ctx, "gatecheck", "ls", t.opts.SemgrepFilename)
	gatecheckCmd.Stdout = t.displayWriter

	return StreamStderr(gatecheckCmd, dstStderr, "gatecheck")
}

type GitleaksCodeScanTask struct {
	opts          CodeScanOptions
	displayWriter io.Writer
}

func (t *GitleaksCodeScanTask) SetOptions(opts CodeScanOptions) {
	t.opts = opts
}

func (t *GitleaksCodeScanTask) SetDisplayWriter(w io.Writer) {
	t.displayWriter = w
}

func (t *GitleaksCodeScanTask) Run(ctx context.Context, dstStderr io.Writer) error {
	args := []string{
		"detect",
		"--exit-code",
		"0",
		"--verbose",
		"--source",
		t.opts.GitleaksSrc,
		"--report-path",
		t.opts.GitleaksFilename,
	}

	gitleaksCmd := exec.CommandContext(ctx, "gitleaks", args...)

	err := StreamStderr(gitleaksCmd, dstStderr, "gitleaks secrets scan")
	if err != nil {
		return err
	}

	gatecheckCmd := exec.CommandContext(ctx, "gatecheck", "ls", t.opts.GitleaksFilename)
	gatecheckCmd.Stdout = t.displayWriter

	return StreamStderr(gatecheckCmd, dstStderr, "gatecheck")
}

type SnykCodeScanTask struct {
	opts          CodeScanOptions
	displayWriter io.Writer
}

func (t *SnykCodeScanTask) SetOptions(opts CodeScanOptions) {
	t.opts = opts
}

func (t *SnykCodeScanTask) SetDisplayWriter(w io.Writer) {
	t.displayWriter = w
}

func (t *SnykCodeScanTask) Run(ctx context.Context, dstStderr io.Writer) error {
	if strings.EqualFold(t.opts.SnykCodeFilename, "") {
		return errors.New("snyk report filename required")
	}
	if strings.EqualFold(t.opts.SnykSrcDir, "") {
		return errors.New("snyk src directory required")
	}

	args := []string{
		"code",
		"test",
		"-d",
		fmt.Sprintf("--sarif-file-ouput=%s", t.opts.SnykCodeFilename),
		t.opts.SnykSrcDir,
	}
	snykCodeCmd := exec.CommandContext(ctx, "snyk", args...)

	return StreamStderr(snykCodeCmd, dstStderr, "snyk code scan")
}
