INSTALL_DIR := env('INSTALL_DIR', '/usr/local/bin')
IMAGE_NAME := "ghcr.io/easy-up/portage"

# build portage binary
build:
    mkdir -p bin
    go build -ldflags="-X 'main.cliVersion=$(git describe --tags)' -X 'main.gitCommit=$(git rev-parse HEAD)' -X 'main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)' -X 'main.gitDescription=$(git log -1 --pretty=%B | tr \' _)'" -o ./bin ./cmd/portage

# build docker image for local use
docker-build-local:
    if ! git diff-index --quiet HEAD; then \
      echo "WARNING: uncommitted changes incorporated into local docker image" >&2; \
    fi
    docker build . --build-arg VERSION=local-$(git show --no-patch HEAD --format='%h') --build-arg GIT_COMMIT=$(git rev-parse HEAD) --build-arg GIT_DESCRIPTION="$(git show --no-patch HEAD --format='%s')" -t "{{IMAGE_NAME}}:local-$(git show --no-patch HEAD --format='%h')"

# build and install binary
install: build
    cp ./bin/portage {{ INSTALL_DIR }}/portage

# golangci-lint view only
lint:
    golangci-lint run --fast

# golangci-lint fix linting errors and format if possible
fix:
    golangci-lint run --fast --fix

upgrade:
    git status --porcelain | grep -q . && echo "Repository is dirty, commit changes before upgrading." && exit 1 || exit 0
    go get -u ./...
    go mod tidy

# Locally serve documentation
serve-docs:
  mdbook serve
