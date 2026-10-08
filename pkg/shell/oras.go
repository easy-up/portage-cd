package shell

import "os/exec"

// OrasVersion prints version of ORAS CLI
//
// Requirements: N/A
//
// Output: version to STDOUT
func OrasVersion(options ...OptionFunc) error {
	o := newOptions(options...)
	exe := exec.Command("oras", "version")
	return run(exe, o)
}

// OrasPushBundle push a gatecheck bundle
//
// Requirements: WithArtifactBundle
//
// Output: debug information to STDERR
func OrasPushBundle(options ...OptionFunc) error {
	o := newOptions(options...)
	exe := exec.Command(
		"oras",
		"push",
		"--disable-path-validation",
		"--artifact-type",
		"application/vnd.gatecheckdev.gatecheck.bundle.tar+gzip",
		o.bundleTag,
		o.gatecheck.bundleFilename,
	)
	return run(exe, o)
}

// OrasManifestDescriptor prints the registry descriptor (mediaType, digest, size) of an image reference
//
// Uses the registry credentials in the docker config (e.g. from docker login)
//
// Requirements: WithImageTag
//
// Output: descriptor JSON to STDOUT
func OrasManifestDescriptor(options ...OptionFunc) error {
	o := newOptions(options...)
	exe := exec.Command("oras", "manifest", "fetch", "--descriptor", o.imageTag)
	return run(exe, o)
}

// OrasManifestFetch prints the raw manifest or index of an image reference
//
// Requirements: WithImageTag
//
// Output: manifest JSON to STDOUT
func OrasManifestFetch(options ...OptionFunc) error {
	o := newOptions(options...)
	exe := exec.Command("oras", "manifest", "fetch", o.imageTag)
	return run(exe, o)
}
