package cmd

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/soulgarden/logfowd/conf"
	"github.com/soulgarden/logfowd/service"
	"github.com/spf13/cobra"
)

func newWorker() *cobra.Command {
	return &cobra.Command{
		Use:   "worker",
		Short: "Main process",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			logger := zerolog.New(os.Stdout).With().Caller().Logger()

			cfg, err := conf.New()
			if err != nil {
				logger.Err(err).Msg("load config")

				os.Exit(1)
			}

			if cfg.DebugMode {
				zerolog.SetGlobalLevel(zerolog.DebugLevel)
			}

			cmdManager := service.NewManager(&logger)

			ctx, _ := cmdManager.ListenSignal()

			go func() {
				<-ctx.Done()

				logger.Debug().Msg("start graceful shutting down")
			}()

			service.NewWatcher(
				cfg,
				service.NewState(cfg, &logger),
				service.NewESCli(cfg, &logger),
				&logger,
			).Start(ctx)
		},
	}
}
