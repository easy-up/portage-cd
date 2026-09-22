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
