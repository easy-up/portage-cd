package pipelines

import (
	"portage/pkg/shell"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestRedactCVEIDsConfig(t *testing.T) {
	v := viper.New()
	BindViper(v)
	if v.GetBool("redactcveids") {
		t.Fatal("redaction must be off by default")
	}

	t.Setenv("PORTAGE_REDACT_CVE_IDS", "true")
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		t.Fatal(err)
	}
	if !config.RedactCVEIDs || !v.GetBool("redactcveids") {
		t.Fatal("want PORTAGE_REDACT_CVE_IDS=true to enable redaction")
	}
}

func TestDeploy_GatecheckCallsCarryRedactionFlag(t *testing.T) {
	h := newDeployHarness(t)
	shell.SetGatecheckRedaction(true)
	t.Cleanup(func() { shell.SetGatecheckRedaction(false) })

	if err := h.run(t); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(h.calls(t)), "\n")
	gatecheckCalls := 0
	for _, line := range lines {
		if !strings.HasPrefix(line, "gatecheck ") {
			continue
		}
		gatecheckCalls++
		if !strings.HasSuffix(line, " --redact-cve-ids") {
			t.Fatalf("gatecheck call without redaction flag: %s", line)
		}
	}
	if gatecheckCalls < 2 {
		t.Fatalf("want bundle and validate calls, got:\n%s", h.calls(t))
	}
}
