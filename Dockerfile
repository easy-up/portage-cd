ARG ALPINE_VERSION=3.20

# Semgrep build is currently broken on alpine > 3.19
FROM alpine:3.19 AS build-semgrep-core

ARG SEMGREP_VERSION=v1.85.0

RUN apk add --no-cache bash build-base git make opam

RUN opam init --compiler=4.14.0 --disable-sandboxing
RUN opam switch 4.14.0

WORKDIR /src

RUN git clone --recurse-submodules --branch ${SEMGREP_VERSION} --depth=1 --single-branch https://github.com/semgrep/semgrep

WORKDIR /src/semgrep

# note that we do not run 'make install-deps-for-semgrep-core' here because it
# configures and builds ocaml-tree-sitter-core too; here we are
# just concerned about installing external packages to maximize docker caching.
RUN make install-deps-ALPINE-for-semgrep-core && \
    make install-opam-deps

RUN apk add --no-cache zstd libpsl-utils

RUN make install-deps-for-semgrep-core

RUN eval "$(opam env)" && \
    make minimal-build && \
    # Sanity check
    /src/semgrep/_build/default/src/main/Main.exe -version

FROM golang:alpine$ALPINE_VERSION AS build-prerequisites

ARG GRYPE_VERSION=v0.78.0
ARG SYFT_VERSION=v1.5.0
ARG GITLEAKS_VERSION=v8.18.3
ARG GATECHECK_VERSION=v0.7.6
ARG ORAS_VERSION=v1.2.0

RUN apk --no-cache add ca-certificates git make

WORKDIR /app

RUN git clone --branch ${GRYPE_VERSION} --depth=1 --single-branch https://github.com/anchore/grype /app/grype
RUN git clone --branch ${SYFT_VERSION} --depth=1 --single-branch https://github.com/anchore/syft /app/syft
RUN git clone --branch ${GITLEAKS_VERSION} --depth=1 --single-branch https://github.com/zricethezav/gitleaks /app/gitleaks
RUN git clone --branch ${GATECHECK_VERSION} --depth=1 --single-branch https://github.com/easy-up/gatecheck /app/gatecheck
RUN git clone --branch ${ORAS_VERSION} --depth=1 --single-branch https://github.com/oras-project/oras /app/oras

RUN cd /app/grype && \
    go build -ldflags="-w -s -extldflags '-static' -X 'main.version=${GRYPE_VERSION}' -X 'main.gitCommit=$(git rev-parse HEAD)' -X 'main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)' -X 'main.gitDescription=$(git log -1 --pretty=%B)'" -o /usr/local/bin ./cmd/grype

RUN cd /app/syft && \
    go build -ldflags="-w -s -extldflags '-static' -X 'main.version=${SYFT_VERSION}' -X 'main.gitCommit=$(git rev-parse HEAD)' -X 'main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)' -X 'main.gitDescription=$(git log -1 --pretty=%B)'" -o /usr/local/bin ./cmd/syft

RUN cd /app/gitleaks && \
    go build -ldflags="-s -w -X=github.com/zricethezav/gitleaks/v8/cmd.Version=${GITLEAKS_VERSION}" -o /usr/local/bin .

RUN cd /app/gatecheck && \
    go build -ldflags="-s -w -X 'main.cliVersion=$(git describe --tags)' -X 'main.gitCommit=$(git rev-parse HEAD)' -X 'main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)' -X 'main.gitDescription=$(git log -1 --pretty=%B)'" -o /usr/local/bin ./cmd/gatecheck

RUN cd /app/oras && \
    make build-linux-amd64 && \
    mv bin/linux/amd64/oras /usr/local/bin/oras

FROM golang:alpine$ALPINE_VERSION AS build

ARG VERSION
ARG GIT_COMMIT
ARG GIT_DESCRIPTION

# install build dependencies
RUN apk add --no-cache git

WORKDIR /app/src

COPY go.* .

# pre-fetch dependencies
RUN go mod download

COPY cmd ./cmd
COPY pkg ./pkg

RUN mkdir -p ../bin && \
    go build -ldflags="-X 'main.cliVersion=${VERSION}' -X 'main.gitCommit=${GIT_COMMIT}' -X 'main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)' -X 'main.gitDescription=${GIT_DESCRIPTION}'" -o ../bin/portage ./cmd/portage

FROM alpine:$ALPINE_VERSION AS portage-base

RUN apk --no-cache add git ca-certificates tzdata clamav

COPY --from=build-prerequisites /usr/local/bin/grype /usr/local/bin/grype
COPY --from=build-prerequisites /usr/local/bin/syft /usr/local/bin/syft
COPY --from=build-prerequisites /usr/local/bin/gitleaks /usr/local/bin/gitleaks
COPY --from=build-prerequisites /usr/local/bin/gatecheck /usr/local/bin/gatecheck
COPY --from=build-prerequisites /usr/local/bin/oras /usr/local/bin/oras
COPY --from=build-semgrep-core /src/semgrep/_build/default/src/main/Main.exe /usr/local/bin/osemgrep

COPY --from=build /app/bin/portage /usr/local/bin/portage

WORKDIR /app

ENV PORTAGE_CODE_SCAN_SEMGREP_EXPERIMENTAL="true"

ENTRYPOINT ["portage"]

LABEL org.opencontainers.image.title="portage-docker"
LABEL org.opencontainers.image.description="A standalone tool for secure, continuous delivery"
LABEL org.opencontainers.image.licenses="Apache-2.0"
LABEL io.artifacthub.package.readme-url="https://github.com/easy-up/portage-cd/blob/main/README.md"
LABEL io.artifacthub.package.license="Apache-2.0"

FROM portage-base AS portage-podman

# Install podman CLIs
RUN apk update && apk add --no-cache podman fuse-overlayfs

COPY docker/storage.conf /etc/containers/
COPY docker/containers.conf /etc/containers/

RUN addgroup -S podman && adduser -S podman -G podman && \
    echo podman:10000:5000 > /etc/subuid && \
    echo podman:10000:5000 > /etc/subgid

COPY docker/rootless-containers.conf /home/podman/.config/containers/containers.conf

RUN mkdir -p /home/podman/.local/share/containers
RUN chown podman:podman -R /home/podman

VOLUME /var/lib/containers
VOLUME /home/podman/.local/share/containers

RUN mkdir -p /var/lib/clamav
RUN chown podman /var/lib/clamav && chown podman /etc/clamav
RUN chmod g+w /var/lib/clamav

LABEL org.opencontainers.image.title="portage-podman"

FROM portage-base

# Install docker CLI
RUN apk update && apk add --no-cache docker-cli-buildx

LABEL org.opencontainers.image.title="portage-docker"
