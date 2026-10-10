package pipelines

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"portage/pkg/shell"
	"strings"
	"time"
)

// Values recorded in the bundle manifest as build.imageVerification
const (
	ImageVerificationMatched    = "matched"
	ImageVerificationExistsOnly = "exists-only"
)

const (
	defaultWaitForImageTimeout      = 10 * time.Minute
	defaultWaitForImagePollInterval = 15 * time.Second
)

// VerifiedImage is the registry image confirmed before submitting to deploy webhooks
type VerifiedImage struct {
	// Reference is the image reference that was checked (the configured image tag)
	Reference string
	// Digest is the registry manifest digest of Reference, suitable for pinning
	Digest string
	// Verification is ImageVerificationMatched or ImageVerificationExistsOnly
	Verification string
}

type ociDescriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
	Platform  *struct {
		OS           string `json:"os"`
		Architecture string `json:"architecture"`
		Variant      string `json:"variant"`
	} `json:"platform,omitempty"`
}

type ociManifest struct {
	MediaType string          `json:"mediaType"`
	Config    *ociDescriptor  `json:"config,omitempty"`
	Manifests []ociDescriptor `json:"manifests,omitempty"`
}

// localImage is the subset of `docker image inspect` used to identify the built image
type localImage struct {
	ID           string `json:"Id"`
	OS           string `json:"Os"`
	Architecture string `json:"Architecture"`
	Variant      string `json:"Variant"`
}

type imageRegistry interface {
	// Descriptor returns the registry descriptor for ref
	Descriptor(ctx context.Context, ref string) (ociDescriptor, error)
	// Manifest returns the raw manifest or index for ref
	Manifest(ctx context.Context, ref string) ([]byte, error)
}

type localImageStore interface {
	// Inspect returns the local image for ref, or an error if it is not available
	Inspect(ctx context.Context, ref string) (*localImage, error)
}

type imageWaiter struct {
	registry     imageRegistry
	local        localImageStore
	timeout      time.Duration
	pollInterval time.Duration
	sleep        func(ctx context.Context, d time.Duration) error
}

// Wait polls the registry until ref exists and, when the image is available locally, until the
// registry image is the local image. It returns an error naming the image, the expected config
// digest and the last one seen if the bounded wait expires.
func (w *imageWaiter) Wait(ctx context.Context, ref string) (*VerifiedImage, error) {
	if ref == "" {
		return nil, errors.New("no image tag configured to verify")
	}

	expected, err := w.local.Inspect(ctx, ref)
	if err != nil {
		slog.Warn("built image is not available locally, the registry image can only be checked for existence",
			"image", ref, "reason", err)
		expected = nil
	} else {
		expected.ID = normalizeDigest(expected.ID)
		slog.Info("waiting for the built image to be published", "image", ref, "expected_config_digest", expected.ID,
			"platform", platformString(expected.OS, expected.Architecture, expected.Variant),
			"timeout", w.timeout.String(), "poll_interval", w.pollInterval.String())
	}

	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	var lastErr error
	lastSeen := ""
	for attempt := 1; ; attempt++ {
		verified, seen, err := w.check(ctx, ref, expected)
		if err == nil {
			slog.Info("published image verified", "image", ref, "digest", verified.Digest,
				"verification", verified.Verification, "attempts", attempt)
			return verified, nil
		}
		lastErr = err
		if seen != "" {
			lastSeen = seen
		}
		slog.Info("published image not ready", "image", ref, "attempt", attempt, "reason", err)

		if sleepErr := w.sleep(ctx, w.pollInterval); sleepErr != nil {
			break
		}
	}

	if expected != nil {
		if lastSeen == "" {
			lastSeen = "none"
		}
		return nil, fmt.Errorf("image %s was not verified within %s: expected config digest %s, last seen %s (last error: %v)",
			ref, w.timeout, expected.ID, lastSeen, lastErr)
	}
	return nil, fmt.Errorf("image %s was not found in the registry within %s (last error: %v)", ref, w.timeout, lastErr)
}

// check performs one registry lookup, seen is the registry config digest when one was read
func (w *imageWaiter) check(ctx context.Context, ref string, expected *localImage) (*VerifiedImage, string, error) {
	desc, err := w.registry.Descriptor(ctx, ref)
	if err != nil {
		return nil, "", err
	}
	if !strings.HasPrefix(desc.Digest, "sha256:") {
		return nil, "", fmt.Errorf("registry returned an unexpected digest %q", desc.Digest)
	}

	if expected == nil {
		return &VerifiedImage{Reference: ref, Digest: desc.Digest, Verification: ImageVerificationExistsOnly}, "", nil
	}

	// With the containerd image store the local ID is the manifest or index digest
	if expected.ID == desc.Digest {
		return &VerifiedImage{Reference: ref, Digest: desc.Digest, Verification: ImageVerificationMatched}, desc.Digest, nil
	}

	manifestDigest, configDigest, err := w.platformConfig(ctx, ref, desc.Digest, expected)
	if err != nil {
		return nil, "", err
	}
	if expected.ID == configDigest || expected.ID == manifestDigest {
		return &VerifiedImage{Reference: ref, Digest: desc.Digest, Verification: ImageVerificationMatched}, configDigest, nil
	}

	return nil, configDigest, fmt.Errorf("registry config digest %s does not match the local image %s", configDigest, expected.ID)
}

// platformConfig resolves the manifest and config digest matching the local image platform
func (w *imageWaiter) platformConfig(ctx context.Context, ref string, digest string, expected *localImage) (string, string, error) {
	repository := imageRepository(ref)
	manifest, err := w.fetchManifest(ctx, repository+"@"+digest)
	if err != nil {
		return "", "", err
	}

	if len(manifest.Manifests) > 0 {
		child, ok := selectPlatform(manifest.Manifests, expected)
		if !ok {
			return "", "", fmt.Errorf("registry index has no manifest for platform %s",
				platformString(expected.OS, expected.Architecture, expected.Variant))
		}
		digest = child.Digest
		manifest, err = w.fetchManifest(ctx, repository+"@"+digest)
		if err != nil {
			return "", "", err
		}
	}

	if manifest.Config == nil || manifest.Config.Digest == "" {
		return "", "", errors.New("registry manifest has no config descriptor")
	}
	return digest, manifest.Config.Digest, nil
}

func (w *imageWaiter) fetchManifest(ctx context.Context, ref string) (*ociManifest, error) {
	content, err := w.registry.Manifest(ctx, ref)
	if err != nil {
		return nil, err
	}
	manifest := new(ociManifest)
	if err := json.Unmarshal(content, manifest); err != nil {
		return nil, fmt.Errorf("cannot decode registry manifest: %w", err)
	}
	return manifest, nil
}

func selectPlatform(manifests []ociDescriptor, expected *localImage) (ociDescriptor, bool) {
	for _, m := range manifests {
		if m.Platform == nil || m.Platform.OS != expected.OS || m.Platform.Architecture != expected.Architecture {
			continue
		}
		if expected.Variant != "" && m.Platform.Variant != "" && m.Platform.Variant != expected.Variant {
			continue
		}
		return m, true
	}
	return ociDescriptor{}, false
}

// imageRepository strips the tag and digest from an image reference
func imageRepository(ref string) string {
	if i := strings.Index(ref, "@"); i >= 0 {
		ref = ref[:i]
	}
	lastSlash := strings.LastIndex(ref, "/")
	if i := strings.LastIndex(ref, ":"); i > lastSlash {
		ref = ref[:i]
	}
	return ref
}

// normalizeDigest adds the sha256: prefix podman omits from image IDs
func normalizeDigest(id string) string {
	if id != "" && !strings.Contains(id, ":") {
		return "sha256:" + id
	}
	return id
}

func platformString(os, arch, variant string) string {
	if variant != "" {
		return os + "/" + arch + "/" + variant
	}
	return os + "/" + arch
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// orasRegistry uses the oras CLI and the CI's docker credentials to read from the registry
type orasRegistry struct{}

func (orasRegistry) Descriptor(ctx context.Context, ref string) (ociDescriptor, error) {
	stdout, err := captureShell(ctx, shell.OrasManifestDescriptor, shell.WithImageTag(ref))
	if err != nil {
		return ociDescriptor{}, err
	}
	var desc ociDescriptor
	if err := json.Unmarshal(stdout, &desc); err != nil {
		return ociDescriptor{}, fmt.Errorf("cannot decode registry descriptor: %w", err)
	}
	return desc, nil
}

func (orasRegistry) Manifest(ctx context.Context, ref string) ([]byte, error) {
	return captureShell(ctx, shell.OrasManifestFetch, shell.WithImageTag(ref))
}

// cliImageStore reads the local image from docker or podman
type cliImageStore struct {
	alias shell.DockerAlias
}

func (s cliImageStore) Inspect(ctx context.Context, ref string) (*localImage, error) {
	stdout, err := captureShell(ctx, shell.DockerImageInspect, shell.WithImageTag(ref), shell.WithDockerAlias(s.alias))
	if err != nil {
		return nil, err
	}
	var images []localImage
	if err := json.Unmarshal(stdout, &images); err != nil {
		return nil, fmt.Errorf("cannot decode image inspect output: %w", err)
	}
	if len(images) == 0 || images[0].ID == "" {
		return nil, errors.New("image inspect returned no image")
	}
	return &images[0], nil
}

// captureShell runs a shell command and returns STDOUT, STDERR is folded into the error
func captureShell(ctx context.Context, command shell.Command, options ...shell.OptionFunc) ([]byte, error) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	options = append(options, shell.WithCtx(ctx), shell.WithStdout(stdout), shell.WithStderr(stderr))
	if err := command(options...); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("%w: %s", err, lastLine(msg))
		}
		return nil, err
	}
	return stdout.Bytes(), nil
}

func lastLine(s string) string {
	if i := strings.LastIndex(s, "\n"); i >= 0 {
		return strings.TrimSpace(s[i+1:])
	}
	return s
}
