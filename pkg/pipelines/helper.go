package pipelines

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path"
	"portage/pkg/shell"
)

const (
	DefaultDirMode  = 0o755
	DefaultFileMode = 0o644
)

// OpenOverwrite is a file mode that will:
// 1. Create the file if it doesn't exist
// 2. Overwrite the content if it does
// 3. Write Only
const OpenOverwrite = os.O_CREATE | os.O_WRONLY | os.O_TRUNC

// MakeDirectoryP will create the directory, creating paths if neccessary with sensible defaults
//
// If the Directory already exists, it will return successfully as nil
func MakeDirectoryP(directoryName string) error {
	slog.Debug("make directory", "path", directoryName)
	return os.MkdirAll(directoryName, DefaultDirMode)
}

// OpenOrCreateFile will create the file or overwrite an existing file
func OpenOrCreateFile(filename string) (*os.File, error) {
	slog.Debug("create or open and overwrite existing file", "path", filename)
	return os.OpenFile(filename, OpenOverwrite, DefaultFileMode)
}

// Common Shell Commands to Functions

// InitGatecheckBundle will encode the config file to JSON and create a new bundle or add it to an existing one
//
// The stderr will be suppressed unless there is anG non-zero exit code
func InitGatecheckBundle(config *Config, stderr io.Writer, dryRunEnabled bool) error {
	tempConfigFilename := path.Join(os.TempDir(), "portage-config.json")

	tempFile, err := OpenOrCreateFile(tempConfigFilename)
	if err != nil {
		slog.Error("cannot create temp config file", "error", err)
		return err
	}

	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempConfigFilename)
	}()

	if err := json.NewEncoder(tempFile).Encode(config); err != nil {
		slog.Error("cannot encode temp config file", "error", err)
		return err
	}

	bundleFilename := path.Join(config.ArtifactDir, config.GatecheckBundleFilename)

	// Check if bundle exists before operations
	if _, err := os.Stat(bundleFilename); err == nil {
		slog.Debug("bundle file already exists before initialization", "bundle", bundleFilename)
	} else if errors.Is(err, os.ErrNotExist) {
		slog.Debug("bundle file does not exist before initialization", "bundle", bundleFilename)
	} else {
		slog.Error("error checking bundle file", "bundle", bundleFilename, "error", err)
	}

	return AddBundleFile(dryRunEnabled, bundleFilename, tempConfigFilename, stderr)
}

func AddBundleFile(dryRunEnabled bool, bundleFilename string, filename string, stderr io.Writer) error {
	slog.Debug("attempting to add file to bundle",
		"bundle", bundleFilename,
		"file", filename,
		"dry_run", dryRunEnabled)

	// Base options that are common for both create and add
	opts := []shell.OptionFunc{
		shell.WithDryRun(dryRunEnabled),
		shell.WithBundleFile(bundleFilename, filename),
	}

	// If we're in debug mode (verbose), show all output
	if slog.Default().Handler().Enabled(nil, slog.LevelDebug) {
		opts = append(opts, shell.WithStderr(stderr))
	} else {
		opts = append(opts, shell.WithErrorOnly(stderr))
	}

	if _, err := os.Stat(bundleFilename); err != nil {
		// The bundle file does not exist
		if errors.Is(err, os.ErrNotExist) {
			slog.Debug("bundle does not exist, creating new bundle")
			return shell.GatecheckBundleCreate(opts...)
		}
		return err
	}

	slog.Debug("bundle exists, adding file to existing bundle")
	return shell.GatecheckBundleAdd(opts...)
}
