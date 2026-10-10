package pipelines

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
)

const testVerdictFail = `{"verdict":{"decision":"fail","summary":"Deployment blocked","reasons":[
  {"rule":"grype.critical","status":"fail","message":"3 critical (limit 0)"},
  {"rule":"grype.high","status":"accepted","message":"4 high (limit 2), 2 covered by approved POA&M"},
  {"rule":"sbom.required","status":"pass","message":"SBOM present"}
],"detailsUrl":"https://belay.example.com/builds/1234?session=abc"}}`

const testVerdictPass = `{"verdict":{"decision":"pass","summary":"Deployment approved"}}`

func TestParseVerdict(t *testing.T) {
	cases := []struct {
		name        string
		contentType string
		body        string
		want        string
	}{
		{name: "fail", contentType: "application/json", body: testVerdictFail, want: "fail"},
		{name: "pass-charset", contentType: "application/json; charset=utf-8", body: testVerdictPass, want: "pass"},
		{name: "pending-case", contentType: "application/problem+json", body: `{"verdict":{"decision":" Pending "}}`, want: "pending"},
		{name: "no-verdict", contentType: "application/json", body: `{"id":"123"}`},
		{name: "empty-body", contentType: "application/json", body: ``},
		{name: "not-json-content-type", contentType: "text/plain", body: testVerdictPass},
		{name: "no-content-type", contentType: "", body: testVerdictPass},
		{name: "invalid-json", contentType: "application/json", body: `{"verdict":`},
		{name: "unknown-decision", contentType: "application/json", body: `{"verdict":{"decision":"maybe"}}`},
		{name: "verdict-wrong-type", contentType: "application/json", body: `{"verdict":"pass"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseVerdict(tc.contentType, []byte(tc.body))
			if tc.want == "" {
				if got != nil {
					t.Fatalf("want no verdict, got %+v", got)
				}
				return
			}
			if got == nil || got.Decision != tc.want {
				t.Fatalf("want decision %q, got %+v", tc.want, got)
			}
		})
	}
}

func TestWriteVerdict(t *testing.T) {
	buf := new(bytes.Buffer)
	writeVerdict(buf, parseVerdict("application/json", []byte(testVerdictFail)))

	want := "Deploy verdict: FAIL - Deployment blocked\n" +
		"  [fail]     grype.critical  3 critical (limit 0)\n" +
		"  [accepted] grype.high      4 high (limit 2), 2 covered by approved POA&M\n" +
		"  [pass]     sbom.required   SBOM present\n" +
		"  Details: https://belay.example.com/builds/1234\n"
	if buf.String() != want {
		t.Fatalf("unexpected output:\n%s\nwant:\n%s", buf.String(), want)
	}
}

func TestWriteVerdict_MinimalAndHostileText(t *testing.T) {
	buf := new(bytes.Buffer)
	writeVerdict(buf, &Verdict{
		Decision: "pass",
		Summary:  "ok\n::error::injected\x1b[31m",
		Reasons:  []VerdictReason{{Rule: "r", Status: "weird", Message: strings.Repeat("x", 1000)}},
	})
	out := buf.String()
	for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		if strings.HasPrefix(line, "::") {
			t.Fatalf("response text started a new line: %q", out)
		}
	}
	if strings.Contains(out, "\x1b") {
		t.Fatal("control characters must be removed")
	}
	if !strings.Contains(out, "Deploy verdict: PASS - ok ::error::injected[31m") {
		t.Fatalf("unexpected header: %q", out)
	}
	if !strings.Contains(out, "[-]") || !strings.Contains(out, strings.Repeat("x", maxVerdictTextLength)+"...") {
		t.Fatalf("want unknown status marker and truncated message: %q", out)
	}
}

func TestWriteVerdict_CapsReasons(t *testing.T) {
	reasons := make([]VerdictReason, maxVerdictReasons+5)
	for i := range reasons {
		reasons[i] = VerdictReason{Rule: "r", Status: "pass"}
	}
	buf := new(bytes.Buffer)
	writeVerdict(buf, &Verdict{Decision: "pass", Reasons: reasons})
	if !strings.Contains(buf.String(), "... 5 more") {
		t.Fatalf("want truncation notice:\n%s", buf.String())
	}
}

func TestDeploy_VerdictPrinted(t *testing.T) {
	h := newDeployHarness(t)
	h.respContentType = "application/json"
	h.respBody = testVerdictFail

	stdout := new(bytes.Buffer)
	err := NewDeploy(stdout, io.Discard).WithConfig(h.config).Run()
	if err != nil {
		t.Fatalf("fail verdict must not fail the step unless deploy.failOnVerdict is set: %v", err)
	}
	if !strings.Contains(stdout.String(), "Deploy verdict: FAIL - Deployment blocked") {
		t.Fatalf("want verdict printed, got:\n%s", stdout.String())
	}
}

func TestDeploy_NoVerdictUnchanged(t *testing.T) {
	h := newDeployHarness(t)
	h.respContentType = "text/plain"
	h.respBody = "accepted"

	stdout := new(bytes.Buffer)
	if err := NewDeploy(stdout, io.Discard).WithConfig(h.config).Run(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout.String(), "Deploy verdict") {
		t.Fatalf("want nothing printed without a verdict:\n%s", stdout.String())
	}
}

func TestDeploy_FailOnVerdict(t *testing.T) {
	h := newDeployHarness(t)
	h.config.Deploy.FailOnVerdict = true
	h.respContentType = "application/json"
	h.respBody = testVerdictFail

	err := h.run(t)
	if err == nil || !strings.Contains(err.Error(), "1 deploy webhook(s) returned a fail verdict") {
		t.Fatalf("want fail verdict error, got %v", err)
	}
	if h.webhooks.Load() != 1 {
		t.Fatalf("want the submission to be sent, got %d", h.webhooks.Load())
	}
}

func TestDeploy_FailOnVerdictPassAndPending(t *testing.T) {
	for _, body := range []string{testVerdictPass, `{"verdict":{"decision":"pending"}}`} {
		h := newDeployHarness(t)
		h.config.Deploy.FailOnVerdict = true
		h.respContentType = "application/json"
		h.respBody = body
		if err := h.run(t); err != nil {
			t.Fatalf("body %s: %v", body, err)
		}
	}
}

func TestDeploy_NonSuccessStatusWithVerdict(t *testing.T) {
	h := newDeployHarness(t)
	h.respStatus = http.StatusForbidden
	h.respContentType = "application/json"
	h.respBody = `{"verdict":{"decision":"fail","summary":"token belongs to another repository"}}`

	stdout := new(bytes.Buffer)
	err := NewDeploy(stdout, io.Discard).WithConfig(h.config).Run()
	if err == nil || !strings.Contains(err.Error(), "status: 403") {
		t.Fatalf("want status error, got %v", err)
	}
	if !strings.Contains(stdout.String(), "token belongs to another repository") {
		t.Fatalf("want verdict printed on error:\n%s", stdout.String())
	}
}

func TestDeploy_WebhookSecretsNotLogged(t *testing.T) {
	logBuf := new(bytes.Buffer)
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(previous)

	h := newDeployHarness(t)
	h.config.Deploy.SuccessWebhooks[0].Url += "?api_key=querysecret"
	h.config.Deploy.SuccessWebhooks[0].AuthorizationVar = "DEPLOY_TOKEN"
	t.Setenv("DEPLOY_TOKEN", "Bearer tokenvalue-WXYZ")
	h.respStatus = http.StatusUnauthorized
	h.respBody = "invalid token"

	err := h.run(t)
	if err == nil {
		t.Fatal("want error")
	}
	for _, secret := range []string{"querysecret", "tokenvalue", "WXYZ"} {
		if strings.Contains(logBuf.String(), secret) {
			t.Fatalf("log leaked %q:\n%s", secret, logBuf.String())
		}
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("error leaked %q: %v", secret, err)
		}
	}
	if !strings.Contains(err.Error(), "invalid token") || !strings.Contains(err.Error(), "/deploy") {
		t.Fatalf("want status body excerpt and redacted url in error: %v", err)
	}
}

func TestDeploy_UnreachableWebhookURLRedacted(t *testing.T) {
	h := newDeployHarness(t)
	h.config.Deploy.SuccessWebhooks[0].Url = "http://127.0.0.1:1/deploy?api_key=querysecret"

	err := h.run(t)
	if err == nil {
		t.Fatal("want connection error")
	}
	if strings.Contains(err.Error(), "querysecret") {
		t.Fatalf("error leaked the query string: %v", err)
	}
}
