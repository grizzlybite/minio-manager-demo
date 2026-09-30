package minio

import (
	"context"
	"log/slog"

	apperrors "github.com/yourorg/minio-manager/internal/errors"
	"github.com/yourorg/minio-manager/internal/state"
)

// CreatePolicy adds a new canned IAM policy from its document.
func (c *Client) CreatePolicy(ctx context.Context, spec state.PolicySpec) error {
	slog.Info("creating policy", "policy", spec.Name)
	if err := c.Admin.AddCannedPolicy(ctx, spec.Name, spec.Document); err != nil {
		return &apperrors.MinioError{Op: "create", Kind: apperrors.ResourcePolicy, Resource: spec.Name, Cause: err}
	}
	return nil
}

// UpdatePolicy overwrites an existing canned policy with the desired document.
// AddCannedPolicy upserts, so it serves for both create and update.
func (c *Client) UpdatePolicy(ctx context.Context, upd state.PolicyUpdate) error {
	slog.Info("updating policy", "policy", upd.Name)
	if err := c.Admin.AddCannedPolicy(ctx, upd.Name, upd.Desired.Document); err != nil {
		return &apperrors.MinioError{Op: "update", Kind: apperrors.ResourcePolicy, Resource: upd.Name, Cause: err}
	}
	return nil
}

// DeletePolicy removes a canned policy.
func (c *Client) DeletePolicy(ctx context.Context, name string) error {
	slog.Warn("deleting policy", "policy", name)
	if err := c.Admin.RemoveCannedPolicy(ctx, name); err != nil {
		return &apperrors.MinioError{Op: "delete", Kind: apperrors.ResourcePolicy, Resource: name, Cause: err}
	}
	return nil
}
