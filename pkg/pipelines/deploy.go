package pipelines

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
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
	if config != nil && !config.Deploy.Enabled {
		slog.Warn("deploy pipeline is disabled, skipping.")
		return nil
	}
	return p
}

func (p *Deploy) preRun() error {
	if p == nil {
		return nil
	}
	p.runtime.bundleFilename = path.Join(p.config.ArtifactDir, p.config.GatecheckBundleFilename)
	return nil
}

//go:embed gatecheck.defaults.yml
var gatecheckDefaultConfig string

func mkDeploymentError(cause error) error {
	return fmt.Errorf("deployment Validation failed: %w", cause)
}

func (p *Deploy) Run() error {
	if p == nil {
		return nil
	}

	slog.Warn("BETA FEATURE: The deploy command performs bundle validation and invokes webhooks. Actual deployment is performed via webhooks.")

	if err := p.preRun(); err != nil {
		return errors.New("deploy Pipeline failed, pre-run error. See logs for details")
	}

	// Ensure artifacts directory exists before attempting to create files in it
	if err := MakeDirectoryP(p.config.ArtifactDir); err != nil {
		slog.Error("failed to create artifact directory", "name", p.config.ArtifactDir)
		return mkDeploymentError(err)
	}

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

		err = mergeAndSaveGatecheckConfig(customConfigFile, gatecheckConfig)
		if err != nil {
			return err
		}
	} else {
		// Automatically handle an optional .gatecheck.yml or .gatecheck.yaml file in the working directory
		// Unlike an explicitly specified configuration file, do not error if it does not exist.
		customConfigFile, err := os.ReadFile(".gatecheck.yml")
		if err != nil {
			if os.IsNotExist(err) {
				customConfigFile, err = os.ReadFile(".gatecheck.yaml")
				if err != nil && !os.IsNotExist(err) {
					return mkDeploymentError(err)
				}
			} else {
				// The file exists, but it isn't readable
				return mkDeploymentError(err)
			}
		}

		if len(customConfigFile) > 0 {
			err = mergeAndSaveGatecheckConfig(customConfigFile, gatecheckConfig)
			if err != nil {
				return err
			}
		} else {
			_, err = gatecheckConfig.Write([]byte(gatecheckDefaultConfig))
			if err != nil {
				return mkDeploymentError(err)
			}
		}
	}

	err = AddBundleFile(p.DryRunEnabled, p.runtime.bundleFilename, gatecheckConfigPath, "gatecheck-config", p.Stderr)
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

	for i, hook := range p.config.Deploy.SuccessWebhooks {
		slog.Info("preparing to submit deployment success webhook",
			"webhook_url", hook.Url,
			"authorization_var_name", hook.AuthorizationVar,
			"index", i)

		// Send a POST request with the bundle file in a multipart form
		bundleFile, err := os.Open(p.runtime.bundleFilename)
		if err != nil {
			slog.Error("failed to open bundle file", "error", err)
			return mkDeploymentError(err)
		}
		defer bundleFile.Close()

		var requestBody bytes.Buffer
		writer := multipart.NewWriter(&requestBody)

		writer.WriteField("action", "deploy")
		writer.WriteField("status", "success")

		bundleFilePart, err := writer.CreateFormFile("bundle", filepath.Base(p.runtime.bundleFilename))
		if err != nil {
			slog.Error("failed to create form file", "error", err)
			return mkDeploymentError(err)
		}

		_, err = io.Copy(bundleFilePart, bundleFile)
		if err != nil {
			slog.Error("failed to copy file content to form part", "error", err)
			return mkDeploymentError(err)
		}

		err = writer.Close()
		if err != nil {
			slog.Error("failed to close multipart writer", "error", err)
			return mkDeploymentError(err)
		}

		req, err := http.NewRequest("POST", hook.Url, &requestBody)
		if err != nil {
			slog.Error("failed to create HTTP request", "error", err)
			return mkDeploymentError(err)
		}

		// Set the Content-Type header to the multipart writer's content type
		req.Header.Set("Content-Type", writer.FormDataContentType())

		// Priority order for auth token:
		// 1. PORTAGE_DEPLOY_WEBHOOK_AUTH_TOKEN environment variable (via config.WebhookAuthToken)
		// 2. Environment variable specified in authorizationVar field in config file
		var authValue string
		var authSource string

		slog.Info("checking authorization configuration",
			"webhook_auth_token_from_viper", p.config.Deploy.WebhookAuthToken != "",
			"authorization_var_from_config", hook.AuthorizationVar)

		if p.config.Deploy.WebhookAuthToken != "" {
			authValue = p.config.Deploy.WebhookAuthToken
			authSource = "PORTAGE_DEPLOY_WEBHOOK_AUTH_TOKEN (via Viper)"
			slog.Info("using auth token from PORTAGE_DEPLOY_WEBHOOK_AUTH_TOKEN", "length", len(authValue))
		} else if hook.AuthorizationVar != "" {
			authValue = os.Getenv(hook.AuthorizationVar)
			authSource = fmt.Sprintf("%s (direct os.Getenv)", hook.AuthorizationVar)

			if authValue != "" {
				slog.Info("successfully retrieved auth token from environment variable",
					"env_var_name", hook.AuthorizationVar,
					"token_length", len(authValue))
			} else {
				slog.Error("environment variable specified but not set or empty",
					"env_var_name", hook.AuthorizationVar,
					"note", "This env var must be set in your GitLab CI environment")
			}
		}

		if authValue != "" {
			req.Header.Set("Authorization", authValue)
			last4 := authValue
			if len(authValue) > 4 {
				last4 = authValue[len(authValue)-4:]
			}
			slog.Info("authorization header added to request", "source", authSource, "auth_last4", last4, "auth_length", len(authValue))
		} else {
			// Only fail if authorization was explicitly configured but not provided
			// If no authorizationVar is specified, just warn (webhook might not require auth)
			if hook.AuthorizationVar != "" {
				return mkDeploymentError(fmt.Errorf("authorization required but environment variable '%s' is not set. Please set this variable in your GitLab CI environment", hook.AuthorizationVar))
			} else {
				slog.Warn("no authorization configured for webhook - proceeding without auth header", "webhook", hook.Url)
			}
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			slog.Error("failed to execute HTTP request", "error", err)
			return mkDeploymentError(err)
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("failed to read response body", "error", err)
			return mkDeploymentError(err)
		}

		slog.Debug("received webhook response",
			"status", resp.StatusCode,
			"webhook", hook.Url,
			"response_body", string(respBody),
			"content_type", resp.Header.Get("Content-Type"))

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			slog.Error("webhook returned non-success status",
				"status", resp.StatusCode,
				"response_body", string(respBody),
				"webhook_url", hook.Url,
				"error_details", map[string]interface{}{
					"status_code": resp.StatusCode,
					"headers":     resp.Header,
					"body":        string(respBody),
				})
			return fmt.Errorf("webhook request failed with status: %d - response: %s - url: %s",
				resp.StatusCode, string(respBody), hook.Url)
		}

		slog.Info("successfully submitted deployment success webhook", "webhook", hook)
	}

	return nil
}

func mergeAndSaveGatecheckConfig(customConfigFile []byte, gatecheckConfig *os.File) error {
	// Unmarshal the YAML into a map
	var customConfig tree.Map
	err := yaml.Unmarshal(customConfigFile, &customConfig)
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
	return nil
}
