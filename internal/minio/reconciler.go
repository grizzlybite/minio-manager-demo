package minio

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/yourorg/minio-manager/internal/state"
	"golang.org/x/sync/errgroup"
)

// Reconciler computes and applies the difference between the desired
// configuration and the current MinIO state.
type Reconciler struct {
	client *Client
}

// NewReconciler returns a Reconciler bound to the given client.
func NewReconciler(client *Client) *Reconciler {
	return &Reconciler{client: client}
}

// Plan fetches the current MinIO state and computes the diff against desired.
// The prune flag is recorded on the result; it does not by itself execute any
// deletions (that happens in Apply).
func (r *Reconciler) Plan(ctx context.Context, desired state.DesiredState, prune bool) (state.DiffResult, error) {
	current, err := r.client.FetchCurrentState(ctx)
	if err != nil {
		return state.DiffResult{}, fmt.Errorf("fetch current state: %w", err)
	}
	diff := state.Diff(desired, *current)
	diff.Prune = prune
	return diff, nil
}

// Apply converges MinIO to the desired state in ordered phases:
//
//	Phase 1: create/update policies (users depend on them);
//	Phase 2: create/update buckets and users in parallel;
//	Phase 3: when Prune is set, delete users -> policies -> buckets.
//
// Within each phase independent operations run concurrently under a shared
// errgroup context, so the first error cancels the rest.
func (r *Reconciler) Apply(ctx context.Context, diff state.DiffResult) error {
	if err := r.applyPolicies(ctx, diff); err != nil {
		return fmt.Errorf("policies apply: %w", err)
	}
	if err := r.applyBucketsAndUsers(ctx, diff); err != nil {
		return fmt.Errorf("resources apply: %w", err)
	}
	if !diff.Prune {
		return nil
	}
	if err := r.prune(ctx, diff); err != nil {
		return fmt.Errorf("prune: %w", err)
	}
	return nil
}

// applyPolicies runs phase 1.
func (r *Reconciler) applyPolicies(ctx context.Context, diff state.DiffResult) error {
	g, gctx := errgroup.WithContext(ctx)
	for _, p := range diff.PoliciesToCreate {
		g.Go(func() error { return r.client.CreatePolicy(gctx, p) })
	}
	for _, p := range diff.PoliciesToUpdate {
		g.Go(func() error { return r.client.UpdatePolicy(gctx, p) })
	}
	return g.Wait()
}

// applyBucketsAndUsers runs phase 2.
func (r *Reconciler) applyBucketsAndUsers(ctx context.Context, diff state.DiffResult) error {
	g, gctx := errgroup.WithContext(ctx)
	for _, b := range diff.BucketsToCreate {
		g.Go(func() error { return r.client.CreateBucket(gctx, b) })
	}
	for _, b := range diff.BucketsToUpdate {
		g.Go(func() error { return r.client.UpdateBucket(gctx, b) })
	}
	for _, u := range diff.UsersToCreate {
		g.Go(func() error { return r.client.CreateUser(gctx, u) })
	}
	for _, u := range diff.UsersToUpdate {
		g.Go(func() error { return r.client.UpdateUser(gctx, u) })
	}
	return g.Wait()
}

// prune runs phase 3: deletions in dependency-safe order (users first, since
// they reference policies; then policies; then buckets).
func (r *Reconciler) prune(ctx context.Context, diff state.DiffResult) error {
	if n := len(diff.UsersToDelete) + len(diff.PoliciesToDelete) + len(diff.BucketsToDelete); n > 0 {
		slog.Warn("pruning resources absent from config",
			"users", len(diff.UsersToDelete),
			"policies", len(diff.PoliciesToDelete),
			"buckets", len(diff.BucketsToDelete),
		)
	}

	if err := r.deleteEach(ctx, diff.UsersToDelete, r.client.DeleteUser); err != nil {
		return err
	}
	if err := r.deleteEach(ctx, diff.PoliciesToDelete, r.client.DeletePolicy); err != nil {
		return err
	}
	return r.deleteEach(ctx, diff.BucketsToDelete, r.client.DeleteBucket)
}

// deleteEach removes each named resource concurrently using del.
func (r *Reconciler) deleteEach(ctx context.Context, names []string, del func(context.Context, string) error) error {
	if len(names) == 0 {
		return nil
	}
	g, gctx := errgroup.WithContext(ctx)
	for _, name := range names {
		g.Go(func() error { return del(gctx, name) })
	}
	return g.Wait()
}
