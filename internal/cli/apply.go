package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/yourorg/minio-manager/internal/config"
	minioc "github.com/yourorg/minio-manager/internal/minio"
)

func newApplyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply the configuration to MinIO",
		RunE: func(cmd *cobra.Command, args []string) error {
			prune, _ := cmd.Flags().GetBool("prune")
			return runApply(cmd.Context(), prune)
		},
	}
	cmd.Flags().Bool("prune", false, "Delete resources absent from config")
	return cmd
}

// runApply loads, validates, diffs and applies every configured target to its
// MinIO cluster. Each target (config file) is processed independently; a failure
// on one cluster is recorded but does not stop the others, and the aggregated
// error is returned at the end so exit status reflects any failure.
func runApply(ctx context.Context, prune bool) error {
	targets, err := loadConfigs(ctx)
	if err != nil {
		return err
	}
	single := len(targets) == 1

	var errs []error
	for _, t := range targets {
		if !single {
			fmt.Fprintf(os.Stdout, "\n=== %s ===\n", t.name)
		}
		if err := applyTarget(ctx, t, prune, single); err != nil {
			slog.Error("apply failed", "target", t.name, "err", err)
			errs = append(errs, fmt.Errorf("%s: %w", t.name, err))
		}
	}
	return errors.Join(errs...)
}

// applyTarget validates, plans and applies a single target against its cluster.
func applyTarget(ctx context.Context, t target, prune, allowFlagOverride bool) error {
	if err := config.Validate(t.cfg, t.name); err != nil {
		return err
	}

	client, err := newMinioClient(t.cfg, allowFlagOverride)
	if err != nil {
		return err
	}

	rec := minioc.NewReconciler(client)
	diff, err := rec.Plan(ctx, t.cfg.ToDesired(), prune)
	if err != nil {
		return err
	}

	printPlan(os.Stdout, diff)
	if diff.Empty() {
		slog.Info("no changes to apply", "target", t.name)
		return nil
	}

	if err := rec.Apply(ctx, diff); err != nil {
		return err
	}
	slog.Info("apply complete", "target", t.name)
	return nil
}
