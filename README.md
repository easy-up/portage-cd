# Portage CD

[![Build portage](https://github.com/easy-up/portage-cd/actions/workflows/delivery.yaml/badge.svg)](https://github.com/easy-up/portage-cd/actions/workflows/delivery.yaml)

![Portage CD Logo](./static/portage-cd-logo.svg)

## Basic Overview

Portage CD is a secure, continuous delivery pipeline built on open source.  Portage CD is designed to orchestrate the process of building and scanning an application image for security vulnerabilities.  The unique aspect of Portage CD is that it is meant to be portable, meaning that a developer can run the entire pipeline locally, address any security vulnerabilities or code issues before pushing before pushing a branch to a CI/CD pipeline based in the cloud that is also running Portage CD. 

This project aims to simplify the CI/CD build process such that developers can focus on building their unit and end to end tests, and spend less time working on security pipeline tooling, and standing up continuous delivery (CD).

Portage CD is designed to make CI/CD easier by providing a preconfigured security and container building pipeline that can be configured such that tools can be overridden where an enterprise SaaS tool may exist. The tool has also been designed so that i can be incorporated into existing CI pipelines running in any CI platform.  Portage CD can be statically compiled as a binary and run on virtually any environment or CI/CD platform.

At it's core, Portage CD is a golang based CLI tool that is configured to chain the security and container building open source tools below to make the CI/CD process easier.

## Getting Started

NOTE: *While Portage CD will easily work with any Windows or Linux environment, the following instructions are done on a MacOS PC environment.  In the future we will update this Readme with instructions for other environments.*

Install Prerequisites:

- Container Engine (e.g. [Docker](https://docs.docker.com/desktop/), Openshift, etc.)
- [Docker](https://docs.docker.com/desktop/) or Podman CLI
- Golang >= v1.22.0 ([brew install](https://formulae.brew.sh/formula/go))
- [Just](https://github.com/casey/just?tab=readme-ov-file#installation) (optional)

```bash
brew install go@1.22 just
```

Prerequisite Tools (For running `portage` local)

- Gitleaks
- Semgrep
- Syft
- Grype
- ClamAV (only the `clamscan` and `freshclam` cli utilities are needed)
- ORAS

If you are running on a mac and have brew installed, you can install these dependencies using the following command.

```bash
brew install gitleaks semgrep syft grype clamav oras
```

Once ClamAV is installed, you will need to configure it to download the virus definitions.  This can be done by adding the following lines to your `/etc/freshclam.conf` file. 

```
# Allow freshclam to run as root
DatabaseMirror database.clamav.net
Foreground yes
```

Freschclam will download and run the latest virus definitions every time you run it.  WARNING: This will generate a lot of network traffic.  Please be aware that if you pull too many times, you will be rate limited by the virus definition servers. 

Run the following command to download the virus definitions.

```bash
freshclam
```

## Compiling Portage CD for Local Use

Clone the repo and navigate to the directory in the shell.

```bash
git clone <this-repo> <target-dir>
cd <target-dir>
mkdir bin
```

Running the just recipe will put the compiled-binary into `./bin`

```bash
just build
```

OR you can compile Portage CD manually

```bash
go build -ldflags="-X 'main.cliVersion=$(git describe --tags)' -X 'main.gitCommit=$(git rev-parse HEAD)' -X 'main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)' -X 'main.gitDescription=$(git log -1 --pretty=%B)'" -o ./bin ./cmd/portage
```

## Running A Pipeline Locally

First, map the portage binary to your $PATH via ~/.zshrc or copy the binary into your /usr/local/bin.

```bash
echo 'export PATH="$PATH:/path/to/portage/bin"' >> ~/.zshrc
source ~/.zshrc
```
OR
```bash
sudo cp /path/to/portage/bin/portage /usr/local/bin/
```

Then navigate to the source code directory that you wish to run the pipeline scanning tools on.  To run the security pipeline directly, run the following command.

```bash
portage run debug
```

### Configuring a Pipeline

Configuration Options:

- Configuration via CLI flags
- Environment Variables
- Config File in JSON
- Config File in YAML
- Config File in TOML

Configuration Order-of-Precedence:

1. CLI Flag
2. Environment Variable
3. Config File Value
4. Default Value

Note: `(none)` means unset, left blank

| Config Key                        | Environment Variable                     | Default Value                        | Description                                                                        |
| --------------------------------- |------------------------------------------| ------------------------------------ |------------------------------------------------------------------------------------|
| codescan.enabled                  | PORTAGE_CODE_SCAN_ENABLED                | 1                                    | Enable/Disable the code scan pipeline                                              |
| codescan.gitleaksfilename         | PORTAGE_CODE_SCAN_GITLEAKS_FILENAME      | gitleaks-secrets-report.json         | The filename for the gitleaks secret report - must contain 'gitleaks'              |
| codescan.gitleakssrcdir           | PORTAGE_CODE_SCAN_GITLEAKS_SRC_DIR       | .                                    | The target directory for the gitleaks scan                                         |
| codescan.semgrepfilename          | PORTAGE_CODE_SCAN_SEMGREP_FILENAME       | semgrep-sast-report.json             | The filename for the semgrep SAST report - must contain 'semgrep'                  |
| codescan.semgreprules             | PORTAGE_CODE_SCAN_SEMGREP_RULES          | p/default                            | Semgrep ruleset manual override                                                    |
| codescan.semgrepexperimental      | PORTAGE_CODE_SCAN_SEMGREP_EXPERIMENTAL   | false                                | Enable the use of the semgrep experimental CLI                                     |
| deploy.enabled                    | PORTAGE_IMAGE_PUBLISH_ENABLED            | 1                                    | Enable/Disable the deploy pipeline                                                 |
| deploy.gatecheckconfigfilename    | PORTAGE_DEPLOY_GATECHECK_CONFIG_FILENAME | -                                    | The filename for the gatecheck config                                              |
| gatecheckbundlefilename           | PORTAGE_GATECHECK_BUNDLE_FILENAME        | artifacts/gatecheck-bundle.tar.gz    | The filename for the gatecheck bundle, a validatable archive of security artifacts |
| imagebuild.args                   | PORTAGE_IMAGE_BUILD_ARGS                 | -                                    | Comma seperated list of build time variables                                       |
| imagebuild.builddir               | PORTAGE_IMAGE_BUILD_DIR                  | .                                    | The build directory to using during an image build                                 |
| imagebuild.cachefrom              | PORTAGE_IMAGE_BUILD_CACHE_FROM           | -                                    | External cache sources (e.g., "user/app:cache", "type=local,src=path/to/dir")      |
| imagebuild.cacheto                | PORTAGE_IMAGE_BUILD_CACHE_TO             | -                                    | Cache export destinations (e.g., "user/app:cache", "type=local,src=path/to/dir")   |
| imagebuild.dockerfile             | PORTAGE_IMAGE_BUILD_DOCKERFILE           | Dockerfile                           | The Dockerfile/Containerfile to use during an image build                          |
| imagebuild.enabled                | PORTAGE_IMAGE_BUILD_ENABLED              | 1                                    | Enable/Disable the image build pipeline                                            |
| imagebuild.platform               | PORTAGE_IMAGE_BUILD_PLATFORM             | -                                    | The target platform for build                                                      |
| imagebuild.squashlayers           | PORTAGE_IMAGE_BUILD_SQUASH_LAYERS        | 0                                    | squash image layers - Only Supported with Podman CLI                               |
| imagebuild.target                 | PORTAGE_IMAGE_BUILD_TARGET               | -                                    | The target build stage to build (e.g., [linux/amd64])                              |
| imagepublish.bundletag            | PORTAGE_IMAGE_PUBLISH_BUNDLE_TAG         |                                      | The full image tag for the target gatecheck bundle image blob                      |
| imagepublish.enabled              | PORTAGE_IMAGE_PUBLISH_ENABLED            | 1                                    | Enable/Disable the image publish pipeline                                          |
| imagescan.clamavfilename          | PORTAGE_IMAGE_SCAN_CLAMAV_FILENAME       | clamav-virus-report.txt              | The filename for the clamscan virus report - must contain 'clamav'                 |
| imagescan.enabled                 | PORTAGE_IMAGE_SCAN_ENABLED               | 1                                    | Enable/Disable the image scan pipeline                                             |
| imagescan.grypeconfigfilename     | PORTAGE_IMAGE_SCAN_GRYPE_CONFIG_FILENAME | -                                    | The config filename for the grype vulnerability report                             |
| imagescan.grypefilename           | PORTAGE_IMAGE_SCAN_GRYPE_FILENAME        | grype-vulnerability-report-full.json | The filename for the grype vulnerability report - must contain 'grype'             |
| imagescan.syftfilename            | PORTAGE_IMAGE_SCAN_SYFT_FILENAME         | syft-sbom-report.json                | The filename for the syft SBOM report - must contain 'syft'                        |

To run a pipeline with all the defaults, run the following command.
```bash
portage run all --tag "ttl.sh/$(uuidgen | tr '[:upper:]' '[:lower:]'):1h"
```


For Example if you want to run portage scans only and not build the container nor deploy it, you can run the following command.

```bash
portage run -imagebuild.enabled 0 -deploy.enabled 0
```


## Running a Pipeline in Docker

The latest Portage Docker container can be found here:

```
TBD
```

When running portage in a docker container there are some pipelines that need to run docker commands.
In order for the docker CLI in the portage to connect to the docker daemon running on the host machine,
you must either mount the `/var/run/docker.sock` in the `portage` container, or provide configuration for
accessing the docker daemon remotely with the `DOCKER_HOST` environment variable.

If you don't have access to Artifactory to pull in the Omnibus base image, you can build the image manually which is
in `images/omnibus/Dockerfile`.

### Using `/var/run/docker.sock`

This approach assumes you have the docker daemon running on your host machine.

Example:

```
docker run -it --rm \
  `# Mount your Dockerfile and supporting files in the working directory: /app` \
  -v "$(pwd):/app:ro" \
  `# Mount docker.sock for use by the docker CLI running inside the container` \
  -v "/var/run/docker.sock:/var/run/docker.sock" \
  `# Run the portage container with the desired arguments` \
  portage run image-build
```

### Using a Remote Daemon

For more information see the
[Docker CLI](https://docs.docker.com/engine/reference/commandline/cli/#environment-variables) and
[Docker Daemon](https://docs.docker.com/config/daemon/remote-access/) documentation pages.

### Using Podman in Docker

In addition to building images with Docker it is also possible to build them with podman. When running podman in docker it is necessary to either launch the container in privileged mode, or to run as the `podman` user:

```bash
docker run --user podman -it --rm \
  `# Mount your Dockerfile and supporting files in the working directory: /app` \
  -v "$(pwd):/app:ro" \
  `# Run the portage container with the desired arguments` \
  portage run image-build -i podman
```

If root access is needed, the easiest solution for using podman inside a docker container is to run the container in "privileged" mode:

```bash
docker run -it --rm \
  `# Mount your Dockerfile and supporting files in the working directory: /app` \
  -v "$(pwd):/app:ro" \
  `# Run the container in privileged mode so that podman is fully functional` \
  --privileged \
  `# Run the portage container with the desired arguments` \
  portage run image-build -i podman
```

### Using Podman in Podman

To run the portage container using podman the process is quite similar, but there are a few additional security options required:

```bash
podman run --user podman  -it --rm \
  `# Mount your Dockerfile and supporting files in the working directory: /app` \
  -v "$(pwd):/app:ro" \
  `# Run the container with additional security options so that podman is fully functional` \
  --security-opt label=disable --device /dev/fuse \
  `# Run the portage container with the desired arguments` \
  portage run image-build -i podman
```
