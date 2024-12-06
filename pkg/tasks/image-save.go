package tasks

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
)

type ImageSaveTask interface {
	Run(context.Context, io.Writer) error
}

func NewImageSaveTask(cliInterface string, opts ...taskOptionFunc) ImageSaveTask {
	o := newDefaultTaskOpts()
	for _, optFunc := range opts {
		optFunc(o)
	}
	task := new(GenericImageSaveTask)
	task.opts = o
	switch strings.ToLower(strings.TrimSpace(cliInterface)) {
	case "docker":
		task.cmdName = "docker"
		return task
	case "podman":
		task.cmdName = "podman"
		return task
	default:
		panic("Unsupported image save cli interface, must be docker or podman")
	}
}

type GenericImageSaveTask struct {
	opts    *taskOptions
	cmdName string
}

func isValidImageName(name string) bool {
	// Only allow characters typically used in Docker image names:
	// - Lowercase letters, numbers
	// - Allowed special chars: period, underscore, hyphen
	// - Forward slash for image paths (e.g., "repository/image")
	// - Colon for tags (e.g., "image:tag")
	for _, char := range name {
		if !((char >= 'a' && char <= 'z') ||
			(char >= '0' && char <= '9') ||
			char == '.' || char == '_' || char == '-' ||
			char == '/' || char == ':') {
			return false
		}
	}
	return len(name) > 0
}

func (t *GenericImageSaveTask) Run(ctx context.Context, stderr io.Writer) error {
	if strings.EqualFold(t.opts.ImageName, "") {
		return errors.New("image name is required")
	}

	if !isValidImageName(t.opts.ImageName) {
		return errors.New("invalid image name: must contain only lowercase letters, numbers, and allowed special characters (./_-/:)")
	}

	// let the open file command handle any invalid filename errors
	// run this first just incase to fail early if something goes wrong
	imageSaveFile, err := os.OpenFile(t.opts.ImageTarFilename, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}

	err = t.runPullImage(ctx, stderr)
	if err != nil {
		return err
	}

	smWriter := NewSizeMonitorWriter("image save", t.opts.ImageTarFilename, stderr)
	monitorCtx, monitorCancel := context.WithCancel(ctx)
	go smWriter.Start(monitorCtx)

	mw := io.MultiWriter(imageSaveFile, smWriter)

	imageSaveCmd := exec.CommandContext(ctx, t.cmdName, []string{"save", t.opts.ImageName}...)
	imageSaveCmd.Stdout = mw

	err = StreamStderr(imageSaveCmd, stderr, "image save")
	monitorCancel()

	if err != nil {
		return err
	}

	return nil
}

func (t *GenericImageSaveTask) runPullImage(ctx context.Context, stderr io.Writer) error {
	if !t.opts.ImageSavePull {
		return nil
	}

	imagePullCmd := exec.CommandContext(ctx, t.cmdName, "pull", t.opts.ImageName)
	// docker pull logs to stdout for some reason

	return StreamStdout(imagePullCmd, stderr, "image pull")
}
