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

Portage accepts three optional top-level values for identifying image bundles that belong to one logical build:

- `buildGroupId` is the identifier shared by every image pipeline in the build, commonly the CI pipeline or workflow-run ID.
- `imageName` is the stable registry path of the image represented by this Portage invocation, without a tag or digest.
- `buildImageNames` is the complete set of image names belonging to the logical build. Every sibling image pipeline must receive the same set. It is not a list of images processed so far.

For example, the API invocation in a two-image build can use:

```yaml
imageTag: registry.example.com/team/api:latest
imageName: registry.example.com/team/api
buildGroupId: pipeline-123
buildImageNames:
  - registry.example.com/team/api
  - registry.example.com/team/worker
```

The worker invocation uses its own `imageTag` and `imageName`, while retaining the same `buildGroupId` and `buildImageNames`. Portage passes these values unchanged to Gatecheck; it does not infer group membership from image tags.

When using environment variables, provide `PORTAGE_BUILD_IMAGE_NAMES` as a comma-separated list:

```shell
export PORTAGE_BUILD_IMAGE_NAMES="registry.example.com/team/api,registry.example.com/team/worker"
```

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
