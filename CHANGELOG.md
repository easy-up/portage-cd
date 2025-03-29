# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Version header format: `## [x.x.x] - yyyy-mm-dd`

## [UNRELEASED]

## [v0.0.6] - 2025-03-29

* Updated gatecheck version to v0.9.2

## [v0.0.5] - 2025-02-14

* Updated gatecheck version to v0.9.1

## [v0.0.4] - 2025-02-08

* Updated golang dependencies and upgrade to OCAML v1.107.0

## [v0.0.3] - 2025-02-05

* Add exclusion for ClamAV CVE-2016-1405
* Fixed more instances of the issue with ' in commit messages
* Don't error when the default gatecheck config file does not exist.
* Misc docker build fixes.
* Upgrade gatecheck version
* Moved docker cache to the GitHub runner temp folder to avoid creating untracked files.
* Enable delivery pipeline on debug branches
* Added caching to the Dockerfile and to the github action config.
* Updated semgrep to v1.104 and fixed docker build issues
* Implemented deployment pipeline success webhook.
* Fixed justfile handling of ' in commit messages
* Add .env to gitignore
* fixed bug where the config flag did not supersede the default config file
* have portage use the provided gatecheck config rather than the copied over config in the /artifacts directory
* Leverage gatecheck submit functionality to send artifacts to Belay
* additional fixes to make .portage.yml the recommended way to configure portage.
* remediate some vulnerabilities found by semgrep and grype
* Move to loading the configuration once at the portage command level.  Also, a few lint formatting fixes
* Removed unfinished v1 CLI.
* Updated config generation with new parameters.

## [v0.0.2] - 2024-10-23

* Update gatecheck version to v0.8.0
* Added a default gatecheck configuration and added merge capabilities
* Added --coverage-file / codescan.coveragefile support
* updated old wfe filename
* Added a CLI argument and env variable to specify the target dir for semgrep
* Dockerfile: Disable opam init interactive prompt, and build using the release profile
* Increase OPAM timeout to decrease likelihood of intermittent docker build failure
* Fix issue with freshclam. Closes issue #3
* Ignore extraneous build output directory
* Standardized the configuration of the semgrep tool. Default to experimental in the docker image.
* Simplified artifact bundle publishing configuration
* Drop the --all flag from the gatecheck list command. Fixes Issue #5
* Added a list of the prerequisites for running portage locally
* Made the Dockerfile completely self-contained and bumped the versions of semgrep and gatecheck.
* Added justfile target for performing local docker builds.
* Fixed semgrep commandlines
* Enable build on pushes to the main branch and tags.
* Fixed portage-cd-actions repository name reference

## [v0.0.1-rc.17]

### Added

- GitHub action auth support
- settings package for config and metaconfig values
- settings package marshalling / unmarshalling
- Grype Image Scan Task
- Task pattern instead of pipeline pattern for simplicity Note: currently the new experimental is behind a build flag
- Image vulnerability scan task
- Image antivirus scan
- Image build
- Snyk SAST Scan

### Changed

- Fixed code scan stderr/stdout collision by moving the stdout dump to the end of the run function
- Fixed image scan stderr/stdout collision by moving the stdout dump to the end of the run function
- Fixed image build disable check
- Upgrade omnibus base image to v1.5.1
- Move existing CLI package to v0
- Task Run pattern

## [v0.0.0-rc.12]

### Changed

- refactored CLI for readability and maintenance
- upgraded to go 1.22.0
- New Executable will default inputs and outputs to OS
- WithStdin, WithStdout, WithStderr all merged to WithIO
- Config syntax to correlated with pipelines
- Code Scan Run organization to use functions for simplicity
- Add multi writer for Gatecheck list
- async run execution for image scan pipeline
- moved shell package to legacy
- image scan execution flow
- docker build argument strategy for shell
- shell command errors instead of exit codes
- shell command rich errors
- async task wraps stderr for cleaner log output
- Limit the number of supported Github action fields

### Added

- Configuration File template rendering with built-in values
- Configuration conversions
- Configuration init with the format option
- Semgrep, osemgrep, gitleaks shell commands
- Code Scan Pipeline
- Config Template auto rendering
- Version Command
- All commands will defer to viper for arguments and defaults
- no push flag to image publish
- Gatecheck Shell Command
- pipeline helper functions for common file operations
- Oras Command
- Deploy pipeline validation only (beta feature)
- clamscan & freshclam for virus scanning
- command run with context
- command run with IO
- grype CMD
- async task object
- "Combo" pipelines for image-delivery and all pipelines
- GitHub Actions Code Generation

### Fixed

- Viper config key names
- Specified CLI command parameters for custom input and output for easier unit testing in the future

## [0.0.1-rc.1] - 2024-01-29

### Changed

- refactored, the directory structure, all pipelines will exist in pkg/pipelines
- updated Version commands to return Commands instead of just an error
- simplified Command Methods
- converted some Command to private to prevent auto-complete overload
- the way command dry running is called, uses builder pattern now
- fixed a bug where only the last docker build flag was being added to the final command
- remove args from wrapper functions in CLI
- fixed the debug-pipeline calling syft scan

### Added

- debugging flag
- pkg/shell/commands Runner interface
- pkg/shell/commands Command struct
- pkg/shell for command wrappers
- grype version
- syft version
- podman version
- docker / podman support via CLI cmd interface
- docker info
- image build pipeline (info only)
- docker info, build, and push commands
- internal logger to image build pipeline
- json, yaml, toml meta tags for pipelines/config
- config parsing with viper
- grype scan sbom command
- image-scan pipeline
- syft scan image command
- syft to image-scan pipeline
- image scan pipeline to CLI
- image scan pipeline wiring in CLI for Viper config variables

### Added

- cmd/portage for cli
- pkg/environments
- pkg/jobs
- pkg/pipelines
- pkg/system
- initial project structure
