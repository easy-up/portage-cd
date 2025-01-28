package shell

import (
	"log/slog"
	"os/exec"
)

// GatecheckVersion print version information
//
// Requirement: N/A
//
// Output: to STDOUT
func GatecheckVersion(options ...OptionFunc) error {
	o := newOptions(options...)
	args := []string{"version"}
	if o.logger.Handler().Enabled(nil, slog.LevelDebug) {
		args = append(args, "-v")
	}
	cmd := exec.Command("gatecheck", args...)
	return run(cmd, o)
}

// GatecheckList will print a summarized view of a a report
//
// Requirement: supported report from STDIN WithReportType
//
// Output: table to STDOUT
func GatecheckList(options ...OptionFunc) error {
	o := newOptions(options...)
	args := []string{"list", "--input-type", o.reportType}
	if o.logger.Handler().Enabled(nil, slog.LevelDebug) {
		args = append(args, "-v")
	}
	if o.listTargetFilename != "" {
		cmd := exec.Command("gatecheck", "list", o.listTargetFilename)
		return run(cmd, o)
	}
	cmd := exec.Command("gatecheck", args...)
	return run(cmd, o)
}

// GatecheckListAll will print a summarized view of a a report with EPSS scores
//
// Requirement: supported report from STDIN
//
// Output: table to STDOUT
func GatecheckListAll(options ...OptionFunc) error {
	o := newOptions(options...)
	args := []string{"list", "--input-type", o.reportType}
	if o.logger.Handler().Enabled(nil, slog.LevelDebug) {
		args = append(args, "-v")
	}
	cmd := exec.Command("gatecheck", args...)
	return run(cmd, o)
}

// GatecheckBundleAdd add a file to an existing bundle
//
// Requirement: WithBundleFile
//
// Output: debug to STDERR
func GatecheckBundleAdd(options ...OptionFunc) error {
	o := newOptions(options...)
	args := []string{"bundle", "add", o.gatecheck.bundleFilename, o.gatecheck.targetFile}
	if len(o.gatecheck.tags) > 0 {
		for _, tag := range o.gatecheck.tags {
			args = append(args, "--tag", tag)
		}
	}
	if o.logger.Handler().Enabled(nil, slog.LevelDebug) {
		args = append(args, "-v")
	}
	cmd := exec.Command("gatecheck", args...)
	return run(cmd, o)
}

// GatecheckBundleCreate new bundle and add a file
//
// Requirement: WithBundleFile
//
// Output: debug to STDERR
func GatecheckBundleCreate(options ...OptionFunc) error {
	o := newOptions(options...)
	args := []string{"bundle", "create", o.gatecheck.bundleFilename, o.gatecheck.targetFile}
	if len(o.gatecheck.tags) > 0 {
		for _, tag := range o.gatecheck.tags {
			args = append(args, "--tag", tag)
		}
	}
	if o.logger.Handler().Enabled(nil, slog.LevelDebug) {
		slog.Debug("debug logging enabled, adding verbose flag to gatecheck command")
		args = append(args, "-v")
	}
	slog.Debug("executing gatecheck bundle create",
		"bundle", o.gatecheck.bundleFilename,
		"target", o.gatecheck.targetFile,
		"tags", o.gatecheck.tags,
		"args", args)
	cmd := exec.Command("gatecheck", args...)
	return run(cmd, o)
}

// GatecheckValidate validates artifacts in a bundle
//
// Requirement: WithTargetFilename
//
// Output: debug to STDERR
func GatecheckValidate(options ...OptionFunc) error {
	o := newOptions(options...)
	args := []string{"validate", o.targetFilename}
	if o.configFilename != "" {
		args = append(args, "--config", o.configFilename)
	}
	// Pass through verbosity flag if portage is in verbose mode
	if o.logger.Handler().Enabled(nil, slog.LevelDebug) {
		args = append(args, "-v")
	}
	cmd := exec.Command("gatecheck", args...)
	return run(cmd, o)
}
