package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yourorg/minio-manager/internal/config"
	minioc "github.com/yourorg/minio-manager/internal/minio"
)

func newPlanCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Show the changes apply would make (dry-run)",
		RunE: func(cmd *cobra.Command, args []string) error {
			prune, _ := cmd.Flags().GetBool("prune")
			return runPlan(cmd.Context(), prune)
		},
	}
	cmd.Flags().Bool("prune", false, "Include resources absent from config (deletions)")
	return cmd
}

// runPlan loads, validates and diffs every configured target against its MinIO
// cluster, printing each plan to stdout without applying any changes. Targets
// are independent; a failure on one is recorded and the rest still run.
func runPlan(ctx context.Context, prune bool) error {
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
		if err := planTarget(ctx, t, prune, single); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", t.name, err))
		}
	}
	return errors.Join(errs...)
}

// planTarget validates and diffs a single target, printing its plan.
func planTarget(ctx context.Context, t target, prune, allowFlagOverride bool) error {
	if err := config.Validate(t.cfg, t.name); err != nil {
		return err
	}

	client, err := newMinioClient(t.cfg, allowFlagOverride)
	if err != nil {
		return err
	}

	diff, err := minioc.NewReconciler(client).Plan(ctx, t.cfg.ToDesired(), prune)
	if err != nil {
		return err
	}
	printPlan(os.Stdout, diff)
	return nil
}
