# Portage CD

[![Build portage](https://github.com/easy-up/portage-cd/actions/workflows/delivery.yaml/badge.svg)](https://github.com/easy-up/portage-cd/actions/workflows/delivery.yaml)

![Portage CD Logo](./static/portage-cd-logo.svg)

Portage CD is a secure, continuous delivery pipeline designed to orchestrate the process of building and scanning an
application image for security vulnerabilities.
It solves the problem of having to configure a hardened-predefined security pipeline using traditional CI/CD.
Portage CD can be statically compiled as a binary and run on virtually any platform, CI/CD
environment, or locally.

## Getting Started

Install Prerequisites:

- Container Engine
- Docker or Podman CLI
- Golang >= v1.22.0
- [Just](https://github.com/casey/just?tab=readme-ov-file#installation) (optional)

Prerequisite Tools (For running `portage` local)

- Gitleaks
- Semgrep
- Syft
- Grype
- ClamAV (only the `clamscan` and `freshclam` cli utilities are needed)
- ORAS

## Compiling Portage CD

Running the just recipe will put the compiled-binary into `./bin`

```bash
just build
```

OR compile manually

```bash
git clone <this-repo> <target-dir>
cd <target-dir>
mkdir bin
go build -o bin/portage ./cmd/portage
```

Optionally, if you care to include metadata about the version of `portage` (displayed when you run `portage version`), use the following build arguments

```shell
go build -ldflags="-X 'main.cliVersion=$(git describe --tags)' -X 'main.gitCommit=$(git rev-parse HEAD)' -X 'main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)' -X 'main.gitDescription=$(git log -1 --pretty=%B | tr \' _)'" -o ./bin ./cmd/portage
```


## Getting Details of the Portage Pipeline Tooling

To view tooling details, run the following command:

```bash
portage run debug
```

## Configuring a Pipeline

Portage uses a number of configuration sources to determine the behavior of the pipeline.  The order of precedence is as follows:

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

The recommended way to configure portage is to create a `.portage.yml` file in the root of your project.  To generate a sample configuration file, run the following command:

```bash
portage config init .portage.yml
```

Note: `(none)` means unset, left blank

| Config Key                        | Environment Variable                     | Default Value                        | Description                                                                        |
| --------------------------------- |------------------------------------------| ------------------------------------ |------------------------------------------------------------------------------------|
| codescan.enabled                  | PORTAGE_CODE_SCAN_ENABLED                | 1                                    | Enable/Disable the code scan pipeline                                              |
| codescan.gitleaksfilename         | PORTAGE_CODE_SCAN_GITLEAKS_FILENAME      | gitleaks-secrets-report.json         | The filename for the gitleaks secret report - must contain 'gitleaks'              |
| codescan.gitleakssrcdir           | PORTAGE_CODE_SCAN_GITLEAKS_SRC_DIR       | .                                    | The target directory for the gitleaks scan                                         |
| codescan.semgrepfilename          | PORTAGE_CODE_SCAN_SEMGREP_FILENAME       | semgrep-sast-report.json             | The filename for the semgrep SAST report - must contain 'semgrep'                  |
| codescan.semgreprules             | PORTAGE_CODE_SCAN_SEMGREP_RULES          | p/default                            | Semgrep ruleset manual override                                                    |
| codescan.semgrepexperimental      | PORTAGE_CODE_SCAN_SEMGREP_EXPERIMENTAL   | false                                | Enable the use of the semgrep experimental CLI                                     |
| deploy.enabled                    | PORTAGE_IMAGE_PUBLISH_ENABLED            | 1                                    | Enable/Disable the publishing to a registry pipeline                                                 |
| deploy.gatecheckconfigfilename    | PORTAGE_DEPLOY_GATECHECK_CONFIG_FILENAME | .gatecheck.yml                                    | The filename for the gatecheck config                                              |
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

The portage pipeline is broken into a number of stages.  Below are the stages and their purpose:

- `code-scan`: Scans the application code for secrets
- `image-build`: Builds the application image
- `image-scan`: Scans the application image for vulnerabilities
- `image-publish`: Publishes the application image to a registry

Note you cannot run image scan or publish without running the image build stage.

## Running in Docker

When running portage in a docker container there are some pipelines that need to run docker commands.
In order for the docker CLI in the portage to connect to the docker daemon running on the host machine,
you must either mount the `/var/run/docker.sock` in the `portage` container, or provide configuration for
accessing the docker daemon remotely with the `DOCKER_HOST` environment variable.

If you don't have access to Artifactory to pull in the Omnibus base image, you can build the image manually which is
in `images/omnibus/Dockerfile`.

### Using `/var/run/docker.sock`

This approach assumes you have the docker daemon running on your host machine.

Example:

```bash
docker run -it --rm \
  `# Mount your Dockerfile and supporting files in the working directory: /app` \
  -v "$(pwd):/app:rw" \
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
  -v "$(pwd):/app:rw" \
  `# Run the portage container with the desired arguments` \
  portage run image-build -i podman
```

If root access is needed, the easiest solution for using podman inside a docker container is to run the container in "privileged" mode:

```bash
docker run -it --rm \
  `# Mount your Dockerfile and supporting files in the working directory: /app` \
  -v "$(pwd):/app:rw" \
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
  -v "$(pwd):/app:rw" \
  `# Run the container with additional security options so that podman is fully functional` \
  --security-opt label=disable --device /dev/fuse \
  `# Run the portage container with the desired arguments` \
  portage run image-build -i podman
```

### Including your Global `.gitignore`

If you use a global gitignore for files created by your IDE or editor, there may be scenarios where these files need to be ignored inside the Portage CD container as well.
In order to have your global `.gitignore` applied inside the container you must take some additional steps.

In order for a global gitignore to be effective inside the container it needs to be mounted in the container in the configured location:

```bash
docker run -it --rm \
  `# Mount your local gitignore as the global gitignore inside the container` \
  -v "$(git config core.excludesfile):/home/portage/.gitignore_global:ro" \
  `# Mount your Dockerfile and supporting files in the working directory: /app` \
  -v "$(pwd):/app:rw" \
  `# Mount docker.sock for use by the docker CLI running inside the container` \
  -v "/var/run/docker.sock:/var/run/docker.sock" \
  `# Run the portage container with the desired arguments` \
  portage run image-build
```

When using the podman container, the same technique can be used, but the home directory will be `/home/podman` instead of `/home/portage`.
