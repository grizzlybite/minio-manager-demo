// Package cli wires the cobra command tree, viper flag/env binding and the
// orchestration of load -> validate -> plan -> apply.
package cli

import (
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// BuildInfo carries version metadata injected at build time (see Makefile
// ldflags) and surfaced by the version command.
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

// NewRootCommand builds the root command with all global flags, env bindings
// and subcommands.
func NewRootCommand(build BuildInfo) *cobra.Command {
	root := &cobra.Command{
		Use:          "minio-manager",
		Short:        "Declarative MinIO resource manager",
		SilenceUsage: true, // do not print usage on errors in CI
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			setupLogger()
			return nil
		},
	}

	pf := root.PersistentFlags()
	pf.StringSlice("config", nil, "Path to a YAML config file; repeatable, each file is a separate MinIO cluster")
	pf.String("config-dir", "", "Directory of *.yaml/*.yml config files; each file is a separate MinIO cluster")
	pf.String("log-level", "info", "Log level: debug|info|warn|error")
	pf.String("log-format", "text", "Log format: text|json")
	pf.String("minio-endpoint", "", "MinIO endpoint (overrides config)")
	pf.String("minio-access-key", "", "MinIO access key (overrides config)")
	pf.String("minio-secret-key", "", "MinIO secret key (overrides config)")
	pf.String("consul-addr", "", "Consul agent address")
	pf.String("consul-key", "", "Consul KV key with config YAML")
	pf.String("vault-addr", "", "Vault address")
	pf.String("vault-path", "", "Vault KV path with config YAML")
	pf.String("vault-field", "config", "Field in Vault secret containing YAML")

	// Bind to ENV via viper (priority: CLI flag > ENV > default).
	bindFlag(root, "config", "MINIO_MANAGER_CONFIG")
	bindFlag(root, "config-dir", "MINIO_MANAGER_CONFIG_DIR")
	bindFlag(root, "log-level", "LOG_LEVEL")
	bindFlag(root, "log-format", "")
	bindFlag(root, "minio-endpoint", "MINIO_ENDPOINT")
	bindFlag(root, "minio-access-key", "MINIO_ACCESS_KEY")
	bindFlag(root, "minio-secret-key", "MINIO_SECRET_KEY")
	bindFlag(root, "consul-addr", "CONSUL_HTTP_ADDR")
	bindFlag(root, "consul-key", "")
	bindFlag(root, "vault-addr", "VAULT_ADDR")
	bindFlag(root, "vault-path", "")
	bindFlag(root, "vault-field", "")

	root.AddCommand(
		newValidateCommand(),
		newPlanCommand(),
		newApplyCommand(),
		newImportCommand(),
		newVersionCommand(build),
	)
	return root
}

// bindFlag binds a persistent flag to viper and, when env is non-empty, to the
// given environment variable.
func bindFlag(cmd *cobra.Command, flag, env string) {
	_ = viper.BindPFlag(flag, cmd.PersistentFlags().Lookup(flag))
	if env != "" {
		_ = viper.BindEnv(flag, env)
	}
}

// setupLogger configures the default slog logger from the log-level/log-format
// flags. Logs go to stderr so that stdout carries only command output (plans).
func setupLogger() {
	level := slog.LevelInfo
	switch strings.ToLower(viper.GetString("log-level")) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if strings.ToLower(viper.GetString("log-format")) == "json" {
		h = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		h = slog.NewTextHandler(os.Stderr, opts)
	}
	slog.SetDefault(slog.New(h))
}
