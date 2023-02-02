package cmd

import (
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

// nolint: gochecknoglobals
var rootCmd = &cobra.Command{
	Use:   "logfwd",
	Short: "Forward docker logs to es",
	Args:  cobra.ExactArgs(1),
}

func Execute() {
	rootCmd.AddCommand(newWorker())

	if err := rootCmd.Execute(); err != nil {
		log.Err(err).Msg("Command execution failed")
		os.Exit(1)
	}
}
