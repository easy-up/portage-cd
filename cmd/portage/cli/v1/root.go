package cli

import (
	"log/slog"
	"portage/pkg/settings"

	"github.com/spf13/cobra"
)

var (
	AppLogLever *slog.LevelVar
	metaConfig  *settings.MetaConfig = settings.NewMetaConfig()
	config      *settings.Config     = settings.NewConfig()
)

func NewPortageCommand() *cobra.Command {
	portageCmd.PersistentFlags().BoolP("verbose", "v", false, "set logging level to debug")
	portageCmd.PersistentFlags().BoolP("silent", "s", false, "set logging level to error")

	portageCmd.SilenceUsage = true

	portageCmd.AddCommand(newRunTaskCommand())
	return portageCmd
}

var portageCmd = &cobra.Command{
	Use:   "portage",
	Short: "A portable, opinionated security pipeline",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		verboseFlag, _ := cmd.Flags().GetBool("verbose")
		silentFlag, _ := cmd.Flags().GetBool("silent")

		switch {
		case verboseFlag:
			AppLogLever.Set(slog.LevelDebug)
		case silentFlag:
			AppLogLever.Set(slog.LevelError)
		}
	},
}
