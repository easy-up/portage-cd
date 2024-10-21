package shell

import "os/exec"

// SemgrepVersion prints version of Semgrep CLI
//
// Requirements: N/A
//
// Output: version to STDOUT
func SemgrepVersion(options ...OptionFunc) error {
	o := newOptions(options...)
	exe := exec.Command("semgrep", "--version")
	if o.semgrep.experimental {
		exe = exec.Command("osemgrep", "--help")
	}
	return run(exe, o)
}

// SemgrepScan runs a Semgrep scan against target artifact dir from env vars
//
// Requirements: WithSemgrep
//
// Output: JSON report to STDOUT
func SemgrepScan(options ...OptionFunc) error {
	o := newOptions(options...)
	exe := exec.Command("semgrep", "scan", "--json", "--config", o.semgrep.rules, o.semgrep.targetDirectory)
	if o.semgrep.experimental {
		exe = exec.Command("osemgrep", "scan", "--json", "--experimental", "--config", o.semgrep.rules, o.semgrep.targetDirectory)
	}
	return run(exe, o)
}
