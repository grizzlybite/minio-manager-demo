package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/yourorg/minio-manager/internal/config"
)

func newValidateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate config syntax and semantics without contacting MinIO",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runValidate(cmd.Context())
		},
	}
}

// runValidate loads and semantically validates every configured target. Each is
// checked independently; failures are aggregated so all problems are reported in
// one run instead of stopping at the first bad file.
func runValidate(ctx context.Context) error {
	targets, err := loadConfigs(ctx)
	if err != nil {
		return err
	}

	var errs []error
	for _, t := range targets {
		if err := config.Validate(t.cfg, t.name); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", t.name, err))
			continue
		}
		slog.Info("configuration is valid",
			"source", t.name,
			"buckets", len(t.cfg.Buckets),
			"users", len(t.cfg.Users),
			"policies", len(t.cfg.Policies),
		)
	}
	return errors.Join(errs...)
}
