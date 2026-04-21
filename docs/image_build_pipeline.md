# Image Build

Run the image build pipeline:

```
portage run image-build [flags]
```

The image build pipeline is responsible for producing the container image that downstream stages (image-scan, image-publish) operate on.

## Command Parameters

Config paths below use the YAML/JSON nested form. The flat (viper) key is the lowercase, dotted equivalent (e.g. `imageBuild.buildDir` ↔ `imagebuild.builddir`). All three — CLI flag, environment variable, and config file entry — reach the same setting, with CLI > env > file > default precedence.

### Image Tag

| CLI Flag | Environment Variable | Config Field        |
|----------|----------------------|---------------------|
| `--tag`  | `PORTAGE_IMAGE_TAG`  | `imageTag` (root)   |

The full image tag to apply to the built container (e.g. `registry.example.com/org/app:latest`). Top-level config field — not nested under `imageBuild`.

### Build Directory

| CLI Flag      | Environment Variable      | Config Field            |
|---------------|---------------------------|-------------------------|
| `--build-dir` | `PORTAGE_IMAGE_BUILD_DIR` | `imageBuild.buildDir`   |

The directory from which to build the container (typically where the Dockerfile lives). Optional. Defaults to the current working directory.

### Dockerfile

| CLI Flag       | Environment Variable              | Config Field               |
|----------------|-----------------------------------|----------------------------|
| `--dockerfile` | `PORTAGE_IMAGE_BUILD_DOCKERFILE`  | `imageBuild.dockerfile`    |

Path to the Dockerfile/Containerfile. Defaults to `Dockerfile`.

### Build Args

| CLI Flag      | Environment Variable       | Config Field         |
|---------------|----------------------------|----------------------|
| `--build-arg` | `PORTAGE_IMAGE_BUILD_ARGS` | `imageBuild.args`    |

Defines [build arguments](https://docs.docker.com/build/guide/build-args/) passed to the container build. Optional.

- **CLI:** pass `--build-arg key=value` repeatedly, once per arg.
- **Environment variable:** `PORTAGE_IMAGE_BUILD_ARGS` must contain a JSON-formatted object, e.g. `{"KEY":"value"}`.
- **Config file (YAML):** the value is a JSON-string, not a YAML object:

  ```yaml
  imageBuild:
    args: |-
      { "KEY": "value" }
  ```

- **Config file (JSON):** same idea — a string containing escaped JSON:

  ```json
  {
    "imageBuild": {
      "args": "{ \"KEY\": \"value\" }"
    }
  }
  ```

The string wrapping is required because portage's config loader lowercases all keys on deserialization, but Docker build args are case-sensitive. Passing the args as an opaque JSON string preserves casing.

### Platform

| CLI Flag     | Environment Variable          | Config Field           |
|--------------|-------------------------------|------------------------|
| `--platform` | `PORTAGE_IMAGE_BUILD_PLATFORM`| `imageBuild.platform`  |

Target platform for the build (e.g. `linux/amd64`, `linux/arm64`).

### Target

| CLI Flag   | Environment Variable         | Config Field         |
|------------|------------------------------|----------------------|
| `--target` | `PORTAGE_IMAGE_BUILD_TARGET` | `imageBuild.target`  |

For [multi-stage Dockerfiles](https://docs.docker.com/build/building/multi-stage/), names the stage to build.

### Cache To

| CLI Flag     | Environment Variable            | Config Field          |
|--------------|---------------------------------|-----------------------|
| `--cache-to` | `PORTAGE_IMAGE_BUILD_CACHE_TO`  | `imageBuild.cacheTo`  |

Where to export the build cache (e.g. `type=local,dest=path/to/dir`, `user/app:cache`).

### Cache From

| CLI Flag       | Environment Variable              | Config Field            |
|----------------|-----------------------------------|-------------------------|
| `--cache-from` | `PORTAGE_IMAGE_BUILD_CACHE_FROM`  | `imageBuild.cacheFrom`  |

Where to import existing build cache from.

### Squash Layers

| CLI Flag          | Environment Variable                | Config Field               |
|-------------------|-------------------------------------|----------------------------|
| `--squash-layers` | `PORTAGE_IMAGE_BUILD_SQUASH_LAYERS` | `imageBuild.squashLayers`  |

Squash image layers. Only supported with the Podman CLI.

### Use Buildx

| CLI Flag       | Environment Variable             | Config Field             |
|----------------|----------------------------------|--------------------------|
| `--use-buildx` | `PORTAGE_IMAGE_BUILD_USE_BUILDX` | `imageBuild.useBuildx`   |

Use Docker Buildx for the build. Only supported with the Docker CLI.
