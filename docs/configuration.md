# Portage CD CLI Configuration

The Portage CD CLI provides a set of commands to manage the configuration of your portage.
These commands allow you to initialize, list variables, render, and convert configuration files in various formats.

This documentation provides a comprehensive overview of the configuration management capabilities available in the
Portage CD CLI.
For further assistance or more detailed examples, refer to the CLI's help command or the official documentation.

## Configuring Using Environment Variables, CLI Arguments, or Configuration Files

The Portage CD supports flexible configuration methods to suit various operational environments.
You can configure the engine using environment variables, command-line (CLI) arguments, or configuration files in JSON,
YAML, or TOML formats.
This flexibility allows you to choose the most convenient way to set up your portage based on your deployment
and development needs.

### Configuration Precedence

The Portage CD uses Viper under the hood to manage its configurations, which follows a specific order of
precedence when merging configuration options:

1. **Command-line Arguments**: These override values specified through other methods.
2. **Environment Variables**: They take precedence over configuration files.
3. **Configuration Files**: Supports JSON, YAML, and TOML formats. The engine reads these files if specified and merges
   them into the existing configuration.
4. **Default Values**: Predefined in the code.

### Multi-image build context

The CI orchestrator owns logical-build identity. It generates one opaque ID once per logical build and gives every image job all three values below:

- `buildGroupId` / `PORTAGE_BUILD_GROUP_ID`: the same opaque ID for every image in the logical build.
- `imageName` / `PORTAGE_IMAGE_NAME`: the stable registry path for the image handled by the current job, without a tag or digest.
- `buildImageNames` / `PORTAGE_BUILD_IMAGE_NAMES`: the complete image-name set for the logical build, not the images completed so far. Every job receives the same set.

Portage transports this context to Gatecheck unchanged. It does not generate an ID, parse CI metadata, infer membership from image tags, or add missing image names. Gatecheck serializes the context without inference, and Belay consumes it to correlate the bundles. Treat `buildGroupId` as opaque: uniqueness and retry semantics belong to the CI configuration.

Grouped mode requires all three values together. Supply `buildGroupId`, `imageName`, and `buildImageNames` for every image job, or omit all three for legacy ungrouped behavior. Partial grouping context is invalid and must not be used. Legacy bundles cannot participate in grouped multi-image replacement.

#### Portable environment contract

For a four-image build, each parallel job receives the same group ID and comma-delimited list. Only `PORTAGE_IMAGE_NAME` and the image-specific tag change:

```shell
export PORTAGE_BUILD_GROUP_ID=ci:project-42:build-781
export PORTAGE_BUILD_IMAGE_NAMES=registry.example.com/team/api,registry.example.com/team/web,registry.example.com/team/worker,registry.example.com/team/migrations

# API job; other jobs select their own name from the same complete set.
export PORTAGE_IMAGE_NAME=registry.example.com/team/api
export PORTAGE_IMAGE_TAG=registry.example.com/team/api:${GIT_COMMIT_SHA}
portage run all
```

`PORTAGE_BUILD_IMAGE_NAMES` is a plain comma-delimited list without spaces or quoting. Portage splits the value literally on commas; it does not trim whitespace or implement quoted-field syntax. Image names must not contain commas.

```text
Safe:   registry.example.com/team/api,registry.example.com/team/web
Unsafe: registry.example.com/team/api, registry.example.com/team/web
Unsafe: "registry.example.com/team/api,registry.example.com/team/web"
```

In the first unsafe value, the second image name begins with a space. In the second, quote characters are part of the parsed image names when passed literally. Avoid both forms.

The equivalent YAML config for one of those jobs is:

```yaml
buildGroupId: ci:project-42:build-781
imageName: registry.example.com/team/api
buildImageNames:
  - registry.example.com/team/api
  - registry.example.com/team/web
  - registry.example.com/team/worker
  - registry.example.com/team/migrations
imageTag: registry.example.com/team/api:abc1234
```

#### GitHub Actions identity

Use the repository ID, workflow run ID, and run attempt:

```yaml
env:
  PORTAGE_BUILD_GROUP_ID: github:${{ github.repository_id }}:${{ github.run_id }}:${{ github.run_attempt }}
  PORTAGE_BUILD_IMAGE_NAMES: registry.example.com/team/api,registry.example.com/team/web,registry.example.com/team/worker,registry.example.com/team/migrations
```

A full workflow rerun increments `github.run_attempt`, creating a new grouped attempt. Do not rerun only failed jobs when the intent is to replace a grouped build: successful sibling jobs would not publish bundles under the new attempt. Start a full rerun so all four bundles share the new ID.

#### GitLab CI/CD identity

Use the project and pipeline IDs:

```yaml
variables:
  PORTAGE_BUILD_GROUP_ID: gitlab:${CI_PROJECT_ID}:${CI_PIPELINE_ID}
  PORTAGE_BUILD_IMAGE_NAMES: registry.example.com/team/api,registry.example.com/team/web,registry.example.com/team/worker,registry.example.com/team/migrations

portage-images:
  parallel:
    matrix:
      - PORTAGE_IMAGE_NAME:
          - registry.example.com/team/api
          - registry.example.com/team/web
          - registry.example.com/team/worker
          - registry.example.com/team/migrations
  script:
    - export PORTAGE_IMAGE_TAG="${PORTAGE_IMAGE_NAME}:${CI_COMMIT_SHA}"
    - portage run all
```

All jobs in one pipeline share `CI_PIPELINE_ID`, including parallel or matrix jobs. Retrying an individual job keeps the same pipeline ID and therefore the same logical group. With Belay's current duplicate-artifact semantics, do not use an individual GitLab job retry to create a grouped replacement. Start a new full pipeline so every image is republished under a new `CI_PIPELINE_ID`.

GitLab documents the [predefined CI/CD variables](https://docs.gitlab.com/ci/variables/predefined_variables/) and the behavior of [retrying jobs](https://docs.gitlab.com/ci/jobs/#retry-jobs).

For parent/child pipelines, do not assume the child pipeline's `CI_PIPELINE_ID` is the root identity. Construct the group ID in the root pipeline and pass it explicitly to every child pipeline so all image jobs retain one shared value. GitLab describes variable forwarding in [downstream pipelines](https://docs.gitlab.com/ci/pipelines/downstream_pipelines/).

#### Other CI systems

Use a provider-qualified project identity plus the provider's logical pipeline/run ID, for example `jenkins:payments:build-781` or `circleci:project-42:workflow-uuid`. The exact format is yours; it only needs to be opaque, stable across all parallel image jobs, and new for a new full grouped attempt. Do not use a per-job ID, timestamp generated independently in each job, image tag, commit SHA alone, or mutable branch name.

### Redacting vulnerability IDs in CI logs

| Config key | Environment variable | Default |
|---|---|---|
| `redactCveIds` | `PORTAGE_REDACT_CVE_IDS` | `false` |

When enabled, portage runs every gatecheck command with `--redact-cve-ids`. Vulnerability IDs (CVE, GHSA and common advisory formats) are replaced with `[redacted]` in gatecheck's logs, at every level including `--verbose`, and in the findings tables printed after the scans. Table rows, packages and severities are kept. Report files, the gatecheck bundle and what is sent to deploy webhooks are unchanged.

The flag is passed explicitly, so a gatecheck build without redaction support fails the step instead of printing IDs. Requires gatecheck with `--redact-cve-ids`.

### Deploy validation mode and remote policy

By default (`enforce`), `portage deploy` stops before the deploy webhooks when `gatecheck validate` fails. When a webhook receiver such as a deployment gate makes the final decision, set report mode so every build reaches it:

| Config key | Environment variable | Default |
|---|---|---|
| `deploy.validation` | `PORTAGE_DEPLOY_VALIDATION` | `enforce` |
| `deploy.policyUrl` | `PORTAGE_DEPLOY_POLICY_URL` | unset |
| `deploy.policyAuthVar` | `PORTAGE_DEPLOY_POLICY_AUTH_VAR` | see below |

**`deploy.validation`**

- `enforce`: unchanged behaviour. A validation failure fails the step and no webhook is sent.
- `report`: `gatecheck validate` still runs and prints its results, but a validation failure (gatecheck exit code 1) is logged as a warning and the webhooks are still invoked. Other errors (missing bundle, gatecheck system errors) still fail the step. Each webhook request gets two extra form fields:
  - `validation`: `passed` or `failed`
  - `policy`: `fetched` (from `deploy.policyUrl`) or `local` (from the repository/default config)
- Any other value is a configuration error.

**`deploy.policyUrl`**

When set, the gatecheck config is downloaded with `gatecheck config fetch` and used as-is for validation and for the `gatecheck-config` entry in the bundle. Local configs (`deploy.gatecheckConfigFilename`, `.gatecheck.yml`) are **ignored, not merged**, so a permissive local file cannot loosen the downloaded limits.

There is **no fallback**: if the policy cannot be downloaded or is not a valid gatecheck config, the deploy step fails in both modes and no webhook is sent. A pipeline never silently validates against a different risk posture than the one it was configured for.

Let CI build the URL from its own variables, for example in GitLab:

```yaml
variables:
  PORTAGE_DEPLOY_VALIDATION: report
  PORTAGE_DEPLOY_POLICY_URL: "https://belay-api.example.com/Policy/$CI_PROJECT_NAMESPACE/$CI_PROJECT_NAME/gatecheck?branch=$CI_COMMIT_REF_NAME"
```

**`deploy.policyAuthVar`** is the *name* of the environment variable holding the credential. Only the name is passed to gatecheck, so the value never appears on a command line. If unset, portage uses `PORTAGE_DEPLOY_WEBHOOK_AUTH_HEADER` when it is set, otherwise the first webhook's `authorizationVar`. A bare token is sent as `Bearer <token>`; a value that already includes a scheme is sent unchanged. The policy URL is logged without its query string.

Requires a gatecheck version with `gatecheck config fetch`.

### Deploy webhook verdict

A deploy webhook can return its deployment decision, and portage prints it in the CI log. Receivers that return no verdict work exactly as before.

**Response format.** Respond with `Content-Type: application/json` (or any `+json` type):

```json
{
  "verdict": {
    "decision": "fail",
    "summary": "Deployment blocked",
    "reasons": [
      { "rule": "grype.critical", "status": "fail", "message": "3 critical (limit 0)" },
      { "rule": "grype.high", "status": "accepted", "message": "4 high (limit 2), 2 covered by approved POA&M" },
      { "rule": "sbom.required", "status": "pass", "message": "SBOM present" }
    ],
    "detailsUrl": "https://gate.example.com/builds/1234"
  }
}
```

| Field | Required | Values |
|---|---|---|
| `verdict.decision` | yes | `pass`, `fail`, `pending` (case-insensitive). Any other value is treated as no verdict. |
| `verdict.summary` | no | One line of text. |
| `verdict.reasons[].rule` | no | Rule identifier. |
| `verdict.reasons[].status` | no | `pass`, `fail`, `accepted` (allowed by a risk acceptance). Other values print as `[-]`. |
| `verdict.reasons[].message` | no | One line, e.g. counts and limits. |
| `verdict.detailsUrl` | no | Link printed without its query string. |

Return HTTP 2xx for a received submission whatever the decision; non-2xx means the submission itself failed (the verdict is still printed if present). Text is printed as single lines with control characters removed and each value capped at 300 characters; at most 50 reasons are shown. Keep messages to counts and limits: anything in them appears in CI logs.

Example log output:

```
Deploy verdict: FAIL - Deployment blocked
  [fail]     grype.critical  3 critical (limit 0)
  [accepted] grype.high      4 high (limit 2), 2 covered by approved POA&M
  [pass]     sbom.required   SBOM present
  Details: https://gate.example.com/builds/1234
```

| Config key | Environment variable | Default |
|---|---|---|
| `deploy.failOnVerdict` | `PORTAGE_DEPLOY_FAIL_ON_VERDICT` | `false` |

By default a `fail` verdict is printed and logged as a warning, and the step still succeeds. With `deploy.failOnVerdict: true` the step fails after all webhooks have been called if any returned `fail`. `pass` and `pending` never fail the step.

Webhook URLs are logged without their query string, and the authorization value is never logged.

### Waiting for the published image before deploy

When the image push runs as a separate CI step, `portage deploy` can submit to the deploy webhooks before the new image is in the registry, and the target deploys whatever the tag pointed to before. Enable `deploy.waitForImage` to hold the webhooks until the image is confirmed. It is off by default; with it off, deploy behaves exactly as before.

| Config key | Environment variable | Default |
|---|---|---|
| `deploy.waitForImage` | `PORTAGE_DEPLOY_WAIT_FOR_IMAGE` | `false` |
| `deploy.waitForImageTimeout` | `PORTAGE_DEPLOY_WAIT_FOR_IMAGE_TIMEOUT` | `10m` |
| `deploy.waitForImagePollInterval` | `PORTAGE_DEPLOY_WAIT_FOR_IMAGE_POLL_INTERVAL` | `15s` |

The image checked is `imageTag` (`PORTAGE_IMAGE_TAG`). Registry lookups use `oras` with the CI job's own docker credentials (e.g. from `docker login`); the deploy webhook receiver never needs registry access.

Before any webhook is sent, portage:

1. Inspects the local image for `imageTag` with `docker` or `podman` (per `--cli-interface`).
2. Polls the registry every poll interval until the timeout:
   - **Local image available:** waits until the registry image *is* the local image: its config digest (selecting the local platform from a multi-arch index) or manifest/index digest equals the local image ID. The bundle records `imageVerification: matched`.
   - **No local image** (e.g. image build disabled): waits until the tag exists, logs a warning, and records `imageVerification: exists-only`.
3. If the timeout expires, the deploy step fails with a message naming the image, the expected config digest and the last one seen. **No webhook is sent.**

When verification succeeds, the gatecheck bundle manifest records:

```json
{
  "build": {
    "publishedImage": "ghcr.io/acme/api:3f9c2a1b",
    "imageDigest": "sha256:<registry manifest digest>",
    "imageVerification": "matched"
  }
}
```

`imageDigest` is the registry manifest (or index) digest, so a deploy target can pin to `<image>@<imageDigest>`. All three fields are omitted when the feature is off.

#### Tag guidance

- **Reused tags** (`latest`, or the branch-name tag in `delivery.yml`) are only safe on the `matched` path. With `exists-only`, a reused tag that still points at the previous image passes immediately.
- **Images built outside portage** (e.g. a separate build-and-push action): use a unique per-build tag such as the commit SHA, or have the deploy target pin by digest. If portage also builds the image locally but a different job rebuilds and pushes it, the two builds normally produce different config digests, so `matched` never succeeds and the step fails at the timeout. In that setup either push the image portage built, or turn off portage's image build so verification runs as `exists-only` against a unique tag.
- `--dry-run` skips the wait.

### Using Environment Variables

Environment variables are a convenient way to configure the application in environments where file access might be
restricted or for overriding specific configurations without changing the configuration files.

To use environment variables:

- Portage CD environment variables are prefixed with `PORTAGE_` to avoid conflicts with other applications (e.g. `PORTAGE_IMAGE_BUILD_DIR`, `PORTAGE_CODE_SCAN_ENABLED`).
- See the full list of supported variables in the [project README](../README.md).

### Using CLI Arguments

CLI arguments provide a way to specify configuration values when running a command.
They are useful for temporary overrides or when scripting actions.
For each configuration option, there is usually a corresponding flag that can be passed to the command.

For example:

```shell
./portage run image-build --build-dir . --dockerfile custom.Dockerfile
```

### Using Configuration Files

Configuration files offer a structured and human-readable way to manage your application settings.
The Portage CD supports JSON, YAML, and TOML formats, allowing you to choose the one that best fits your
preferences or existing infrastructure.

- [JSON](https://www.json.org/json-en.html): A lightweight data-interchange format.
- [YAML](https://yaml.org/): A human-readable data serialization standard. 
- [TOML](https://toml.io/en/):A minimal configuration file format that's easy to read due to its clear semantics.

To specify which configuration file to use, you can typically pass the file path as a CLI argument or set an
environment variable pointing to the file.

### Merging Configuration

Portage CD merges configuration from different sources in the order of precedence mentioned above.
If the same configuration is specified in multiple places, the source with the highest precedence overrides the others.
This mechanism allows for flexible configuration strategies, such as defining default values in a file and overriding
them with environment variables or CLI arguments as needed.

## Commands - Managing the configuration file

### `config init`

Initializes the configuration file with default settings.

### `config vars`

Lists supported built-in variables that can be used in templates.

### `config render`

Renders a configuration template using the `--file` flag or STDIN and writes the output to STDOUT.

### `config convert`

Converts a configuration file from one format to another.

## Examples

### Render Configuration Template

Rendering a configuration template from `config.json.tmpl` to JSON format:

```shell
$ cat config.json.tmpl | ./portage config render
```

**Output**:

```json
{
  "image": {...},
  "artifacts": {...}
}
```

### Convert Configuration Format

Attempting to convert the configuration without specifying required flags results in an error:

```shell
$ cat config.json.tmpl | ./portage config render  | ./portage config convert
```

**Error Output**:

```shell
Error: at least one of the flags in the group [file input] is required
```

Successful conversion from JSON to TOML format:

```shell
$ cat config.json.tmpl | ./portage config render  | ./portage config convert -i json -o toml
```

**Output**:

```toml
[image]
buildDir = '.'
...
```
