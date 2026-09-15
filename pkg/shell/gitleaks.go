package shell

import "os/exec"

// GitLeaksVersion prints version of GitLeaks CLI
//
// Requirements: N/A
//
// Output: version to STDOUT
func GitLeaksVersion(options ...OptionFunc) error {
	o := newOptions(options...)
	exe := exec.Command("gitleaks", "version")
	return run(exe, o)
}

// GitLeaksDetect prints version of GitLeaks CLI
//
// Requirements: WithGitleaks
//
// Output: debug to STDERR
func GitLeaksDetect(options ...OptionFunc) error {
	o := newOptions(options...)
	exe := exec.Command("gitleaks",
		"detect",
		"--exit-code",
		"0",
		"--verbose",
		// Redact matched secrets in the report/logs. The gitleaks report is bundled to
		// Belay and uploaded as a CI artifact, so it must never carry the plaintext
		// secret — findings still include rule/file/line/commit, enough to locate and
		// rotate. Bare --redact = 100% (works across gitleaks v8; v8.19+ treats it as
		// the default percentage).
		"--redact",
		"--source",
		o.gitleaks.targetDirectory,
		"--report-path",
		o.gitleaks.reportPath,
	)
	return run(exe, o)
}
