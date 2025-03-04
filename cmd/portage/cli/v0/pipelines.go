package cli

import (
	"io"
	"log/slog"
	"os"
	"portage/pkg/pipelines"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newRunCommand() *cobra.Command {
	// Create Flags, Bind Flags, Bind Environment Variables
	debugCmd := newBasicCommand("debug", "pipeline for smoke testing this application", runDebug)

	// run image-build
	imageBuildCmd := newBasicCommand("image-build", "builds an image", runImageBuild)

	imageBuildCmd.Flags().String("build-dir", "", "image build context directory")
	_ = viper.BindPFlag("imagebuild.builddir", imageBuildCmd.Flags().Lookup("build-dir"))

	imageBuildCmd.Flags().String("dockerfile", "", "image build custom Dockerfile")
	_ = viper.BindPFlag("imagebuild.dockerfile", imageBuildCmd.Flags().Lookup("dockerfile"))

	imageBuildCmd.Flags().StringArray("build-arg", make([]string, 0), "A build argument passed to the container build command")
	_ = viper.BindPFlag("imagebuild.args", imageBuildCmd.Flags().Lookup("build-arg"))

	imageBuildCmd.Flags().String("platform", "", "image build custom platform option")
	_ = viper.BindPFlag("imagebuild.platform", imageBuildCmd.Flags().Lookup("platform"))

	imageBuildCmd.Flags().String("target", "", "image build custom target option")
	_ = viper.BindPFlag("imagebuild.target", imageBuildCmd.Flags().Lookup("target"))

	imageBuildCmd.Flags().String("cache-to", "", "image build custom cache-to option")
	_ = viper.BindPFlag("imagebuild.cacheto", imageBuildCmd.Flags().Lookup("cache-to"))

	imageBuildCmd.Flags().String("cache-from", "", "image build custom cache-from option")
	_ = viper.BindPFlag("imagebuild.cachefrom", imageBuildCmd.Flags().Lookup("cache-from"))

	imageBuildCmd.Flags().Bool("squash-layers", false, "image build squash all layers into one option")
	_ = viper.BindPFlag("imagebuild.squashlayers", imageBuildCmd.Flags().Lookup("squash-layers"))

	// run image-scan
	imageScanCmd := newBasicCommand("image-scan", "run security scans on an image", runImageScan)

	imageScanCmd.Flags().String("sbom-filename", "", "the output filename for the syft SBOM")
	_ = viper.BindPFlag("imagescan.syftfilename", imageScanCmd.Flags().Lookup("sbom-filename"))

	imageScanCmd.Flags().String("grype-filename", "", "the output filename for the grype vulnerability report")
	_ = viper.BindPFlag("imagescan.grypefilename", imageScanCmd.Flags().Lookup("grype-filename"))

	imageScanCmd.Flags().String("clamav-filename", "", "the output filename for the ClamAV scan report")
	_ = viper.BindPFlag("imagescan.clamavfilename", imageScanCmd.Flags().Lookup("clamav-filename"))

	// run image-publish
	imagePublishCmd := newBasicCommand("image-publish", "publishes an image", runimagePublish)

	imagePublishCmd.Flags().String("bundle-tag", "", "image tag for publishing the artifact bundle")
	_ = viper.BindPFlag("imagepublish.bundletag", imagePublishCmd.Flags().Lookup("bundle-tag"))

	// run code-scan
	codeScanCmd := newBasicCommand("code-scan", "run Static Application Security Tests (SAST) scans", runCodeScan)

	codeScanCmd.Flags().String("gitleaks-filename", "", "the output filename for the gitleaks vulnerability report")
	_ = viper.BindPFlag("codescan.gitleaksfilename", codeScanCmd.Flags().Lookup("gitleaks-filename"))

	codeScanCmd.Flags().String("semgrep-filename", "", "the output filename for the semgrep vulnerability report")
	_ = viper.BindPFlag("codescan.semgrepfilename", codeScanCmd.Flags().Lookup("semgrep-filename"))

	codeScanCmd.Flags().String("semgrep-rules", "", "the rules semgrep will use for the scan")
	_ = viper.BindPFlag("codescan.semgreprules", codeScanCmd.Flags().Lookup("semgrep-rules"))

	codeScanCmd.Flags().Bool("semgrep-experimental", false, "Enable the use of the semgrep experimental CLI")
	_ = viper.BindPFlag("codescan.semgrepexperimental", codeScanCmd.Flags().Lookup("semgrep-experimental"))

	codeScanCmd.Flags().String("coverage-file", "", "An externally generated code coverage file to validate")
	_ = viper.BindPFlag("codescan.coveragefile", codeScanCmd.Flags().Lookup("coverage-file"))

	// run deploy
	deployCmd := newBasicCommand("deploy", "BETA FEATURE: The deploy command performs bundle validation and invokes webhooks. Actual deployment is performed via webhooks.", runDeployExplicit)
	deployCmd.Flags().String("gatecheck-config", "", "gatecheck configuration file")
	_ = viper.BindPFlag("deploy.gatecheckconfigfilename", deployCmd.Flags().Lookup("gatecheck-config"))

	// run image-delivery

	imageDeliveryCmd := newBasicCommand("image-delivery", "run image build + image scan + image publish", runImageDelivery)

	imageDeliveryCmd.Flags().AddFlagSet(imageBuildCmd.Flags())
	imageDeliveryCmd.Flags().AddFlagSet(imageScanCmd.Flags())
	imageDeliveryCmd.Flags().AddFlagSet(imagePublishCmd.Flags())

	// run all

	allCmd := newBasicCommand("all", "run code scan + image delivery + deployment validation", runAll)

	allCmd.Flags().AddFlagSet(codeScanCmd.Flags())
	allCmd.Flags().AddFlagSet(imageBuildCmd.Flags())
	allCmd.Flags().AddFlagSet(imagePublishCmd.Flags())
	allCmd.Flags().AddFlagSet(deployCmd.Flags())

	// run
	cmd := &cobra.Command{Use: "run", Short: "run a pipeline"}

	// Persistent Flags, available on all sub commands
	cmd.PersistentFlags().BoolP("dry-run", "n", false, "log commands to debug but don't execute")
	cmd.PersistentFlags().StringP("config", "f", "", "portage config file in json, yaml, or toml")
	cmd.PersistentFlags().StringP("cli-interface", "i", "docker", "[docker|podman] CLI interface to use for image building")
	cmd.PersistentFlags().String("artifact-dir", "", "the target output directory for security report artifacts")
	cmd.PersistentFlags().String("tag", "", "the target image tag (ex. alpine:latest)")
	// cmd.PersistentFlags().String("template", "t", "", "portage config template that will be auto rendered")

	// necessary for the persistent flags
	_ = viper.BindPFlag("artifactdir", cmd.PersistentFlags().Lookup("artifact-dir"))
	_ = viper.BindPFlag("imagetag", cmd.PersistentFlags().Lookup("tag"))
	_ = viper.BindPFlag("config", cmd.PersistentFlags().Lookup("config"))

	// Flag marks
	_ = cmd.MarkFlagFilename("config", "json", "yaml", "yml", "toml")
	_ = cmd.MarkFlagDirname("build-dir")

	// Other settings
	cmd.SilenceUsage = true

	// Add sub commands
	cmd.AddCommand(debugCmd, imageBuildCmd, imageScanCmd, imagePublishCmd, codeScanCmd, deployCmd, imageDeliveryCmd, allCmd)

	return cmd
}

// Run Functions - Parsing flags and arguments at command runtime

func runDebug(cmd *cobra.Command, _ []string) error {
	dryRunEnabled, _ := cmd.Flags().GetBool("dry-run")
	return debugPipeline(cmd.OutOrStdout(), cmd.ErrOrStderr(), dryRunEnabled)
}

func runDeployExplicit(cmd *cobra.Command, args []string) error {
	return runDeploy(cmd, args, true)
}

func runDeploy(cmd *cobra.Command, _ []string, force bool) error {
	dryRunEnabled, _ := cmd.Flags().GetBool("dry-run")
	config := new(pipelines.Config)
	if err := viper.Unmarshal(config); err != nil {
		return err
	}
	if force && !config.Deploy.Enabled {
		slog.Debug("explicitly enabling deploy task")
		config.Deploy.Enabled = true
	} else if config.Deploy.Enabled {
		deployEnabledVar := os.Getenv("PORTAGE_DEPLOY_ENABLED")
		slog.Debug("deploy task is enabled", "env", deployEnabledVar)
	}
	return deployPipeline(cmd.OutOrStdout(), cmd.ErrOrStderr(), config, dryRunEnabled)
}

func runImageBuild(cmd *cobra.Command, _ []string) error {
	dryRunEnabled, _ := cmd.Flags().GetBool("dry-run")
	cliInterface, _ := cmd.Flags().GetString("cli-interface")

	config := new(pipelines.Config)
	if err := viper.Unmarshal(config); err != nil {
		return err
	}
	return imageBuildPipeline(cmd.OutOrStdout(), cmd.ErrOrStderr(), config, dryRunEnabled, cliInterface)
}

func runImageScan(cmd *cobra.Command, _ []string) error {
	dryRunEnabled, _ := cmd.Flags().GetBool("dry-run")
	cliInterface, _ := cmd.Flags().GetString("cli-interface")

	config := new(pipelines.Config)
	if err := viper.Unmarshal(config); err != nil {
		return err
	}

	return imageScanPipeline(cmd.OutOrStdout(), cmd.ErrOrStderr(), config, dryRunEnabled, cliInterface)
}

func runimagePublish(cmd *cobra.Command, _ []string) error {
	dryRunEnabled, _ := cmd.Flags().GetBool("dry-run")
	cliInterface, _ := cmd.Flags().GetString("cli-interface")

	config := new(pipelines.Config)
	if err := viper.Unmarshal(config); err != nil {
		return err
	}
	return imagePublishPipeline(cmd.OutOrStdout(), cmd.ErrOrStderr(), config, dryRunEnabled, cliInterface)
}

func runCodeScan(cmd *cobra.Command, _ []string) error {
	dryRunEnabled, _ := cmd.Flags().GetBool("dry-run")

	config := new(pipelines.Config)
	if err := viper.Unmarshal(config); err != nil {
		return err
	}
	return codeScanPipeline(cmd.OutOrStdout(), cmd.ErrOrStderr(), config, dryRunEnabled)
}

func runImageDelivery(cmd *cobra.Command, args []string) error {
	err := runImageBuild(cmd, args)
	if err != nil {
		return err
	}
	err = runImageScan(cmd, args)
	if err != nil {
		return err
	}

	return runimagePublish(cmd, args)
}

func runAll(cmd *cobra.Command, args []string) error {
	err := runCodeScan(cmd, args)
	if err != nil {
		return err
	}
	err = runImageDelivery(cmd, args)
	if err != nil {
		return err
	}

	return runDeploy(cmd, args, false)
}

// Execution functions - Logic for command execution

func imageBuildPipeline(stdout io.Writer, stderr io.Writer, config *pipelines.Config, dryRunEnabled bool, cliInterface string) error {
	pipeline := pipelines.NewImageBuild(stdout, stderr)
	pipeline.DryRunEnabled = dryRunEnabled
	pipeline.DockerAlias = cliInterface
	return pipeline.WithBuildConfig(config).Run()
}

func imageScanPipeline(stdout io.Writer, stderr io.Writer, config *pipelines.Config, dryRunEnabled bool, cliInterface string) error {
	pipeline := pipelines.NewImageScan(stdout, stderr)
	pipeline.DryRunEnabled = dryRunEnabled
	pipeline.DockerAlias = cliInterface

	return pipeline.WithConfig(config).Run()
}

func imagePublishPipeline(stdout io.Writer, stderr io.Writer, config *pipelines.Config, dryRunEnabled bool, cliInterface string) error {
	pipeline := pipelines.NewimagePublish(stdout, stderr)
	pipeline.DryRunEnabled = dryRunEnabled
	pipeline.DockerAlias = cliInterface
	return pipeline.WithConfig(config).Run()
}

func codeScanPipeline(stdout io.Writer, stderr io.Writer, config *pipelines.Config, dryRunEnabled bool) error {
	pipeline := pipelines.NewCodeScan(stdout, stderr)
	pipeline.DryRunEnabled = dryRunEnabled

	return pipeline.WithConfig(config).Run()
}

func deployPipeline(stdout io.Writer, stderr io.Writer, config *pipelines.Config, dryRunEnabled bool) error {
	pipeline := pipelines.NewDeploy(stdout, stderr)
	pipeline.DryRunEnabled = dryRunEnabled

	return pipeline.WithConfig(config).Run()
}

func debugPipeline(stdout io.Writer, stderr io.Writer, dryRunEnabled bool) error {
	pipeline := pipelines.NewDebug(stdout, stderr)
	pipeline.DryRunEnabled = dryRunEnabled

	return pipeline.Run()
}
