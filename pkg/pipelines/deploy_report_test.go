package pipelines

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testPolicy = "# from policy server\nversion: \"1\"\ngrype:\n  severityLimit:\n    critical:\n      enabled: true\n      limit: 0\n"

func TestDeploy_EnforceIsDefaultAndSendsNoNewFields(t *testing.T) {
	h := newDeployHarness(t)
	if h.config.Deploy.Validation != DeployValidationEnforce {
		t.Fatalf("want default validation %q, got %q", DeployValidationEnforce, h.config.Deploy.Validation)
	}

	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	fields := h.lastFields(t)
	if _, ok := fields["validation"]; ok {
		t.Fatal("enforce mode must not send a validation field")
	}
	if _, ok := fields["policy"]; ok {
		t.Fatal("enforce mode must not send a policy field")
	}
	if fields["action"] != "deploy" || fields["status"] != "success" {
		t.Fatalf("unexpected existing fields %v", fields)
	}
}

func TestDeploy_EmptyValidationIsEnforce(t *testing.T) {
	h := newDeployHarness(t)
	h.config.Deploy.Validation = ""
	t.Setenv("STUB_VALIDATE_EXIT", "1")

	if err := h.run(t); err == nil {
		t.Fatal("want validation failure to block when deploy.validation is unset")
	}
	if h.webhooks.Load() != 0 {
		t.Fatal("want no webhook")
	}
}

func TestDeploy_InvalidValidationMode(t *testing.T) {
	h := newDeployHarness(t)
	h.config.Deploy.Validation = "reprot"

	err := h.run(t)
	if err == nil || !strings.Contains(err.Error(), `invalid deploy.validation "reprot"`) {
		t.Fatalf("want invalid mode error, got %v", err)
	}
	if h.webhooks.Load() != 0 || strings.Contains(h.calls(t), "gatecheck") {
		t.Fatal("want nothing run for an invalid mode")
	}
}

func TestDeploy_ReportModePassed(t *testing.T) {
	h := newDeployHarness(t)
	h.config.Deploy.Validation = "Report"

	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	fields := h.lastFields(t)
	if fields["validation"] != "passed" || fields["policy"] != "local" {
		t.Fatalf("want validation=passed policy=local, got %v", fields)
	}
}

func TestDeploy_ReportModeValidationFailureContinues(t *testing.T) {
	h := newDeployHarness(t)
	h.config.Deploy.Validation = DeployValidationReport
	t.Setenv("STUB_VALIDATE_EXIT", "1")

	if err := h.run(t); err != nil {
		t.Fatalf("want report mode to continue past a validation failure, got %v", err)
	}
	if h.webhooks.Load() != 1 {
		t.Fatalf("want 1 webhook, got %d", h.webhooks.Load())
	}
	if got := h.lastFields(t)["validation"]; got != "failed" {
		t.Fatalf("want validation=failed, got %q", got)
	}
}

func TestDeploy_ReportModeSystemErrorStillBlocks(t *testing.T) {
	h := newDeployHarness(t)
	h.config.Deploy.Validation = DeployValidationReport
	t.Setenv("STUB_VALIDATE_EXIT", "2")

	if err := h.run(t); err == nil {
		t.Fatal("want a gatecheck system error to fail the deploy in report mode")
	}
	if h.webhooks.Load() != 0 {
		t.Fatal("want no webhook on a system error")
	}
}

func TestDeploy_PolicyFetchedReplacesLocalConfig(t *testing.T) {
	h := newDeployHarness(t)
	h.config.Deploy.Validation = DeployValidationReport
	h.config.Deploy.PolicyURL = "https://policy.example.com/Policy/acme/api/gatecheck?branch=main"
	h.config.Deploy.SuccessWebhooks[0].AuthorizationVar = "DEPLOY_TOKEN"
	t.Setenv("DEPLOY_TOKEN", "tok-abcdef-123456")
	t.Setenv("STUB_POLICY", testPolicy)

	if err := h.run(t); err != nil {
		t.Fatal(err)
	}

	calls := h.calls(t)
	configPath := filepath.Join(h.config.ArtifactDir, "gatecheck-config.yml")
	want := "gatecheck config fetch --url " + h.config.Deploy.PolicyURL + " -o " + configPath + " --auth-env DEPLOY_TOKEN"
	if !strings.Contains(calls, want) {
		t.Fatalf("want call %q:\n%s", want, calls)
	}
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != testPolicy {
		t.Fatalf("want fetched policy used unmodified as the gatecheck config, got:\n%s", content)
	}
	if got := h.lastFields(t)["policy"]; got != "fetched" {
		t.Fatalf("want policy=fetched, got %q", got)
	}
}

func TestDeploy_PolicyFetchFailureFailsInBothModes(t *testing.T) {
	for _, mode := range []string{DeployValidationEnforce, DeployValidationReport} {
		t.Run(mode, func(t *testing.T) {
			h := newDeployHarness(t)
			h.config.Deploy.Validation = mode
			h.config.Deploy.PolicyURL = "https://policy.example.com/Policy/acme/api/gatecheck?branch=main"
			t.Setenv("STUB_FETCH_EXIT", "255")

			err := h.run(t)
			if err == nil || !strings.Contains(err.Error(), "could not fetch gatecheck policy from https://policy.example.com/Policy/acme/api/gatecheck") {
				t.Fatalf("want policy fetch error, got %v", err)
			}
			if strings.Contains(err.Error(), "branch=main") {
				t.Fatalf("error must not include the query string: %v", err)
			}
			if h.webhooks.Load() != 0 {
				t.Fatal("want no webhook when the policy cannot be fetched")
			}
			if strings.Contains(h.calls(t), "gatecheck validate") {
				t.Fatal("want no validation against a fallback config")
			}
		})
	}
}

func TestDeploy_PolicyAuthVarResolution(t *testing.T) {
	cases := []struct {
		name          string
		policyAuthVar string
		headerEnv     string
		hookVar       string
		want          string
	}{
		{name: "explicit", policyAuthVar: "POLICY_TOKEN", headerEnv: "x", hookVar: "HOOK_TOKEN", want: "POLICY_TOKEN"},
		{name: "webhook-header-env", headerEnv: "Bearer x", hookVar: "HOOK_TOKEN", want: "PORTAGE_DEPLOY_WEBHOOK_AUTH_HEADER"},
		{name: "webhook-authorization-var", hookVar: "HOOK_TOKEN", want: "HOOK_TOKEN"},
		{name: "none", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PORTAGE_DEPLOY_WEBHOOK_AUTH_HEADER", tc.headerEnv)
			p := &Deploy{config: NewDefaultConfig()}
			p.config.Deploy.PolicyAuthVar = tc.policyAuthVar
			p.config.Deploy.SuccessWebhooks = []webhookConfig{{Url: "https://example.com", AuthorizationVar: tc.hookVar}}
			if got := p.policyAuthVar(); got != tc.want {
				t.Fatalf("want %q, got %q", tc.want, got)
			}
		})
	}
}

func TestDeploy_PolicySecretsNotLogged(t *testing.T) {
	logBuf := new(bytes.Buffer)
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(previous)

	h := newDeployHarness(t)
	h.config.Deploy.Validation = DeployValidationReport
	h.config.Deploy.PolicyURL = "https://policy.example.com/Policy/acme/api/gatecheck?branch=main&sig=querysecret"
	h.config.Deploy.PolicyAuthVar = "POLICY_TOKEN"
	t.Setenv("POLICY_TOKEN", "policy-token-value-xyz")
	t.Setenv("STUB_POLICY", testPolicy)

	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"policy-token-value-xyz", "querysecret"} {
		if strings.Contains(logBuf.String(), secret) {
			t.Fatalf("log leaked %q:\n%s", secret, logBuf.String())
		}
	}
	if strings.Contains(h.calls(t), "policy-token-value-xyz") {
		t.Fatal("credential value must not be passed on the command line")
	}
}
