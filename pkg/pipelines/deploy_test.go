package pipelines

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Stub CLIs record their arguments to $STUB_LOG, one line per call
const stubGatecheck = `#!/bin/sh
echo "gatecheck $*" >> "$STUB_LOG"
if [ "$1" = "bundle" ] && [ "$2" = "create" ]; then echo bundle > "$3"; fi
if [ "$1" = "validate" ]; then exit "${STUB_VALIDATE_EXIT:-0}"; fi
if [ "$1" = "config" ] && [ "$2" = "fetch" ]; then
  if [ -n "$STUB_FETCH_EXIT" ]; then echo "Error: config fetch: returned status 403" >&2; exit "$STUB_FETCH_EXIT"; fi
  out=""
  while [ $# -gt 0 ]; do
    if [ "$1" = "-o" ]; then out="$2"; fi
    shift
  done
  printf '%s' "$STUB_POLICY" > "$out"
fi
exit 0
`

const stubOras = `#!/bin/sh
echo "oras $*" >> "$STUB_LOG"
if [ -n "$STUB_ORAS_FAIL" ]; then echo "Error: $STUB_ORAS_FAIL" >&2; exit 1; fi
if [ "$3" = "--descriptor" ]; then printf '%s' "$STUB_ORAS_DESCRIPTOR"; exit 0; fi
printf '%s' "$STUB_ORAS_MANIFEST"
`

const stubDocker = `#!/bin/sh
echo "docker $*" >> "$STUB_LOG"
if [ -z "$STUB_DOCKER_INSPECT" ]; then echo "Error: No such image: $3" >&2; exit 1; fi
printf '%s' "$STUB_DOCKER_INSPECT"
`

type deployHarness struct {
	config   *Config
	logFile  string
	webhooks *atomic.Int32
	// fields holds the multipart form values of each webhook request
	fields []map[string]string
	mu     sync.Mutex
	// response returned by the webhook server, defaults to an empty 200
	respStatus      int
	respContentType string
	respBody        string
}

func (h *deployHarness) lastFields(t *testing.T) map[string]string {
	t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.fields) == 0 {
		t.Fatal("no webhook request received")
	}
	return h.fields[len(h.fields)-1]
}

func newDeployHarness(t *testing.T) *deployHarness {
	t.Helper()
	binDir := t.TempDir()
	for name, script := range map[string]string{"gatecheck": stubGatecheck, "oras": stubOras, "docker": stubDocker} {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	workDir := t.TempDir()
	logFile := filepath.Join(workDir, "stub.log")
	t.Setenv("STUB_LOG", logFile)

	gatecheckConfig := filepath.Join(workDir, "custom-gatecheck.yml")
	if err := os.WriteFile(gatecheckConfig, []byte("version: \"1\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	h := &deployHarness{logFile: logFile, webhooks: new(atomic.Int32)}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.webhooks.Add(1)
		fields := map[string]string{}
		if err := r.ParseMultipartForm(1 << 20); err == nil {
			for key, values := range r.MultipartForm.Value {
				fields[key] = values[0]
			}
		}
		h.mu.Lock()
		h.fields = append(h.fields, fields)
		status, contentType, body := h.respStatus, h.respContentType, h.respBody
		h.mu.Unlock()
		if status == 0 {
			status = http.StatusOK
		}
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	config := NewDefaultConfig()
	config.ArtifactDir = filepath.Join(workDir, "artifacts")
	config.ImageTag = testRef
	config.Deploy.Enabled = true
	config.Deploy.GatecheckConfigFilename = gatecheckConfig
	config.Deploy.SuccessWebhooks = []webhookConfig{{Url: server.URL + "/deploy"}}

	h.config = config
	return h
}

func (h *deployHarness) calls(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile(h.logFile)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(content)
}

func (h *deployHarness) run(t *testing.T) error {
	t.Helper()
	return NewDeploy(io.Discard, io.Discard).WithConfig(h.config).Run()
}

func TestDeploy_DefaultDoesNotWaitForImage(t *testing.T) {
	h := newDeployHarness(t)

	if err := h.run(t); err != nil {
		t.Fatal(err)
	}

	calls := h.calls(t)
	if strings.Contains(calls, "oras ") || strings.Contains(calls, "docker ") {
		t.Fatalf("default deploy must not query the registry or local images:\n%s", calls)
	}
	if strings.Contains(calls, "--build-published-image") || strings.Contains(calls, "--build-image-") {
		t.Fatalf("default deploy must not record a published image:\n%s", calls)
	}
	if h.webhooks.Load() != 1 {
		t.Fatalf("want 1 webhook, got %d", h.webhooks.Load())
	}
}

func TestDeploy_DefaultValidationFailureStillBlocks(t *testing.T) {
	h := newDeployHarness(t)
	t.Setenv("STUB_VALIDATE_EXIT", "1")

	if err := h.run(t); err == nil {
		t.Fatal("want validation failure")
	}
	if h.webhooks.Load() != 0 {
		t.Fatalf("want no webhook on validation failure, got %d", h.webhooks.Load())
	}
}

func TestDeploy_WaitForImageMatched(t *testing.T) {
	h := newDeployHarness(t)
	h.config.Deploy.WaitForImage = true
	t.Setenv("STUB_ORAS_DESCRIPTOR", `{"mediaType":"application/vnd.oci.image.manifest.v1+json","digest":"`+testManifestDigest+`","size":100}`)
	t.Setenv("STUB_ORAS_MANIFEST", imageManifestJSON(testLocalConfig))
	t.Setenv("STUB_DOCKER_INSPECT", `[{"Id":"`+testLocalConfig+`","Os":"linux","Architecture":"amd64"}]`)

	if err := h.run(t); err != nil {
		t.Fatal(err)
	}

	calls := h.calls(t)
	for _, want := range []string{
		"docker image inspect " + testRef,
		"oras manifest fetch --descriptor " + testRef,
		"oras manifest fetch " + testRepo + "@" + testManifestDigest,
		"--build-published-image " + testRef + " --build-image-digest " + testManifestDigest + " --build-image-verification matched",
	} {
		if !strings.Contains(calls, want) {
			t.Fatalf("want call containing %q:\n%s", want, calls)
		}
	}
	if h.webhooks.Load() != 1 {
		t.Fatalf("want 1 webhook, got %d", h.webhooks.Load())
	}
}

func TestDeploy_WaitForImageExistsOnly(t *testing.T) {
	h := newDeployHarness(t)
	h.config.Deploy.WaitForImage = true
	t.Setenv("STUB_ORAS_DESCRIPTOR", `{"digest":"`+testManifestDigest+`"}`)

	if err := h.run(t); err != nil {
		t.Fatal(err)
	}

	calls := h.calls(t)
	if !strings.Contains(calls, "--build-image-digest "+testManifestDigest+" --build-image-verification exists-only") {
		t.Fatalf("want exists-only verification recorded:\n%s", calls)
	}
	if h.webhooks.Load() != 1 {
		t.Fatalf("want 1 webhook, got %d", h.webhooks.Load())
	}
}

func TestDeploy_WaitForImageTimeoutBlocksWebhooks(t *testing.T) {
	h := newDeployHarness(t)
	h.config.Deploy.WaitForImage = true
	h.config.Deploy.WaitForImageTimeout = 300 * time.Millisecond
	h.config.Deploy.WaitForImagePollInterval = 50 * time.Millisecond
	t.Setenv("STUB_ORAS_FAIL", testRef+": not found")
	t.Setenv("STUB_DOCKER_INSPECT", `[{"Id":"`+testLocalConfig+`","Os":"linux","Architecture":"amd64"}]`)

	err := h.run(t)
	if err == nil {
		t.Fatal("want error when the image is never published")
	}
	for _, want := range []string{testRef, "expected config digest " + testLocalConfig, "last seen none", "not found"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("want error containing %q, got: %v", want, err)
		}
	}
	if h.webhooks.Load() != 0 {
		t.Fatalf("want no webhook when the image is not verified, got %d", h.webhooks.Load())
	}
	if strings.Contains(h.calls(t), "gatecheck validate") {
		t.Fatal("want no validation after image verification fails")
	}
}

func TestDeploy_WaitForImageStaleTagBlocksWebhooks(t *testing.T) {
	h := newDeployHarness(t)
	h.config.Deploy.WaitForImage = true
	h.config.Deploy.WaitForImageTimeout = 300 * time.Millisecond
	h.config.Deploy.WaitForImagePollInterval = 50 * time.Millisecond
	t.Setenv("STUB_ORAS_DESCRIPTOR", `{"digest":"`+testStaleManifest+`"}`)
	t.Setenv("STUB_ORAS_MANIFEST", imageManifestJSON(testStaleConfig))
	t.Setenv("STUB_DOCKER_INSPECT", `[{"Id":"`+testLocalConfig+`","Os":"linux","Architecture":"amd64"}]`)

	err := h.run(t)
	if err == nil || !strings.Contains(err.Error(), "last seen "+testStaleConfig) {
		t.Fatalf("want stale image error, got %v", err)
	}
	if h.webhooks.Load() != 0 {
		t.Fatalf("want no webhook for a stale image, got %d", h.webhooks.Load())
	}
}

func TestDeploy_WaitForImageDryRun(t *testing.T) {
	h := newDeployHarness(t)
	h.config.Deploy.WaitForImage = true

	pipeline := NewDeploy(io.Discard, io.Discard).WithConfig(h.config)
	pipeline.DryRunEnabled = true
	// a dry run never creates the bundle, so the webhook step fails as it did before this feature
	_ = pipeline.Run()
	if strings.Contains(h.calls(t), "oras ") {
		t.Fatal("dry run must not query the registry")
	}
}
