# Troubleshooting

## Grype: "vulnerability database was built N weeks ago"

**Symptom:** Image-scan pipeline fails (or warns) with a message like:
```
the vulnerability database was built 26 weeks ago (max allowed age is 5 days)
```

**Cause:** Anchore deprecated the legacy DBv5 feed in early 2025. Grype versions older than ~v0.80 still point at that frozen feed and will receive a stale DB no matter how often they refresh — "no update available" from `grype db update` doesn't mean the DB is current, it means the server has nothing newer to give.

**Fix (portage-cd image):** the Dockerfile now pins a modern grype (v0.110.0+) that uses the current DBv6 feed. Modern grype **warns and auto-updates** the DB mid-scan instead of hard-failing, so this situation self-heals on a fresh container.

**Fix (local Homebrew grype on macOS):** if you see this running portage against locally installed binaries, run `grype db status` to check the DB path and schema. The cache lives at `~/Library/Caches/grype/db/<schema>/`. A truly stale cache may need `grype db delete` followed by `grype db update`.

## GitLab CI: "client version 1.52 is too new. Maximum supported API version is 1.40"

**Symptom:** The first `docker login` step in a GitLab job fails with the above error when using the portage container image.

**Cause:** The portage image ships a modern Docker CLI (Alpine 3.23, Docker 27+, API v1.47+). Older dind service images like `docker:stable-dind` are frozen on Docker 19.03 (API v1.40 max) and can't talk to newer clients.

**Fix:** In `.gitlab-ci.yml`, pin the dind service to a modern major:
```yaml
services:
  - docker:27-dind
```
Using a pinned major (`docker:27-dind`, `docker:28-dind`) is better than the floating `docker:dind` because it avoids surprise breakage when upstream publishes a new major.

## Portage runs docker with literal `${PORTAGE_IMAGE_TAG}` as the tag

**Symptom:**
```
ERROR: failed to build: invalid tag "${PORTAGE_IMAGE_TAG}": invalid reference format
```
or the same pattern on any other `${VAR}` placeholder in a value reaching a shell command.

**Cause:** Portage's config loader does not perform environment-variable substitution on string values in YAML/JSON/TOML files. When a config file contains `imageTag: "${PORTAGE_IMAGE_TAG}"` and the env var is unset, viper reads the literal string and passes it straight through.

In CI, this usually "works" because the CI system sets `PORTAGE_IMAGE_TAG` and viper's env-var override wins over the config file entry — the literal value in the YAML is never actually read. Locally, the env var is often unset and you see the literal.

**Fix:** Set the env var before running portage:
```bash
export PORTAGE_IMAGE_TAG="myrepo/myapp:local"
portage run all
```
Or use a separate config file for local runs with a hardcoded tag (e.g. `.portage-local.yml`) and pass it via `--config .portage-local.yml`.

## Portage self-build fails on webhook POST to a dead URL

**Symptom (in portage-cd's own `delivery.yaml`):**
```
WRN authorization environment variable is empty envVar=DEPLOY_WEBHOOK_AUTH_HEADER
ERR failed to execute HTTP request error="Post ...: dial tcp: lookup ... no such host"
```

**Cause:** A `successWebhooks` block in portage-cd's own `.portage.yml` combined with `deploy_enabled: true` in the workflow causes the self-build to try to POST validation results to a stale webhook URL.

**Fix:** Portage-cd's own `.portage.yml` should **not** contain a `successWebhooks` block — the self-build has no business notifying downstream consumers. The webhook feature itself lives in `pkg/pipelines/deploy.go` and is unaffected; downstream consumers like `belay-portage-gitlab-example-app` configure `successWebhooks` in *their own* `.portage.yml`.

## First CI build after Dockerfile change takes ~40 min

**Symptom:** The `Portage Image Build` GitHub Actions workflow takes 30–40 minutes on the first build after a Dockerfile change.

**Cause:** The Dockerfile builds `semgrep-core` from OCaml source. Cold-cache, this involves OPAM init, OCaml 5.3.0 switch creation, full dependency solve, and compilation — all expensive.

**Expected:** Warm-cache builds are dramatically faster thanks to buildx cache mounts (`--mount=type=cache,id=opam-...`) plus GitHub's `actions/cache@v5` persistence of `${{ runner.temp }}/.docker-cache`. After the first expensive bootstrap, subsequent builds that don't invalidate the OCaml layers typically land in 10–15 minutes.

**If consistently slow:** check that the Docker cache is being restored between runs. The `Load Docker Cache` step in `delivery.yaml` should report a cache hit on non-first runs.

## Mac M1 Docker Container Execution Failure

If you are running on a Mac M1, and are getting an error similar to:

```
ERR execution failure error="input:1: container.from.withEnvVariable.withExec.stdout process \"echo sample output from debug container\" did not complete successfully: exit code: 1\n\nStdout:\n\nStderr:\n"
```

You may need to install [colima](https://github.com/abiosoft/colima).

To install colima on a Mac using Homebrew:

```
brew install colima
```

Start colima:

```
colima start --arch x86_64
```

Then go ahead and run portage.
