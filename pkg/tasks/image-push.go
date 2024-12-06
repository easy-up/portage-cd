package tasks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
)

// Add a whitelist of allowed CLI tools
var allowedCLITools = map[string]bool{
	"docker":  true,
	"podman":  true,
	"buildah": true,
	// Add other valid CLI tools as needed
}

type GenericImagePushTask struct {
	TagName string
	cmdName string
}

func NewGenericImagePushTask(cliInterface string, tagName string) (*GenericImagePushTask, error) {
	// Validate CLI interface against whitelist
	if !allowedCLITools[cliInterface] {
		return nil, errors.New("invalid CLI interface specified")
	}

	return &GenericImagePushTask{
		TagName: tagName,
		cmdName: cliInterface,
	}, nil
}

func (t *GenericImagePushTask) Run(ctx context.Context, stderr io.Writer) error {
	pushCmd := exec.Command(t.cmdName, "push")
	return StreamStderr(pushCmd, stderr, fmt.Sprintf("%s push", t.cmdName))
}
