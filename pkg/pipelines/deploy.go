package pipelines

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"portage/pkg/shell"

	"github.com/jarxorg/tree"
	"gopkg.in/yaml.v3"
)

type Deploy struct {
	Stdout        io.Writer
	Stderr        io.Writer
	DryRunEnabled bool
	config        *Config
	runtime       struct {
		bundleFilename string
	}
}

func NewDeploy(stdout io.Writer, stderr io.Writer) *Deploy {
	return &Deploy{
		Stdout:        stdout,
		Stderr:        stderr,
		DryRunEnabled: false,
	}
}

func (p *Deploy) WithConfig(config *Config) *Deploy {
	p.config = config
	return p
}

func (p *Deploy) preRun() error {
	p.runtime.bundleFilename = path.Join(p.config.ArtifactDir, p.config.GatecheckBundleFilename)
	return nil
}

//go:embed gatecheck.defaults.yml
var gatecheckDefaultConfig string

func mkDeploymentError(cause error) error {
	return fmt.Errorf("deployment Validation failed: %w", cause)
}

func (p *Deploy) Run() error {
	if !p.config.Deploy.Enabled {
		slog.Warn("deployment pipeline disabled, skip.")
		return nil
	}
	if err := p.preRun(); err != nil {
		return errors.New("deploy Pipeline failed, pre-run error. See logs for details")
	}

	slog.Warn("deployment pipeline is a beta feature. Only gatecheck validation will be conducted.")

	gatecheckConfigPath := path.Join(p.config.ArtifactDir, "gatecheck-config.yml")
	gatecheckConfig, err := os.OpenFile(gatecheckConfigPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return mkDeploymentError(err)
	}
	defer gatecheckConfig.Close()

	if p.config.Deploy.GatecheckConfigFilename != "" {
		customConfigFile, err := os.ReadFile(p.config.Deploy.GatecheckConfigFilename)
		if err != nil {
			return mkDeploymentError(err)
		}

		// Unmarshal the YAML into a map
		var customConfig tree.Map
		err = yaml.Unmarshal(customConfigFile, &customConfig)
		if err != nil {
			return mkDeploymentError(err)
		}

		var baseConfig tree.Map
		err = yaml.Unmarshal([]byte(gatecheckDefaultConfig), &baseConfig)
		if err != nil {
			return mkDeploymentError(err)
		}

		// Merge the trees and write the result to gatecheckConfig os.File
		mergedConfig := tree.Merge(baseConfig, customConfig, tree.MergeOptionReplaceArray|tree.MergeOptionOverrideMap)

		b, err := yaml.Marshal(mergedConfig)
		if err != nil {
			return mkDeploymentError(err)
		}
		_, err = gatecheckConfig.Write(b)
		if err != nil {
			return mkDeploymentError(err)
		}
	} else {
		_, err = gatecheckConfig.Write([]byte(gatecheckDefaultConfig))
		if err != nil {
			return mkDeploymentError(err)
		}
		gatecheckConfig.Close()
	}

	err = AddBundleFile(p.DryRunEnabled, p.runtime.bundleFilename, gatecheckConfigPath, p.Stderr)
	if err != nil {
		return mkDeploymentError(err)
	}

	err = shell.GatecheckValidate(
		shell.WithDryRun(p.DryRunEnabled),
		shell.WithStderr(p.Stderr),
		shell.WithStdout(p.Stdout),
		shell.WithTargetFile(p.runtime.bundleFilename),
		shell.WithConfigFile(gatecheckConfigPath),
	)
	if err != nil {
		return mkDeploymentError(err)
	}

	return nil
}
