package cli

import (
	"log/slog"
	"portage/pkg/pipelines"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	AppMetadata ApplicationMetadata
	AppLogLever *slog.LevelVar
)

func NewPortageCommand() *cobra.Command {
	viper.SetConfigName(".portage")
	viper.SetConfigType("yml")
	viper.AddConfigPath(".")
	pipelines.BindViper(viper.GetViper())

	// Load default config
	config := new(pipelines.Config)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			slog.Info("no config file specified for this command, using defaults if applicable")
		} else {
			slog.Error("error reading config file", "error", err)
		}
	} else {
		slog.Info("using config file", "path", viper.ConfigFileUsed())
	}

	if err := viper.Unmarshal(config); err != nil {
		slog.Error("failed to parse config", "error", err)
	}

	// Debug log the full config
	slog.Debug("loaded configuration",
		"version", config.Version,
		"imageTag", config.ImageTag,
		"artifactDir", config.ArtifactDir,
		"gatecheckBundleFilename", config.GatecheckBundleFilename,
		"imageBuild", config.ImageBuild,
		"imageScan", config.ImageScan,
		"codeScan", config.CodeScan,
		"imagePublish", config.ImagePublish,
		"deploy", config.Deploy)

	versionCmd := newBasicCommand("version", "print version information", runVersion)
	cmd := &cobra.Command{
		Use:              "portage",
		Short:            "A portable, opinionated security pipeline",
		PersistentPreRun: runCheckLoggingFlags,
	}

	// Create log leveling flags
	cmd.PersistentFlags().BoolP("verbose", "v", false, "verbose logging output")
	cmd.PersistentFlags().BoolP("silent", "q", false, "only log errors")
	cmd.MarkFlagsMutuallyExclusive("verbose", "silent")

	// Turn off usage after an error occurs which polutes the terminal
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	// Add Sub-commands
	cmd.AddCommand(newConfigCommand(), newRunCommand(), versionCmd)

	return cmd
}

func runCheckLoggingFlags(cmd *cobra.Command, _ []string) {
	verboseFlag, _ := cmd.Flags().GetBool("verbose")
	silentFlag, _ := cmd.Flags().GetBool("silent")

	switch {
	case verboseFlag:
		AppLogLever.Set(slog.LevelDebug)
	case silentFlag:
		AppLogLever.Set(slog.LevelError)
	}
}

// portage version
func runVersion(cmd *cobra.Command, args []string) error {
	versionFlag, _ := cmd.Flags().GetBool("version")
	switch {
	case versionFlag:
		cmd.Println(AppMetadata.CLIVersion)
		return nil
	default:
		_, err := AppMetadata.WriteTo(cmd.OutOrStdout())
		return err
	}
}
