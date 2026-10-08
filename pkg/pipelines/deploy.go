package pipelines

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"portage/pkg/shell"
	"strings"

	"github.com/jarxorg/tree"
	"gopkg.in/yaml.v3"
)

// Values for deploy.validation
const (
	DeployValidationEnforce = "enforce"
	DeployValidationReport  = "report"
)

// Values sent in the report mode webhook form fields "validation" and "policy"
const (
	validationResultPassed = "passed"
	validationResultFailed = "failed"
	policySourceFetched    = "fetched"
	policySourceLocal      = "local"
)

// gatecheck validate exits 1 for a validation failure, other non-zero codes are system errors
const gatecheckExitValidationFail = 1

type Deploy struct {
	Stdout        io.Writer
	Stderr        io.Writer
	DryRunEnabled bool
	DockerAlias   string
	config        *Config
	imageWaiter   *imageWaiter
	runtime       struct {
		bundleFilename string
		verifiedImage  *VerifiedImage
		validationMode string
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

	mode, err := deployValidationMode(p.config.Deploy.Validation)
	if err != nil {
		return err
	}
	p.runtime.validationMode = mode
	if mode == DeployValidationReport {
		slog.Warn("deploy validation is in report mode: gatecheck results are printed but do not block deploy webhooks; the webhook receiver makes the deployment decision")
	}

	// Ensure artifacts directory exists before attempting to create files in it
	if err := MakeDirectoryP(p.config.ArtifactDir); err != nil {
		slog.Error("failed to create artifact directory", "name", p.config.ArtifactDir)
		return mkDeploymentError(err)
	}

	if err := p.waitForImage(); err != nil {
		return err
	}

	gatecheckConfigPath := path.Join(p.config.ArtifactDir, "gatecheck-config.yml")
	policySource := policySourceLocal
	if p.config.Deploy.PolicyURL != "" {
		if err := p.fetchPolicy(gatecheckConfigPath); err != nil {
			return err
		}
		policySource = policySourceFetched
	} else if err := p.writeLocalGatecheckConfig(gatecheckConfigPath); err != nil {
		return err
	}

	err = addBundleFile(p.config, p.DryRunEnabled, p.runtime.bundleFilename, gatecheckConfigPath, "gatecheck-config", p.Stderr, p.runtime.verifiedImage)
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
	validationResult := validationResultPassed
	if err != nil {
		var cmdErr *shell.ErrCommand
		isValidationFailure := errors.As(err, &cmdErr) && cmdErr.ExitCode == gatecheckExitValidationFail
		if p.runtime.validationMode != DeployValidationReport || !isValidationFailure {
			return mkDeploymentError(err)
		}
		validationResult = validationResultFailed
		slog.Warn("gatecheck validation FAILED (report mode): continuing to deploy webhooks, the webhook receiver makes the deployment decision")
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
		if p.runtime.validationMode == DeployValidationReport {
			writer.WriteField("validation", validationResult)
			writer.WriteField("policy", policySource)
		}

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

		// Priority order for auth header:
		// 1. PORTAGE_DEPLOY_WEBHOOK_AUTH_HEADER environment variable (via config.WebhookAuthHeader)
		// 2. Environment variable specified in authorizationVar field in config file
		var authValue string
		var authSource string

		slog.Info("checking authorization configuration",
			"webhook_auth_header_from_viper", p.config.Deploy.WebhookAuthHeader != "",
			"authorization_var_from_config", hook.AuthorizationVar)

		if p.config.Deploy.WebhookAuthHeader != "" {
			authValue = p.config.Deploy.WebhookAuthHeader
			authSource = "PORTAGE_DEPLOY_WEBHOOK_AUTH_HEADER (via Viper)"
			slog.Info("using auth header from PORTAGE_DEPLOY_WEBHOOK_AUTH_HEADER", "length", len(authValue))
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

// writeLocalGatecheckConfig merges the configured or working directory gatecheck config with the defaults
func (p *Deploy) writeLocalGatecheckConfig(gatecheckConfigPath string) error {
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
	return nil
}

// fetchPolicy downloads the gatecheck config from deploy.policyUrl into gatecheckConfigPath
//
// There is no fallback: if the policy cannot be fetched the deploy fails, so a pipeline never
// silently validates against a different (possibly more permissive) local config.
func (p *Deploy) fetchPolicy(gatecheckConfigPath string) error {
	safeURL := redactURL(p.config.Deploy.PolicyURL)
	authVar := p.policyAuthVar()

	ignored := p.config.Deploy.GatecheckConfigFilename
	if ignored == "" {
		ignored = ".gatecheck.yml/.gatecheck.yaml"
	}
	slog.Info("using gatecheck policy from deploy.policyUrl, local gatecheck config is ignored",
		"policy_url", safeURL, "auth_var_name", authVar, "ignored_local_config", ignored)

	err := shell.GatecheckConfigFetch(
		shell.WithDryRun(p.DryRunEnabled),
		shell.WithStdout(p.Stdout),
		shell.WithStderr(p.Stderr),
		shell.WithPolicyFetch(p.config.Deploy.PolicyURL, authVar),
		shell.WithTargetFile(gatecheckConfigPath),
		shell.WithDisplayCommand("gatecheck config fetch --url "+safeURL+" -o "+gatecheckConfigPath),
	)
	if err != nil {
		slog.Error("failed to fetch gatecheck policy, deploy webhooks will not be invoked", "policy_url", safeURL)
		return fmt.Errorf("deploy pipeline failed: could not fetch gatecheck policy from %s: %w", safeURL, err)
	}
	return nil
}

// policyAuthVar resolves the environment variable NAME holding the policy credential
//
// Order: deploy.policyAuthVar, PORTAGE_DEPLOY_WEBHOOK_AUTH_HEADER when set, then the first
// webhook authorizationVar. The credential value is never passed on the command line.
func (p *Deploy) policyAuthVar() string {
	if p.config.Deploy.PolicyAuthVar != "" {
		return p.config.Deploy.PolicyAuthVar
	}
	if os.Getenv("PORTAGE_DEPLOY_WEBHOOK_AUTH_HEADER") != "" {
		return "PORTAGE_DEPLOY_WEBHOOK_AUTH_HEADER"
	}
	for _, hook := range p.config.Deploy.SuccessWebhooks {
		if hook.AuthorizationVar != "" {
			return hook.AuthorizationVar
		}
	}
	return ""
}

func deployValidationMode(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", DeployValidationEnforce:
		return DeployValidationEnforce, nil
	case DeployValidationReport:
		return DeployValidationReport, nil
	}
	return "", fmt.Errorf("deploy pipeline failed: invalid deploy.validation %q, must be %q or %q",
		value, DeployValidationEnforce, DeployValidationReport)
}

// redactURL returns scheme://host/path without user info, query or fragment
func redactURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "<invalid url>"
	}
	return (&url.URL{Scheme: u.Scheme, Host: u.Host, Path: u.Path}).String()
}

// waitForImage blocks until the configured image is verified in the registry when deploy.waitForImage is enabled
//
// Webhooks are never invoked if the image cannot be verified within the timeout.
func (p *Deploy) waitForImage() error {
	if !p.config.Deploy.WaitForImage {
		return nil
	}
	if p.DryRunEnabled {
		slog.Info("dry run: skipping wait for published image", "image", p.config.ImageTag)
		return nil
	}

	waiter := p.imageWaiter
	if waiter == nil {
		alias := shell.DockerAliasDocker
		if strings.ToLower(p.DockerAlias) == "podman" {
			alias = shell.DockerAliasPodman
		}
		waiter = &imageWaiter{
			registry:     orasRegistry{},
			local:        cliImageStore{alias: alias},
			timeout:      p.config.Deploy.WaitForImageTimeout,
			pollInterval: p.config.Deploy.WaitForImagePollInterval,
			sleep:        sleepContext,
		}
	}
	if waiter.timeout <= 0 {
		waiter.timeout = defaultWaitForImageTimeout
	}
	if waiter.pollInterval <= 0 {
		waiter.pollInterval = defaultWaitForImagePollInterval
	}

	verified, err := waiter.Wait(context.Background(), p.config.ImageTag)
	if err != nil {
		slog.Error("published image verification failed, deploy webhooks will not be invoked", "image", p.config.ImageTag)
		return fmt.Errorf("deploy pipeline failed: %w", err)
	}
	if verified.Verification == ImageVerificationExistsOnly {
		slog.Warn("published image exists but was not compared to a local build; it is only safe to deploy if the tag is unique to this build",
			"image", verified.Reference, "digest", verified.Digest)
	}
	p.runtime.verifiedImage = verified
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
