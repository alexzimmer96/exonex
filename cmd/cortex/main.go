package main

import (
	"log/slog"
	"os"
	"strings"

	"github.com/alexzimmer96/exonex/internal/cortex"
	"github.com/alexzimmer96/exonex/pkg"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	configFile   string
	debugLogging bool
)

func init() {
	handler := slog.New(
		slog.NewTextHandler(os.Stdout, nil),
	)
	slog.SetDefault(handler)

	cobra.OnInitialize(initConfig)

	apiServerCommand.PersistentFlags().String("server-addr", ":8080", "Address for serving the api server")
	apiServerCommand.PersistentFlags().String("metrics-addr", ":8090", "Address for for serving the metrics server")
	apiServerCommand.PersistentFlags().String("database-url", "", "URL for the Postgres Database to connect")
	pkg.Must(viper.BindPFlags(apiServerCommand.PersistentFlags()))

	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "config.yaml", "Path to the configuration file")
	rootCmd.PersistentFlags().BoolVarP(&debugLogging, "debug", "d", false, "Enable Debug logging")
	pkg.Must(viper.BindPFlags(rootCmd.PersistentFlags()))

	rootCmd.AddCommand(
		apiServerCommand,
	)
}

// =====================================================================================================================

func initConfig() {
	viper.SetConfigFile(configFile)
	viper.SetEnvPrefix("CORTEX")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err == nil {
		slog.Info("loaded configuration file", slog.String("file", viper.ConfigFileUsed()))
	}
}

// =====================================================================================================================

func defaultPrerun(cmd *cobra.Command, args []string) {
	slog.SetLogLoggerLevel(slog.LevelInfo)
	if debugLogging {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}
}

// =====================================================================================================================

var rootCmd = &cobra.Command{
	Use:    "cortex",
	Short:  "All components that built the Cortex Platform bundled in one binary",
	PreRun: defaultPrerun,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

// =====================================================================================================================

var apiServerCommand = &cobra.Command{
	Use:    "api-server",
	Short:  "Starts the Cortex API-Server",
	PreRun: defaultPrerun,
	Run: func(cmd *cobra.Command, args []string) {
		var config cortex.Config
		if err := viper.Unmarshal(&config); err != nil {
			slog.Error("failed to read config", slog.String("error", err.Error()))
			os.Exit(1)
		}
		if err := config.Validate(); err != nil {
			slog.Error("failed to validate loaded config", slog.String("error", err.Error()))
			os.Exit(1)
		}

		srv := cortex.NewServer(config)
		srv.ListenAndServe()
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
