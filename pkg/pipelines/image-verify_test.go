package pipelines

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

const (
	testRef            = "registry.example.com/team/api:abc1234"
	testRepo           = "registry.example.com/team/api"
	testLocalConfig    = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	testStaleConfig    = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	testManifestDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	testStaleManifest  = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	testIndexDigest    = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	testArmManifest    = "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	testAttestManifest = "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
)

// registryState is one snapshot of the registry, keyed by reference
type registryState struct {
	descriptors map[string]string // ref -> digest
	manifests   map[string]string // ref@digest -> manifest JSON
}

// fakeRegistry returns states in order, one per Descriptor call, repeating the last
type fakeRegistry struct {
	states []registryState
	calls  int
}

func (r *fakeRegistry) current() registryState {
	i := r.calls - 1
	if i >= len(r.states) {
		i = len(r.states) - 1
	}
	return r.states[i]
}

func (r *fakeRegistry) Descriptor(_ context.Context, ref string) (ociDescriptor, error) {
	r.calls++
	digest, ok := r.current().descriptors[ref]
	if !ok {
		return ociDescriptor{}, errors.New("manifest unknown")
	}
	return ociDescriptor{Digest: digest}, nil
}

func (r *fakeRegistry) Manifest(_ context.Context, ref string) ([]byte, error) {
	content, ok := r.current().manifests[ref]
	if !ok {
		return nil, errors.New("manifest unknown")
	}
	return []byte(content), nil
}

type fakeLocalStore struct {
	image *localImage
}

func (s fakeLocalStore) Inspect(context.Context, string) (*localImage, error) {
	if s.image == nil {
		return nil, errors.New("No such image")
	}
	copied := *s.image
	return &copied, nil
}

func imageManifestJSON(configDigest string) string {
	b, _ := json.Marshal(ociManifest{
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Config:    &ociDescriptor{MediaType: "application/vnd.oci.image.config.v1+json", Digest: configDigest},
	})
	return string(b)
}

func singleImageState(manifestDigest, configDigest string) registryState {
	return registryState{
		descriptors: map[string]string{testRef: manifestDigest},
		manifests:   map[string]string{testRepo + "@" + manifestDigest: imageManifestJSON(configDigest)},
	}
}

func newTestWaiter(registry imageRegistry, local *localImage) *imageWaiter {
	return &imageWaiter{
		registry:     registry,
		local:        fakeLocalStore{image: local},
		timeout:      200 * time.Millisecond,
		pollInterval: time.Millisecond,
		sleep:        sleepContext,
	}
}

func amd64Image(id string) *localImage {
	return &localImage{ID: id, OS: "linux", Architecture: "amd64"}
}

func TestImageWaiter_MatchedSingleManifest(t *testing.T) {
	registry := &fakeRegistry{states: []registryState{singleImageState(testManifestDigest, testLocalConfig)}}

	got, err := newTestWaiter(registry, amd64Image(testLocalConfig)).Wait(context.Background(), testRef)
	if err != nil {
		t.Fatal(err)
	}
	want := VerifiedImage{Reference: testRef, Digest: testManifestDigest, Verification: ImageVerificationMatched}
	if *got != want {
		t.Fatalf("want %+v, got %+v", want, *got)
	}
}

func TestImageWaiter_MatchedIndexSelectsPlatform(t *testing.T) {
	index, _ := json.Marshal(map[string]any{
		"mediaType": "application/vnd.oci.image.index.v1+json",
		"manifests": []map[string]any{
			{"digest": testAttestManifest, "platform": map[string]string{"os": "unknown", "architecture": "unknown"}},
			{"digest": testStaleManifest, "platform": map[string]string{"os": "linux", "architecture": "amd64"}},
			{"digest": testArmManifest, "platform": map[string]string{"os": "linux", "architecture": "arm64", "variant": "v8"}},
		},
	})
	registry := &fakeRegistry{states: []registryState{{
		descriptors: map[string]string{testRef: testIndexDigest},
		manifests: map[string]string{
			testRepo + "@" + testIndexDigest:   string(index),
			testRepo + "@" + testStaleManifest: imageManifestJSON(testStaleConfig),
			testRepo + "@" + testArmManifest:   imageManifestJSON(testLocalConfig),
		},
	}}}
	local := &localImage{ID: testLocalConfig, OS: "linux", Architecture: "arm64", Variant: "v8"}

	got, err := newTestWaiter(registry, local).Wait(context.Background(), testRef)
	if err != nil {
		t.Fatal(err)
	}
	if got.Digest != testIndexDigest || got.Verification != ImageVerificationMatched {
		t.Fatalf("want index digest reported as matched, got %+v", *got)
	}
}

func TestImageWaiter_MatchedContainerdStoreID(t *testing.T) {
	// with the containerd image store the local ID is the pushed manifest digest
	registry := &fakeRegistry{states: []registryState{singleImageState(testManifestDigest, testStaleConfig)}}

	got, err := newTestWaiter(registry, amd64Image(testManifestDigest)).Wait(context.Background(), testRef)
	if err != nil {
		t.Fatal(err)
	}
	if got.Verification != ImageVerificationMatched || got.Digest != testManifestDigest {
		t.Fatalf("unexpected result %+v", *got)
	}
}

func TestImageWaiter_PodmanIDWithoutPrefix(t *testing.T) {
	registry := &fakeRegistry{states: []registryState{singleImageState(testManifestDigest, testLocalConfig)}}
	podmanID := strings.TrimPrefix(testLocalConfig, "sha256:")

	got, err := newTestWaiter(registry, amd64Image(podmanID)).Wait(context.Background(), testRef)
	if err != nil {
		t.Fatal(err)
	}
	if got.Verification != ImageVerificationMatched {
		t.Fatalf("unexpected result %+v", *got)
	}
}

func TestImageWaiter_PollsUntilPushCompletes(t *testing.T) {
	registry := &fakeRegistry{states: []registryState{
		{}, // not pushed yet
		singleImageState(testStaleManifest, testStaleConfig), // previous image still under the tag
		singleImageState(testStaleManifest, testStaleConfig),
		singleImageState(testManifestDigest, testLocalConfig), // new push lands
	}}
	waiter := newTestWaiter(registry, amd64Image(testLocalConfig))
	waiter.timeout = 5 * time.Second

	got, err := waiter.Wait(context.Background(), testRef)
	if err != nil {
		t.Fatal(err)
	}
	if got.Digest != testManifestDigest || got.Verification != ImageVerificationMatched {
		t.Fatalf("want new image digest, got %+v", *got)
	}
	if registry.calls != 4 {
		t.Fatalf("want 4 registry checks, got %d", registry.calls)
	}
}

func TestImageWaiter_TimeoutOnStaleImage(t *testing.T) {
	registry := &fakeRegistry{states: []registryState{singleImageState(testStaleManifest, testStaleConfig)}}

	_, err := newTestWaiter(registry, amd64Image(testLocalConfig)).Wait(context.Background(), testRef)
	if err == nil {
		t.Fatal("want error when the registry never has the built image")
	}
	for _, want := range []string{testRef, "expected config digest " + testLocalConfig, "last seen " + testStaleConfig} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("want error containing %q, got: %v", want, err)
		}
	}
}

func TestImageWaiter_TimeoutNeverPushed(t *testing.T) {
	registry := &fakeRegistry{states: []registryState{{}}}

	_, err := newTestWaiter(registry, amd64Image(testLocalConfig)).Wait(context.Background(), testRef)
	if err == nil || !strings.Contains(err.Error(), "last seen none") || !strings.Contains(err.Error(), "manifest unknown") {
		t.Fatalf("want timeout error with last seen none, got %v", err)
	}
}

func TestImageWaiter_ExistsOnlyWithoutLocalImage(t *testing.T) {
	registry := &fakeRegistry{states: []registryState{singleImageState(testManifestDigest, testStaleConfig)}}

	got, err := newTestWaiter(registry, nil).Wait(context.Background(), testRef)
	if err != nil {
		t.Fatal(err)
	}
	want := VerifiedImage{Reference: testRef, Digest: testManifestDigest, Verification: ImageVerificationExistsOnly}
	if *got != want {
		t.Fatalf("want %+v, got %+v", want, *got)
	}
}

func TestImageWaiter_ExistsOnlyTimeout(t *testing.T) {
	registry := &fakeRegistry{states: []registryState{{}}}

	_, err := newTestWaiter(registry, nil).Wait(context.Background(), testRef)
	if err == nil || !strings.Contains(err.Error(), "was not found in the registry") || !strings.Contains(err.Error(), testRef) {
		t.Fatalf("want not found error, got %v", err)
	}
}

func TestImageWaiter_IndexWithoutLocalPlatform(t *testing.T) {
	index, _ := json.Marshal(map[string]any{
		"manifests": []map[string]any{
			{"digest": testArmManifest, "platform": map[string]string{"os": "linux", "architecture": "arm64"}},
		},
	})
	registry := &fakeRegistry{states: []registryState{{
		descriptors: map[string]string{testRef: testIndexDigest},
		manifests:   map[string]string{testRepo + "@" + testIndexDigest: string(index)},
	}}}

	_, err := newTestWaiter(registry, amd64Image(testLocalConfig)).Wait(context.Background(), testRef)
	if err == nil || !strings.Contains(err.Error(), "no manifest for platform linux/amd64") {
		t.Fatalf("want missing platform error, got %v", err)
	}
}

func TestImageWaiter_RequiresTag(t *testing.T) {
	if _, err := newTestWaiter(&fakeRegistry{}, nil).Wait(context.Background(), ""); err == nil {
		t.Fatal("want error without an image tag")
	}
}

func TestImageRepository(t *testing.T) {
	cases := map[string]string{
		"alpine":                                "alpine",
		"alpine:3.20":                           "alpine",
		"registry.example.com:5000/team/api":    "registry.example.com:5000/team/api",
		"registry.example.com:5000/team/api:v1": "registry.example.com:5000/team/api",
		"ghcr.io/org/app@sha256:abc":            "ghcr.io/org/app",
		"ghcr.io/org/app:1.0@sha256:abc":        "ghcr.io/org/app",
		"localhost:5000/app":                    "localhost:5000/app",
	}
	for in, want := range cases {
		if got := imageRepository(in); got != want {
			t.Errorf("imageRepository(%q) = %q, want %q", in, got, want)
		}
	}
}
