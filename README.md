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

Note: `(none)` means unset, left blank

| Configuration Option              | Environment Variable                   | Default Value                        | Description                                                                      |
|-----------------------------------|----------------------------------------|--------------------------------------|----------------------------------------------------------------------------------|
| imagescan.grypefilename           | PORTAGE_IMAGE_SCAN_GRYPE_FILENAME      | grype-vulnerability-report-full.json | The filename for the grype vulnerability report - must contain 'grype'           |
| imagescan.syftfilename            | PORTAGE_IMAGE_SCAN_SYFT_FILENAME       | syft-sbom-report.json                | The filename for the syft SBOM report - must contain 'syft'                      |

To run a pipeline with all the defaults, run the following command.
```bash
portage run all --tag "ttl.sh/$(uuidgen | tr '[:upper:]' '[:lower:]'):1h"
```


For Example if you want to run portage scans only and not build the container nor deploy it, you can run the following command.

```bash
portage run all --tag "ttl.sh/$(uuidgen | tr '[:upper:]' '[:lower:]'):1h" --imagebuild.enabled 0 --deploy.enabled 0
```


## Running a Pipeline in Docker

The latest Portage Docker container can be found here:

```
TBD
```

When running portage in a docker container there are some pipelines that need to run docker commands.
In order for the docker CLI in the portage to connect to the docker daemon running on the host machine,


### Run the portage container with the desired arguments
Using a .portage config files
```bash
portage run image-build
```

```yaml
imageTag: user/image-name
imagePublish:
  enabled: false
codeScan:
  coverageFile: coverage/cobertura-coverage.xml
deploy:
  gatecheckConfigFilename: .gatecheck.yml
```

Using flags

```bash
docker run -it --rm -v "$(pwd):/app:rw" -v "/var/run/docker.sock:/var/run/docker.sock:rw" ghcr.io/easy-up/portage:v0.0.1-rc.19 run all --tag "ttl.sh/$(uuidgen | tr [:upper:] [:lower:]):1h"
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
